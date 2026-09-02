package nettools

import (
	"context"
	"errors"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"math"
	"net"
	"sync"
	"time"
)

type Mtr struct {
	Host  string
	Send  int
	Loss  int
	Last  time.Duration
	Avg   time.Duration
	Best  time.Duration
	Wrst  time.Duration
	StDev float64
}

const (
	mtrProbeCount    = 10
	mtrProbeInterval = time.Second
)

func RunMtr(Addr string, maxrtt time.Duration, maxttl int, maxtimeout int) ([]Mtr, error) {
	return RunMtrContext(context.Background(), Addr, maxrtt, maxttl, maxtimeout)
}

func RunMtrContext(ctx context.Context, Addr string, maxrtt time.Duration, maxttl int, maxtimeout int) ([]Mtr, error) {
	result := []Mtr{}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if maxttl <= 0 {
		return result, nil
	}
	if maxttl > 255 {
		return result, errors.New("Invalid maximum TTL")
	}
	if maxrtt <= 0 {
		return result, errors.New("Invalid probe timeout")
	}
	if maxtimeout <= 0 {
		return result, errors.New("Invalid maximum consecutive timeouts")
	}
	dest, err := resolveIPv4Context(ctx, Addr)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return result, ctxErr
		}
		return result, errors.New("Unable to resolve destination host")
	}
	Lock := sync.Mutex{}
	var wg sync.WaitGroup
	mtr := map[int][]ICMP{}
	timeouts := 0
	for ttl := 1; ttl <= maxttl; ttl++ {
		if ctx.Err() != nil {
			break
		}
		id := randomUint16()
		seq := nextICMPSequence()
		res := pkg{
			maxrtt: maxrtt,
			id:     id,
			seq:    seq,
			msg:    icmp.Message{Type: ipv4.ICMPTypeEcho, Code: 0, Body: &icmp.Echo{ID: id, Seq: seq}},
			dest:   dest,
		}
		res.netmsg, err = res.msg.Marshal(nil)
		if nil != err {
			return result, err
		}
		next := res.SendContext(ctx, ttl)
		if ctx.Err() != nil {
			break
		}
		if next.Timeout {
			timeouts++
		} else {
			timeouts = 0
		}
		if timeouts == maxtimeout {
			break
		}
		Lock.Lock()
		mtr[ttl] = append(mtr[ttl], next)
		Lock.Unlock()
		wg.Add(1)
		go func(ittl int) {
			defer wg.Done()
			for j := 1; j < mtrProbeCount; j++ {
				if ctx.Err() != nil {
					return
				}
				id := randomUint16()
				seq := nextICMPSequence()
				res := pkg{
					maxrtt: maxrtt,
					id:     id,
					seq:    seq,
					msg:    icmp.Message{Type: ipv4.ICMPTypeEcho, Code: 0, Body: &icmp.Echo{ID: id, Seq: seq}},
				}
				res.dest = dest
				netmsg, marshalErr := res.msg.Marshal(nil)
				if marshalErr != nil {
					Lock.Lock()
					mtr[ittl] = append(mtr[ittl], ICMP{Timeout: true, Error: marshalErr})
					Lock.Unlock()
					continue
				}
				res.netmsg = netmsg
				nowTime := time.Now()
				next := res.SendContext(ctx, ittl)
				if ctx.Err() != nil {
					return
				}
				Lock.Lock()
				mtr[ittl] = append(mtr[ittl], next)
				Lock.Unlock()
				if j < mtrProbeCount-1 {
					sleepFor := mtrProbeInterval - time.Since(nowTime)
					if sleepFor > 0 && waitForContext(ctx, sleepFor) != nil {
						return
					}
				}
			}
		}(ttl)
		if isTerminalMtrResponse(next) {
			break
		}
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return result, err
	}
	for i := 1; i <= len(mtr); i++ {
		vals, ok := mtr[i]
		if !ok || len(vals) == 0 {
			continue
		}
		result = append(result, summarizeMtr(vals))

	}
	return result, nil
}

func resolveIPv4Context(ctx context.Context, address string) (*net.IPAddr, error) {
	if parsed := net.ParseIP(address); parsed != nil {
		if ipv4Address := parsed.To4(); ipv4Address != nil {
			return &net.IPAddr{IP: ipv4Address}, nil
		}
		return nil, errors.New("not an IPv4 address")
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, address)
	if err != nil {
		return nil, err
	}
	for _, resolved := range addresses {
		if ipv4Address := resolved.IP.To4(); ipv4Address != nil {
			return &net.IPAddr{IP: ipv4Address, Zone: resolved.Zone}, nil
		}
	}
	return nil, errors.New("no IPv4 address found")
}

func waitForContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func isTerminalMtrResponse(response ICMP) bool {
	return response.Final || response.Down
}

func summarizeMtr(vals []ICMP) Mtr {
	imtr := Mtr{}
	received := 0
	for _, val := range vals {
		if val.Addr != nil {
			imtr.Host = val.Addr.String()
		} else if imtr.Host == "" {
			imtr.Host = "???"
		}
		imtr.Send++
		if val.Timeout || val.Error != nil {
			imtr.Loss++
			continue
		}

		if imtr.Wrst < val.RTT {
			imtr.Wrst = val.RTT
		}
		if received == 0 || imtr.Best > val.RTT {
			imtr.Best = val.RTT
		}
		imtr.Avg += val.RTT
		imtr.Last = val.RTT
		received++
	}

	if received > 0 {
		imtr.Avg /= time.Duration(received)
		for _, val := range vals {
			if !val.Timeout && val.Error == nil {
				v := (float64(val.RTT.Nanoseconds()) / 1e6) - (float64(imtr.Avg.Nanoseconds()) / 1e6)
				imtr.StDev += v * v
			}
		}
		imtr.StDev = math.Sqrt(imtr.StDev / float64(received))
	}
	return imtr
}
