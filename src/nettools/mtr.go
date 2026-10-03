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
	dest, err := ResolveIPv4Context(ctx, Addr)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return result, ctxErr
		}
		return result, errors.New("Unable to resolve destination host")
	}
	return runMtrProbesContext(ctx, maxttl, maxtimeout, mtrProbeInterval, func(ctx context.Context, ttl int) ICMP {
		return sendMtrProbeContext(ctx, dest, maxrtt, ttl)
	})
}

func sendMtrProbeContext(ctx context.Context, dest net.Addr, maxrtt time.Duration, ttl int) ICMP {
	id := randomUint16()
	seq := nextICMPSequence()
	res := pkg{
		maxrtt: maxrtt,
		id:     id,
		seq:    seq,
		msg:    icmp.Message{Type: ipv4.ICMPTypeEcho, Code: 0, Body: &icmp.Echo{ID: id, Seq: seq}},
		dest:   dest,
	}
	var err error
	res.netmsg, err = res.msg.Marshal(nil)
	if err != nil {
		return ICMP{Error: err}
	}
	return res.SendContext(ctx, ttl)
}

func runMtrProbesContext(ctx context.Context, maxttl, maxtimeout int, interval time.Duration, probe func(context.Context, int) ICMP) ([]Mtr, error) {
	result := []Mtr{}
	probeCtx, cancelProbes := context.WithCancel(ctx)
	defer cancelProbes()
	probeErrors := make(chan error, 1)
	reportProbeError := func(err error) {
		select {
		case probeErrors <- err:
			cancelProbes()
		default:
		}
	}
	resultLock := sync.Mutex{}
	var wg sync.WaitGroup
	mtr := map[int][]ICMP{}
	timeouts := 0
	for ttl := 1; ttl <= maxttl; ttl++ {
		if probeCtx.Err() != nil {
			break
		}
		discoveryStarted := time.Now()
		next := probe(probeCtx, ttl)
		if ctx.Err() != nil {
			break
		}
		if next.Error != nil {
			reportProbeError(next.Error)
			break
		}
		resultLock.Lock()
		var stop bool
		timeouts, stop = recordMtrDiscoveryProbe(mtr, ttl, next, timeouts, maxtimeout)
		resultLock.Unlock()
		if stop {
			break
		}
		wg.Add(1)
		go func(ittl int, nextProbe time.Time) {
			defer wg.Done()
			for j := 1; j < mtrProbeCount; j++ {
				if waitForContext(probeCtx, time.Until(nextProbe)) != nil {
					return
				}
				if probeCtx.Err() != nil {
					return
				}
				// Include discovery in the cadence, then schedule from each
				// actual start so slow probes never build up overdue sends.
				nextProbe = time.Now().Add(interval)
				next := probe(probeCtx, ittl)
				if next.Error != nil {
					if probeCtx.Err() == nil {
						reportProbeError(next.Error)
					}
					return
				}
				if probeCtx.Err() != nil {
					return
				}
				resultLock.Lock()
				mtr[ittl] = append(mtr[ittl], next)
				resultLock.Unlock()
			}
		}(ttl, discoveryStarted.Add(interval))
		if isTerminalMtrResponse(next) {
			break
		}
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return result, err
	}
	select {
	case err := <-probeErrors:
		return result, err
	default:
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

func recordMtrDiscoveryProbe(results map[int][]ICMP, ttl int, response ICMP, consecutiveTimeouts, timeoutLimit int) (int, bool) {
	results[ttl] = append(results[ttl], response)
	if response.Timeout {
		consecutiveTimeouts++
	} else {
		consecutiveTimeouts = 0
	}
	return consecutiveTimeouts, consecutiveTimeouts >= timeoutLimit
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
