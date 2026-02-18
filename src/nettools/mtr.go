package nettools

import (
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

func RunMtr(Addr string, maxrtt time.Duration, maxttl int, maxtimeout int) ([]Mtr, error) {
	result := []Mtr{}
	Lock := sync.Mutex{}
	var wg sync.WaitGroup
	mtr := map[int][]ICMP{}
	var err error
	timeouts := 0
	for ttl := 1; ttl <= maxttl; ttl++ {
		id := randomUint16()
		seq := randomUint16()
		res := pkg{
			maxrtt: maxrtt,
			id:     id,
			seq:    seq,
			msg:    icmp.Message{Type: ipv4.ICMPTypeEcho, Code: 0, Body: &icmp.Echo{ID: id, Seq: seq}},
		}
		res.dest, err = net.ResolveIPAddr("ip", Addr)
		if err != nil {
			return result, errors.New("Unable to resolve destination host")
		}
		res.netmsg, err = res.msg.Marshal(nil)
		if nil != err {
			return result, err
		}
		next := res.Send(ttl)
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
			for j := 1; j < 10; j++ {
				id := randomUint16()
				seq := randomUint16()
				res := pkg{
					maxrtt: maxrtt,
					id:     id,
					seq:    seq,
					msg:    icmp.Message{Type: ipv4.ICMPTypeEcho, Code: 0, Body: &icmp.Echo{ID: id, Seq: seq}},
				}
				dest, resolveErr := net.ResolveIPAddr("ip", Addr)
				if resolveErr != nil {
					Lock.Lock()
					mtr[ittl] = append(mtr[ittl], ICMP{Timeout: true, Error: resolveErr})
					Lock.Unlock()
					continue
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
				next := res.Send(ittl)
				Lock.Lock()
				mtr[ittl] = append(mtr[ittl], next)
				Lock.Unlock()
				sleepFor := time.Second - time.Since(nowTime)
				if sleepFor > 0 {
					time.Sleep(sleepFor)
				}
			}
		}(ttl)
		if next.Final {
			break
		}
	}
	wg.Wait()
	for i := 1; i <= len(mtr); i++ {
		vals, ok := mtr[i]
		if !ok || len(vals) == 0 {
			continue
		}
		imtr := Mtr{}
		for id, val := range vals {
			if val.Addr != nil {
				imtr.Host = val.Addr.String()
			} else {
				if imtr.Host == "" {
					imtr.Host = "???"
				}
			}
			imtr.Send += 1
			if val.Timeout {
				imtr.Loss += 1
			} else if val.Error != nil {
				imtr.Loss += 1
			} else {
				if imtr.Wrst < val.RTT {
					imtr.Wrst = val.RTT
				}
				if id == 0 {
					imtr.Best = val.RTT
				}
				if imtr.Best > val.RTT {
					imtr.Best = val.RTT
				}
				imtr.Avg += val.RTT
				imtr.Last = val.RTT
			}
		}
		if (imtr.Send - imtr.Loss) > 0 {
			imtr.Avg = imtr.Avg / time.Duration(imtr.Send-imtr.Loss)
			for _, val := range vals {
				if !val.Timeout && val.Error == nil {
					v := (float64(val.RTT.Nanoseconds()) / 1e6) - (float64(imtr.Avg.Nanoseconds()) / 1e6)
					imtr.StDev += v * v
				}
			}
			imtr.StDev = math.Sqrt(imtr.StDev / float64(imtr.Send-imtr.Loss))
		}
		result = append(result, imtr)

	}
	return result, nil
}
