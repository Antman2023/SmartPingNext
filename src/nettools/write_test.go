package nettools

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

type pipePacketWriter struct {
	net.Conn
	started chan struct{}
}

func (writer pipePacketWriter) WriteTo(message []byte, _ net.Addr) (int, error) {
	select {
	case writer.started <- struct{}{}:
	default:
	}
	return writer.Write(message)
}

func TestICMPWriteCancellationReleasesSocketForNextRequest(t *testing.T) {
	writer, reader := net.Pipe()
	defer writer.Close()
	defer reader.Close()
	packet := pipePacketWriter{Conn: writer, started: make(chan struct{}, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		result <- writeICMPPacketContext(ctx, packet, []byte("first"), nil, time.Now().Add(time.Minute))
	}()
	select {
	case <-packet.started:
	case <-time.After(time.Second):
		t.Fatal("write did not start")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("write returned %v, want cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation did not interrupt blocked write")
	}
	readResult := make(chan error, 1)
	go func() {
		message := make([]byte, len("second"))
		_, err := io.ReadFull(reader, message)
		if err == nil && string(message) != "second" {
			err = errors.New("unexpected packet content")
		}
		readResult <- err
	}()
	if err := writeICMPPacketContext(context.Background(), packet, []byte("second"), nil, time.Now().Add(time.Second)); err != nil {
		t.Fatalf("previous cancellation affected next write: %v", err)
	}
	if err := <-readResult; err != nil {
		t.Fatal(err)
	}
}

func TestICMPWriteHonorsDeadlineWhenSocketIsBlocked(t *testing.T) {
	for _, useContextDeadline := range []bool{false, true} {
		name := "packet deadline"
		if useContextDeadline {
			name = "earlier context deadline"
		}
		t.Run(name, func(t *testing.T) {
			writer, reader := net.Pipe()
			defer writer.Close()
			defer reader.Close()
			packet := pipePacketWriter{Conn: writer}
			ctx := context.Background()
			deadline := time.Now().Add(20 * time.Millisecond)
			if useContextDeadline {
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(ctx, deadline)
				defer cancel()
				deadline = time.Now().Add(time.Minute)
			}
			result := make(chan error, 1)
			go func() {
				result <- writeICMPPacketContext(ctx, packet, []byte("probe"), nil, deadline)
			}()
			select {
			case err := <-result:
				var networkError net.Error
				if !errors.As(err, &networkError) || !networkError.Timeout() {
					t.Fatalf("blocked write returned %v, want timeout", err)
				}
			case <-time.After(time.Second):
				t.Fatal("blocked write did not respect its deadline")
			}
		})
	}
}
