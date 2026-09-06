package nettools

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/ipv4"
)

func TestWaiterKeyUsesIdentifierAndSequence(t *testing.T) {
	base := waiterKey(100, 200)
	if base == waiterKey(100, 201) {
		t.Fatalf("waiterKey should distinguish different sequence values")
	}
	if base == waiterKey(101, 200) {
		t.Fatalf("waiterKey should distinguish different identifier values")
	}
}

func TestEmbeddedEchoWaiterKeyUsesIPv4HeaderLength(t *testing.T) {
	for _, headerLength := range []int{ipv4.HeaderLen, ipv4.HeaderLen + 4, 60} {
		t.Run(fmt.Sprintf("header-%d", headerLength), func(t *testing.T) {
			data := make([]byte, headerLength+8)
			data[0] = 0x40 | byte(headerLength/4)
			data[9] = 1
			data[headerLength] = byte(ipv4.ICMPTypeEcho)
			binary.BigEndian.PutUint16(data[headerLength+4:headerLength+6], 0x1234)
			binary.BigEndian.PutUint16(data[headerLength+6:headerLength+8], 0x5678)

			key, ok := embeddedEchoWaiterKey(data)
			if !ok {
				t.Fatal("embeddedEchoWaiterKey rejected a valid embedded echo request")
			}
			if want := waiterKey(0x1234, 0x5678); key != want {
				t.Fatalf("waiter key = %#x, want %#x", key, want)
			}
		})
	}
}

func TestEmbeddedEchoWaiterKeyRejectsMalformedPackets(t *testing.T) {
	valid := make([]byte, ipv4.HeaderLen+8)
	valid[0] = 0x45
	valid[9] = 1
	valid[ipv4.HeaderLen] = byte(ipv4.ICMPTypeEcho)

	tests := map[string][]byte{
		"short header":          valid[:ipv4.HeaderLen-1],
		"wrong IP version":      append([]byte{0x65}, valid[1:]...),
		"short IHL":             append([]byte{0x44}, valid[1:]...),
		"truncated options":     append([]byte{0x46}, valid[1:]...),
		"non ICMP protocol":     append(append([]byte(nil), valid[:9]...), append([]byte{17}, valid[10:]...)...),
		"non echo ICMP message": append(append([]byte(nil), valid[:ipv4.HeaderLen]...), append([]byte{byte(ipv4.ICMPTypeEchoReply)}, valid[ipv4.HeaderLen+1:]...)...),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if _, ok := embeddedEchoWaiterKey(data); ok {
				t.Fatal("embeddedEchoWaiterKey accepted a malformed packet")
			}
		})
	}
}

func TestNextICMPSequenceCoversFullCycleWithoutDuplicates(t *testing.T) {
	seen := make([]bool, 1<<16)
	for i := 0; i < len(seen); i++ {
		sequence := nextICMPSequence()
		if sequence < 0 || sequence >= len(seen) {
			t.Fatalf("ICMP sequence is outside 16-bit range: %d", sequence)
		}
		if seen[sequence] {
			t.Fatalf("ICMP sequence %d repeated before the full cycle completed", sequence)
		}
		seen[sequence] = true
	}
}

func TestEvaluatePingResultRequiresFinalReply(t *testing.T) {
	if _, err := evaluatePingResult(ICMP{RTT: time.Millisecond}); err == nil {
		t.Fatalf("non-final ICMP response should not count as successful ping")
	}
	delay, err := evaluatePingResult(ICMP{RTT: 1500 * time.Microsecond, Final: true})
	if err != nil {
		t.Fatalf("final reply returned error: %v", err)
	}
	if delay != 1.5 {
		t.Fatalf("delay = %v, want 1.5ms", delay)
	}
}

func TestEvaluatePingResultPreservesErrors(t *testing.T) {
	want := errors.New("send failed")
	if _, err := evaluatePingResult(ICMP{Error: want}); !errors.Is(err, want) {
		t.Fatalf("evaluatePingResult error = %v, want %v", err, want)
	}
}

func TestRunPingContextReturnsBeforeSocketInitializationWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := RunPingContext(ctx, &net.IPAddr{IP: net.ParseIP("127.0.0.1")}, time.Second, 64, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunPingContext error = %v, want context canceled", err)
	}
}

func TestRunPingContextRejectsInvalidArgumentsBeforeSocketInitialization(t *testing.T) {
	validAddress := &net.IPAddr{IP: net.ParseIP("127.0.0.1")}
	tests := []struct {
		name    string
		address *net.IPAddr
		timeout time.Duration
		ttl     int
	}{
		{name: "nil address", timeout: time.Second, ttl: 64},
		{name: "IPv6 address", address: &net.IPAddr{IP: net.ParseIP("2001:db8::1")}, timeout: time.Second, ttl: 64},
		{name: "zero timeout", address: validAddress, ttl: 64},
		{name: "zero TTL", address: validAddress, timeout: time.Second},
		{name: "oversized TTL", address: validAddress, timeout: time.Second, ttl: 256},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := RunPingContext(context.Background(), tt.address, tt.timeout, tt.ttl, 0); err == nil {
				t.Fatal("RunPingContext should reject invalid arguments")
			}
		})
	}
}

func TestWaitForICMPResponseHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := waitForICMPResponse(ctx, make(chan icmpResponse), time.Now(), time.Minute, &net.IPAddr{IP: net.ParseIP("192.0.2.1")})
	if !errors.Is(result.Error, context.Canceled) {
		t.Fatalf("waitForICMPResponse error = %v, want context canceled", result.Error)
	}
	if result.Timeout || result.Final || result.Down {
		t.Fatalf("canceled response contains an unexpected terminal state: %#v", result)
	}
}

func TestWaitForICMPResponseIgnoresFinalReplyFromUnexpectedSource(t *testing.T) {
	destination := &net.IPAddr{IP: net.ParseIP("192.0.2.1")}
	responses := make(chan icmpResponse, 2)
	responses <- icmpResponse{addr: &net.IPAddr{IP: net.ParseIP("198.51.100.1")}, final: true}
	responses <- icmpResponse{addr: destination, final: true}

	result := waitForICMPResponse(context.Background(), responses, time.Now(), time.Second, destination)
	if !result.Final || !sameIPAddress(result.Addr, destination) {
		t.Fatalf("waitForICMPResponse returned unexpected final response: %#v", result)
	}
}

func TestWaitForICMPResponseAcceptsIntermediateRouter(t *testing.T) {
	destination := &net.IPAddr{IP: net.ParseIP("192.0.2.1")}
	router := &net.IPAddr{IP: net.ParseIP("198.51.100.1")}
	responses := make(chan icmpResponse, 1)
	responses <- icmpResponse{addr: router}

	result := waitForICMPResponse(context.Background(), responses, time.Now(), time.Second, destination)
	if result.Final || !sameIPAddress(result.Addr, router) {
		t.Fatalf("waitForICMPResponse rejected intermediate router response: %#v", result)
	}
}

func TestSameIPAddress(t *testing.T) {
	ipv4 := &net.IPAddr{IP: net.IP{192, 0, 2, 1}}
	mapped := &net.IPAddr{IP: net.ParseIP("192.0.2.1")}
	if !sameIPAddress(ipv4, mapped) {
		t.Fatal("sameIPAddress should match equivalent 4-byte and 16-byte IPv4 addresses")
	}
	if sameIPAddress(ipv4, &net.IPAddr{IP: net.ParseIP("192.0.2.2")}) {
		t.Fatal("sameIPAddress should reject different addresses")
	}
	if sameIPAddress(nil, mapped) {
		t.Fatal("sameIPAddress should reject a missing address")
	}
}

func TestICMPPoolRegisterRejectsCollision(t *testing.T) {
	pool := &icmpPool{waiters: make(map[uint32]icmpWaiter)}
	if _, ok := pool.register(42, nil); !ok {
		t.Fatalf("first waiter registration should succeed")
	}
	if _, ok := pool.register(42, nil); ok {
		t.Fatalf("duplicate waiter registration should be rejected")
	}
}

func TestICMPPoolOldCleanupPreservesReplacementWaiter(t *testing.T) {
	localPool := &icmpPool{waiters: make(map[uint32]icmpWaiter)}
	old, _ := localPool.register(42, nil)
	if err := localPool.close(); err != nil {
		t.Fatal(err)
	}
	replacement, ok := localPool.register(42, nil)
	if !ok {
		t.Fatal("replacement registration failed")
	}
	localPool.unregister(42, old)
	localPool.dispatch(42, icmpResponse{down: true})
	select {
	case response := <-replacement:
		if !response.down {
			t.Fatal("unexpected response")
		}
	default:
		t.Fatal("old request cleanup removed the replacement waiter")
	}
	localPool.unregister(42, replacement)
	if len(localPool.waiters) != 0 {
		t.Fatal("replacement request cleanup retained its waiter")
	}
}

func TestICMPPoolIgnoresResponsesFromClosedConnection(t *testing.T) {
	openConnection := func() net.PacketConn {
		conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
		return conn
	}
	old := openConnection()
	current := openConnection()
	localPool := &icmpPool{conn: old, waiters: make(map[uint32]icmpWaiter)}
	if err := localPool.close(); err != nil {
		t.Fatal(err)
	}
	localPool.conn = current
	responses, _ := localPool.register(42, nil)
	localPool.dispatchFrom(old, 42, icmpResponse{down: true})
	if len(responses) != 0 {
		t.Fatal("old connection response reached the new waiter")
	}
	localPool.dispatchFrom(current, 42, icmpResponse{down: true})
	if len(responses) != 1 {
		t.Fatal("current connection response was dropped")
	}
}

func TestICMPPoolConcurrentSendAndCloseReleasesWaiters(t *testing.T) {
	// UDP exercises the lifecycle and IPv4 socket options without raw-socket privileges.
	receiver, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()
	localPool := &icmpPool{listenPacket: func(_, _ string) (net.PacketConn, error) {
		return net.ListenPacket("udp4", "127.0.0.1:0")
	}}
	defer localPool.close()
	var workers sync.WaitGroup
	for id := 0; id < 4; id++ {
		workers.Add(1)
		go func(id int) {
			defer workers.Done()
			for seq := 0; seq < 20; seq++ {
				result := localPool.sendICMP(id, seq, 64, []byte{0}, receiver.LocalAddr(), time.Millisecond)
				if result.Error != nil && !errors.Is(result.Error, net.ErrClosed) {
					t.Errorf("send during close: %v", result.Error)
				}
			}
		}(id)
	}
	workers.Add(1)
	go func() {
		defer workers.Done()
		for i := 0; i < 20; i++ {
			if err := localPool.close(); err != nil {
				t.Errorf("concurrent close: %v", err)
			}
		}
	}()
	workers.Wait()
	localPool.mu.RLock()
	defer localPool.mu.RUnlock()
	if len(localPool.waiters) != 0 {
		t.Fatalf("completed requests left %d waiters", len(localPool.waiters))
	}
}

func TestICMPPoolDispatchFiltersUnexpectedFinalSource(t *testing.T) {
	destination := &net.IPAddr{IP: net.ParseIP("192.0.2.1")}
	router := &net.IPAddr{IP: net.ParseIP("198.51.100.1")}
	pool := &icmpPool{waiters: make(map[uint32]icmpWaiter)}
	responses, ok := pool.register(42, destination)
	if !ok {
		t.Fatal("register rejected a new waiter")
	}

	pool.dispatch(42, icmpResponse{addr: router, final: true})
	if len(responses) != 0 {
		t.Fatal("unexpected final response occupied the waiter queue")
	}
	pool.dispatch(42, icmpResponse{addr: destination, final: true})
	if len(responses) != 1 {
		t.Fatal("expected final response was not dispatched")
	}
	<-responses
	pool.dispatch(42, icmpResponse{addr: router})
	if len(responses) != 1 {
		t.Fatal("intermediate router response was not dispatched")
	}
}

func TestICMPPoolRetriesInitializationAfterFailure(t *testing.T) {
	wantErr := errors.New("temporary listen failure")
	listenCalls := 0
	pool := &icmpPool{
		listenPacket: func(_, _ string) (net.PacketConn, error) {
			listenCalls++
			return nil, wantErr
		},
	}

	for attempt := 1; attempt <= 2; attempt++ {
		if err := pool.init(); !errors.Is(err, wantErr) {
			t.Fatalf("attempt %d error = %v, want %v", attempt, err, wantErr)
		}
	}
	if listenCalls != 2 {
		t.Fatalf("listen calls = %d, want retry on each initialization", listenCalls)
	}
	if pool.conn != nil || pool.ipconn != nil {
		t.Fatalf("failed initialization retained partial connection state")
	}
}

func TestICMPPoolCloseReleasesAndResetsConnection(t *testing.T) {
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("create packet connection: %v", err)
	}
	localPool := &icmpPool{
		conn:    conn,
		ipconn:  ipv4.NewPacketConn(conn),
		waiters: make(map[uint32]icmpWaiter),
	}
	responses, registered := localPool.register(42, &net.IPAddr{IP: net.ParseIP("192.0.2.1")})
	if !registered {
		t.Fatal("register rejected a new waiter")
	}
	waitResult := make(chan ICMP, 1)
	go func() {
		waitResult <- waitForICMPResponse(
			context.Background(),
			responses,
			time.Now(),
			time.Minute,
			&net.IPAddr{IP: net.ParseIP("192.0.2.1")},
		)
	}()

	if err := localPool.close(); err != nil {
		t.Fatalf("close returned error: %v", err)
	}
	if localPool.conn != nil || localPool.ipconn != nil {
		t.Fatalf("close retained connection state: %#v", localPool)
	}
	if len(localPool.waiters) != 0 {
		t.Fatalf("close retained %d ICMP waiters", len(localPool.waiters))
	}
	select {
	case result := <-waitResult:
		if !errors.Is(result.Error, net.ErrClosed) {
			t.Fatalf("waiter error after close = %v, want %v", result.Error, net.ErrClosed)
		}
	case <-time.After(time.Second):
		t.Fatal("close did not wake the ICMP waiter")
	}
	if err := localPool.close(); err != nil {
		t.Fatalf("second close should be idempotent, got: %v", err)
	}
}
