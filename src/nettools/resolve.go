package nettools

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"
)

const dnsLookupTimeout = 5 * time.Second

// ResolveIPv4Context resolves an IPv4 literal or host name and honors cancellation.
// Hostname lookups have a bounded wait even for background monitoring tasks.
func ResolveIPv4Context(ctx context.Context, address string) (*net.IPAddr, error) {
	return resolveIPv4Context(ctx, address, net.DefaultResolver.LookupIP)
}

func resolveIPv4Context(ctx context.Context, address string, lookup func(context.Context, string, string) ([]net.IP, error)) (*net.IPAddr, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	host := strings.TrimSpace(address)
	if host == "" {
		return nil, errors.New("IPv4 address is empty")
	}
	if parsed := net.ParseIP(host); parsed != nil {
		ipv4Address := parsed.To4()
		if ipv4Address == nil {
			return nil, errors.New("address has no IPv4 representation")
		}
		return &net.IPAddr{IP: ipv4Address}, nil
	}

	lookupCtx, cancel := context.WithTimeout(ctx, dnsLookupTimeout)
	defer cancel()
	addresses, err := lookup(lookupCtx, "ip4", host)
	if ctxErr := lookupCtx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil {
		return nil, err
	}
	for _, resolved := range addresses {
		if ipv4Address := resolved.To4(); ipv4Address != nil {
			return &net.IPAddr{IP: ipv4Address}, nil
		}
	}
	return nil, errors.New("no IPv4 address found")
}
