package nettools

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestResolveIPv4BoundsDNSWait(t *testing.T) {
	started := time.Now()
	var lookupContext context.Context
	_, err := resolveIPv4Context(context.Background(), "slow.example", func(ctx context.Context, network, host string) ([]net.IP, error) {
		lookupContext = ctx
		deadline, ok := ctx.Deadline()
		if !ok || deadline.After(started.Add(6*time.Second)) {
			t.Fatal("DNS lookup has no bounded deadline")
		}
		if network != "ip4" || host != "slow.example" {
			t.Fatalf("unexpected lookup: %q %q", network, host)
		}
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("slow DNS error = %v, want deadline exceeded", err)
	}
	if lookupContext.Err() == nil {
		t.Fatal("DNS context remained active after timeout")
	}
}

func TestResolveIPv4PreservesEarlierDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	wantDeadline, _ := ctx.Deadline()
	_, err := resolveIPv4Context(ctx, "slow.example", func(lookupCtx context.Context, _, _ string) ([]net.IP, error) {
		if deadline, _ := lookupCtx.Deadline(); deadline != wantDeadline {
			t.Fatal("DNS lookup extended the caller deadline")
		}
		<-lookupCtx.Done()
		return nil, lookupCtx.Err()
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want deadline exceeded", err)
	}
}

func TestResolveIPv4ReleasesDNSContextAfterSuccess(t *testing.T) {
	var lookupContext context.Context
	address, err := resolveIPv4Context(context.Background(), " fast.example ", func(ctx context.Context, _, host string) ([]net.IP, error) {
		lookupContext = ctx
		if host != "fast.example" {
			t.Fatalf("hostname = %q", host)
		}
		return []net.IP{net.ParseIP("192.0.2.1")}, nil
	})
	if err != nil || address.String() != "192.0.2.1" {
		t.Fatalf("result = %v, %v", address, err)
	}
	if !errors.Is(lookupContext.Err(), context.Canceled) {
		t.Fatal("successful lookup did not release its timeout context")
	}
}

func TestResolveIPv4LiteralSkipsDNS(t *testing.T) {
	address, err := resolveIPv4Context(context.Background(), "127.0.0.1", func(context.Context, string, string) ([]net.IP, error) {
		t.Fatal("literal address used DNS")
		return nil, nil
	})
	if err != nil || address.String() != "127.0.0.1" {
		t.Fatalf("result = %v, %v", address, err)
	}
}

func TestResolveIPv4DiscardsResultAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	address, err := resolveIPv4Context(ctx, "fast.example", func(context.Context, string, string) ([]net.IP, error) {
		cancel()
		return []net.IP{net.ParseIP("192.0.2.1")}, nil
	})
	if address != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled result = %v, %v", address, err)
	}
}

func TestResolveIPv4CancelsResolverDial(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	dialFinished := make(chan struct{}, 1)
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(dialCtx context.Context, _, _ string) (net.Conn, error) {
			<-dialCtx.Done()
			select {
			case dialFinished <- struct{}{}:
			default:
			}
			return nil, dialCtx.Err()
		},
	}
	address, err := resolveIPv4Context(ctx, "slow.example.", resolver.LookupIP)
	if address != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("slow resolver result = %v, %v", address, err)
	}
	select {
	case <-dialFinished:
	case <-time.After(time.Second):
		t.Fatal("resolver dial did not stop after cancellation")
	}
}
