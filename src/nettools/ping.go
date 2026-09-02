package nettools

import (
	"bytes"
	"context"
	"errors"
	"net"
	"sync/atomic"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

var seqCounter uint32

func nextICMPSequence() int {
	return int(atomic.AddUint32(&seqCounter, 1) & 0xFFFF)
}

type pkg struct {
	msg    icmp.Message
	netmsg []byte
	id     int
	seq    int
	maxrtt time.Duration
	dest   net.Addr
}

type ICMP struct {
	Addr    net.Addr
	RTT     time.Duration
	MaxRTT  time.Duration
	MinRTT  time.Duration
	AvgRTT  time.Duration
	Final   bool
	Timeout bool
	Down    bool
	Error   error
}

func (t *pkg) Send(ttl int) ICMP {
	return pool.sendICMP(t.id, t.seq, ttl, t.netmsg, t.dest, t.maxrtt)
}

func (t *pkg) SendContext(ctx context.Context, ttl int) ICMP {
	return pool.sendICMPContext(ctx, t.id, t.seq, ttl, t.netmsg, t.dest, t.maxrtt)
}

func RunPing(IpAddr *net.IPAddr, maxrtt time.Duration, maxttl int, seq int) (float64, error) {
	return RunPingContext(context.Background(), IpAddr, maxrtt, maxttl, seq)
}

func RunPingContext(ctx context.Context, IpAddr *net.IPAddr, maxrtt time.Duration, maxttl int, seq int) (float64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if IpAddr == nil || IpAddr.IP.To4() == nil {
		return 0, errors.New("invalid IPv4 destination")
	}
	if maxrtt <= 0 {
		return 0, errors.New("invalid ping timeout")
	}
	if maxttl < 1 || maxttl > 255 {
		return 0, errors.New("invalid IPv4 TTL")
	}
	id := randomUint16()
	uniqueSeq := nextICMPSequence()
	msg := icmp.Message{Type: ipv4.ICMPTypeEcho, Code: 0, Body: &icmp.Echo{ID: id, Seq: uniqueSeq, Data: bytes.Repeat([]byte("Go Smart Ping!"), 4)}}
	netmsg, err := msg.Marshal(nil)
	if err != nil {
		return 0, err
	}
	result := pool.sendICMPContext(ctx, id, uniqueSeq, maxttl, netmsg, IpAddr, maxrtt)
	return evaluatePingResult(result)
}

func evaluatePingResult(result ICMP) (float64, error) {
	if result.Timeout {
		return 0, errors.New("request timeout")
	}
	if result.Down {
		return 0, errors.New("destination unreachable")
	}
	if result.Error != nil {
		return 0, result.Error
	}
	if !result.Final {
		return 0, errors.New("destination not reached")
	}
	return float64(result.RTT.Nanoseconds()) / 1e6, nil
}
