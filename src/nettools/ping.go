package nettools

import (
	"bytes"
	"errors"
	"net"
	"sync/atomic"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

var seqCounter uint32

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

func RunPing(IpAddr *net.IPAddr, maxrtt time.Duration, maxttl int, seq int) (float64, error) {
	id := randomUint16()
	uniqueSeq := int(atomic.AddUint32(&seqCounter, 1) & 0xFFFF)
	msg := icmp.Message{Type: ipv4.ICMPTypeEcho, Code: 0, Body: &icmp.Echo{ID: id, Seq: uniqueSeq, Data: bytes.Repeat([]byte("Go Smart Ping!"), 4)}}
	netmsg, err := msg.Marshal(nil)
	if err != nil {
		return 0, err
	}
	result := pool.sendICMP(id, uniqueSeq, maxttl, netmsg, IpAddr, maxrtt)
	if result.Timeout {
		return 0, errors.New("request timeout")
	}
	if result.Down {
		return 0, result.Error
	}
	return float64(result.RTT.Nanoseconds()) / 1e6, result.Error
}
