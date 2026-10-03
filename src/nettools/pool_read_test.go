package nettools

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

type icmpReadStep struct {
	packet []byte
	addr   net.Addr
	err    error
}

// Script errors and packets without raw sockets or external network traffic.
type scriptedICMPReader struct {
	steps     chan icmpReadStep
	closed    chan struct{}
	closeOnce sync.Once
	repeatErr error
	reads     atomic.Int32
}

func (reader *scriptedICMPReader) ReadFrom(packet []byte) (int, net.Addr, error) {
	reader.reads.Add(1)
	select {
	case <-reader.closed:
		return 0, nil, net.ErrClosed
	default:
	}
	var step icmpReadStep
	if reader.repeatErr != nil {
		select {
		case step = <-reader.steps:
		default:
			return 0, nil, reader.repeatErr
		}
	} else {
		select {
		case step = <-reader.steps:
		case <-reader.closed:
			return 0, nil, net.ErrClosed
		}
	}
	return copy(packet, step.packet), step.addr, step.err
}

func (reader *scriptedICMPReader) Close() error {
	reader.closeOnce.Do(func() { close(reader.closed) })
	return nil
}

func (*scriptedICMPReader) WriteTo(packet []byte, _ net.Addr) (int, error) {
	return len(packet), nil
}
func (*scriptedICMPReader) LocalAddr() net.Addr              { return &net.IPAddr{IP: net.IPv4zero} }
func (*scriptedICMPReader) SetDeadline(time.Time) error      { return nil }
func (*scriptedICMPReader) SetReadDeadline(time.Time) error  { return nil }
func (*scriptedICMPReader) SetWriteDeadline(time.Time) error { return nil }

func startScriptedICMPReader(t *testing.T, localPool *icmpPool, reader *scriptedICMPReader) <-chan struct{} {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		localPool.readLoop(reader)
	}()
	t.Cleanup(func() {
		localPool.close()
		// A stale reader is no longer owned by the pool, so close it separately.
		reader.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("ICMP reader did not exit after cleanup")
		}
	})
	return done
}

func icmpReadTestReply(t *testing.T, id, seq int) []byte {
	t.Helper()
	packet, err := (&icmp.Message{
		Type: ipv4.ICMPTypeEchoReply,
		Body: &icmp.Echo{ID: id, Seq: seq},
	}).Marshal(nil)
	if err != nil {
		t.Fatal(err)
	}
	return packet
}

func TestICMPReaderRepeatedErrorsReleaseWaitersAndAllowReinitialization(t *testing.T) {
	reader := &scriptedICMPReader{
		steps: make(chan icmpReadStep), closed: make(chan struct{}),
		repeatErr: errors.New("persistent read failure"),
	}
	localPool := &icmpPool{
		conn: reader, waiters: make(map[uint32]icmpWaiter),
		listenPacket: func(_, _ string) (net.PacketConn, error) {
			return net.ListenPacket("udp4", "127.0.0.1:0")
		},
	}
	first, _ := localPool.register(42, nil)
	second, _ := localPool.register(43, nil)
	started := time.Now()
	done := startScriptedICMPReader(t, localPool, reader)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("persistent read failures left the reader retrying indefinitely")
	}
	if reader.reads.Load() != 5 {
		t.Fatalf("read calls = %d, want 5 bounded attempts", reader.reads.Load())
	}
	if time.Since(started) < 120*time.Millisecond {
		t.Fatal("consecutive read errors were retried without progressive backoff")
	}
	for _, responses := range []chan icmpResponse{first, second} {
		result := waitForICMPResponse(context.Background(), responses, started, time.Minute, nil)
		if !errors.Is(result.Error, net.ErrClosed) || result.Timeout {
			t.Fatalf("failed connection waiter = %+v, want closed error rather than probe timeout", result)
		}
	}
	if localPool.conn != nil || localPool.ipconn != nil || len(localPool.waiters) != 0 {
		t.Fatal("retired reader retained connection state or waiters")
	}
	if err := localPool.init(); err != nil {
		t.Fatal(err)
	}
	localPool.initMu.Lock()
	replacement := localPool.conn
	destination := &net.IPAddr{IP: net.ParseIP("192.0.2.1")}
	responses, registered := localPool.register(42, destination)
	localPool.initMu.Unlock()
	if replacement == nil || replacement == reader || !registered {
		t.Fatal("next initialization did not provide a replacement with reusable identifiers")
	}
	// Exercise the replacement reader with a real UDP-carried ICMP error,
	// rather than invoking dispatch directly.
	quoted := make([]byte, ipv4.HeaderLen+8)
	quoted[0], quoted[9], quoted[ipv4.HeaderLen] = 0x45, 1, byte(ipv4.ICMPTypeEcho)
	copy(quoted[16:20], destination.IP.To4())
	binary.BigEndian.PutUint16(quoted[ipv4.HeaderLen+6:], 42)
	packet, err := (&icmp.Message{
		Type: ipv4.ICMPTypeTimeExceeded, Body: &icmp.TimeExceeded{Data: quoted},
	}).Marshal(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := replacement.WriteTo(packet, replacement.LocalAddr()); err != nil {
		t.Fatal(err)
	}
	select {
	case response, open := <-responses:
		if !open || response.final || response.down || !sameIPAddress(response.quotedDestination, destination) {
			t.Fatal("replacement did not deliver response")
		}
	case <-time.After(time.Second):
		t.Fatal("replacement waiter did not receive response")
	}
}

func TestICMPReaderSuccessfulReadResetsConsecutiveErrors(t *testing.T) {
	for _, malformed := range []bool{false, true} {
		name := "echo reply"
		if malformed {
			name = "malformed packet"
		}
		t.Run(name, func(t *testing.T) {
			reader := &scriptedICMPReader{steps: make(chan icmpReadStep, 10), closed: make(chan struct{})}
			destination := &net.IPAddr{IP: net.ParseIP("192.0.2.1")}
			for burst := 0; burst < 2; burst++ {
				for i := 0; i < 4; i++ {
					reader.steps <- icmpReadStep{err: errors.New("transient read failure")}
				}
				packet := icmpReadTestReply(t, 12, 34)
				if malformed && burst == 0 {
					packet = []byte{0}
				}
				reader.steps <- icmpReadStep{packet: packet, addr: destination}
			}
			localPool := &icmpPool{conn: reader, waiters: make(map[uint32]icmpWaiter)}
			responses, _ := localPool.register(waiterKey(12, 34), destination)
			startScriptedICMPReader(t, localPool, reader)
			for i := 0; i < 2; i++ {
				if malformed && i == 0 {
					continue
				}
				select {
				case response, open := <-responses:
					if !open || !response.final || !sameIPAddress(response.addr, destination) {
						t.Fatal("recoverable read errors retired the working connection")
					}
				case <-time.After(time.Second):
					t.Fatal("reader did not resume packet dispatch after transient errors")
				}
			}
			localPool.initMu.Lock()
			current := localPool.conn
			localPool.initMu.Unlock()
			if current != reader {
				t.Fatal("successful reads did not reset the error streak")
			}
		})
	}
}

func TestICMPReaderRepeatedOldErrorsCannotRetireReplacement(t *testing.T) {
	reader := &scriptedICMPReader{
		steps: make(chan icmpReadStep), closed: make(chan struct{}),
		repeatErr: errors.New("stale reader failure"),
	}
	replacement, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	localPool := &icmpPool{conn: replacement, waiters: make(map[uint32]icmpWaiter)}
	responses, _ := localPool.register(42, nil)
	done := startScriptedICMPReader(t, localPool, reader)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stale failing reader did not stop")
	}
	localPool.dispatchFrom(replacement, 42, icmpResponse{down: true})
	select {
	case response, open := <-responses:
		if !open || !response.down {
			t.Fatal("stale reader closed the replacement waiter")
		}
	default:
		t.Fatal("stale reader disrupted replacement response dispatch")
	}
}
