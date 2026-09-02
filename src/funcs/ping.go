package funcs

import (
	"context"
	"fmt"
	"smartping/src/g"
	"smartping/src/nettools"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	defaultPingCount      = 20
	defaultPingIntervalMs = 3000
	defaultPingTimeoutMs  = 3000
	defaultPingStaggerMs  = 100
)

type boundedJobGate struct {
	mu      sync.Mutex
	running bool
	waiter  *boundedJobWaiter
}

type boundedJobWaiter struct {
	ready   chan struct{}
	granted bool
}

func newBoundedJobGate() *boundedJobGate {
	return &boundedJobGate{}
}

func (gate *boundedJobGate) acquire(ctx context.Context) (acquired bool, queued bool) {
	if ctx.Err() != nil {
		return false, false
	}

	gate.mu.Lock()
	if ctx.Err() != nil {
		gate.mu.Unlock()
		return false, false
	}
	if !gate.running {
		gate.running = true
		gate.mu.Unlock()
		return true, false
	}
	if gate.waiter != nil {
		gate.mu.Unlock()
		return false, false
	}
	waiter := &boundedJobWaiter{ready: make(chan struct{})}
	gate.waiter = waiter
	gate.mu.Unlock()

	select {
	case <-waiter.ready:
		return true, true
	case <-ctx.Done():
		gate.mu.Lock()
		if gate.waiter == waiter {
			gate.waiter = nil
		}
		granted := waiter.granted
		gate.mu.Unlock()
		// If release won the race, accept the handoff so the caller can release
		// the slot even though its context has already been canceled.
		return granted, true
	}
}

func (gate *boundedJobGate) release() {
	gate.mu.Lock()
	defer gate.mu.Unlock()
	if !gate.running {
		panic("boundedJobGate: release without acquire")
	}
	if gate.waiter == nil {
		gate.running = false
		return
	}

	waiter := gate.waiter
	gate.waiter = nil
	waiter.granted = true
	close(waiter.ready)
}

var pingRoundGate = newBoundedJobGate()

func boundedBaseInt(config g.Config, key string, defaultValue int, minValue int, maxValue int) int {
	value, ok := config.Base[key]
	if !ok || value < minValue || value > maxValue {
		return defaultValue
	}
	return value
}

func resolvePingRoundConfig(config g.Config) (int, time.Duration, time.Duration, time.Duration) {
	pingCount := boundedBaseInt(config, "PingCount", defaultPingCount, 1, 120)
	pingInterval := time.Duration(boundedBaseInt(config, "PingIntervalMs", defaultPingIntervalMs, 100, 60000)) * time.Millisecond
	pingTimeout := time.Duration(boundedBaseInt(config, "PingTimeoutMs", defaultPingTimeoutMs, 100, 60000)) * time.Millisecond
	pingStagger := time.Duration(boundedBaseInt(config, "PingStaggerMs", defaultPingStaggerMs, 0, 60000)) * time.Millisecond

	if pingTimeout <= 0 {
		pingTimeout = defaultPingTimeoutMs * time.Millisecond
	}
	if pingStagger < 0 {
		pingStagger = 0
	}
	if pingStagger >= pingInterval {
		invalidStagger := pingStagger
		pingStagger = 0
		logrus.Warnf("[func:Ping] PingStaggerMs(%dms) >= PingIntervalMs(%dms), disable stagger", invalidStagger.Milliseconds(), pingInterval.Milliseconds())
	}
	if pingTimeout > pingInterval {
		logrus.Warnf("[func:Ping] PingTimeoutMs(%dms) > PingIntervalMs(%dms), schedule drift risk may increase", pingTimeout.Milliseconds(), pingInterval.Milliseconds())
	}
	return pingCount, pingInterval, pingTimeout, pingStagger
}

func pingTargetOffset(index int, stagger, interval time.Duration) time.Duration {
	if index <= 0 || stagger <= 0 || interval <= 0 {
		return 0
	}
	return (time.Duration(index) * stagger) % interval
}

func Ping() {
	PingContext(context.Background())
}

func PingContext(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	acquired, queued := pingRoundGate.acquire(ctx)
	if !acquired {
		if ctx.Err() == nil {
			logrus.Warn("[func:Ping] Active and queued rounds still running, skip latest trigger")
		}
		return
	}
	if queued {
		logrus.Info("[func:Ping] Starting delayed round after previous round completed")
	}
	roundTime := time.Now().Truncate(time.Minute)
	func() {
		defer pingRoundGate.release()
		runPingRoundContext(ctx, roundTime)
	}()
	if ctx.Err() != nil {
		return
	}
	StartAlertContext(ctx)
}

func runPingRound(roundTime time.Time) {
	runPingRoundContext(context.Background(), roundTime)
}

func runPingRoundContext(ctx context.Context, roundTime time.Time) {
	config := g.ConfigSnapshot()
	pingCount, pingInterval, pingTimeout, pingStagger := resolvePingRoundConfig(config)
	logtime := roundTime.Format("2006-01-02 15:04")
	selfConfig := config.Network[config.Addr]

	var wg sync.WaitGroup
	validIndex := 0
	for _, target := range selfConfig.Ping {
		if ctx.Err() != nil {
			break
		}
		t, ok := config.Network[target]
		if !ok || strings.TrimSpace(t.Addr) == "" {
			logrus.Warnf("[func:Ping] Skip invalid ping target: %q", target)
			continue
		}
		targetOffset := pingTargetOffset(validIndex, pingStagger, pingInterval)
		validIndex++
		wg.Add(1)
		go PingTaskContext(ctx, t, pingCount, pingInterval, pingTimeout, targetOffset, roundTime, logtime, &wg)
	}
	wg.Wait()
}

// ping main function
func PingTask(t g.NetworkMember, pingCount int, pingInterval time.Duration, pingTimeout time.Duration, targetOffset time.Duration, roundTime time.Time, logtime string, wg *sync.WaitGroup) {
	PingTaskContext(context.Background(), t, pingCount, pingInterval, pingTimeout, targetOffset, roundTime, logtime, wg)
}

func PingTaskContext(ctx context.Context, t g.NetworkMember, pingCount int, pingInterval time.Duration, pingTimeout time.Duration, targetOffset time.Duration, roundTime time.Time, logtime string, wg *sync.WaitGroup) {
	defer wg.Done()

	logrus.Info("Start Ping " + t.Addr + "..")
	stat := g.PingSt{}
	stat.MinDelay = -1
	lossPK := 0
	ipaddr, err := nettools.ResolveIPv4Context(ctx, t.Addr)
	roundStart := roundTime.Add(targetOffset)
	if err == nil {
		for i := 0; i < pingCount; i++ {
			nextTick := roundStart.Add(time.Duration(i) * pingInterval)
			sleepFor := time.Until(nextTick)
			if sleepFor > 0 && sleepContext(ctx, sleepFor) != nil {
				logrus.Info("Cancel Ping " + t.Addr + "..")
				return
			}

			delay, pingErr := nettools.RunPingContext(ctx, ipaddr, pingTimeout, 64, i)
			if ctx.Err() != nil {
				logrus.Info("Cancel Ping " + t.Addr + "..")
				return
			}
			if pingErr == nil {
				stat.AvgDelay = stat.AvgDelay + delay
				if stat.MaxDelay < delay {
					stat.MaxDelay = delay
				}
				if stat.MinDelay == -1 || stat.MinDelay > delay {
					stat.MinDelay = delay
				}
				stat.RevcPk = stat.RevcPk + 1
				logrus.Debug("[func:StartPing IcmpPing] ID:", i, " IP:", t.Addr)
			} else {
				logrus.Debug("[func:StartPing IcmpPing] ID:", i, " IP:", t.Addr, "| err:", pingErr)
				lossPK = lossPK + 1
			}
			stat.SendPk = stat.SendPk + 1
			stat.LossPk = int((float64(lossPK) / float64(stat.SendPk)) * 100)
		}
		if stat.RevcPk > 0 {
			stat.AvgDelay = stat.AvgDelay / float64(stat.RevcPk)
		} else {
			stat.AvgDelay = 0.0
			stat.MinDelay = 0.0
		}
		logrus.Debug("[func:IcmpPing] Finish Addr:", t.Addr, " MaxDelay:", stat.MaxDelay, " MinDelay:", stat.MinDelay, " AvgDelay:", stat.AvgDelay, " Revc:", stat.RevcPk, " LossPK:", stat.LossPk)
	} else {
		stat.AvgDelay = 0.00
		stat.MinDelay = 0.00
		stat.MaxDelay = 0.00
		stat.SendPk = 0
		stat.RevcPk = 0
		stat.LossPk = 100
		logrus.Debug("[func:IcmpPing] Finish Addr:", t.Addr, " Unable to resolve destination host")
	}
	if ctx.Err() != nil {
		return
	}
	PingStorageContext(ctx, stat, t.Addr, logtime)
	logrus.Info("Finish Ping " + t.Addr + "..")
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// storage ping data
func PingStorage(pingres g.PingSt, Addr string, logtime string) {
	PingStorageContext(context.Background(), pingres, Addr, logtime)
}

func PingStorageContext(ctx context.Context, pingres g.PingSt, Addr string, logtime string) {
	if ctx.Err() != nil {
		return
	}
	if strings.TrimSpace(logtime) == "" {
		logtime = time.Now().Format("2006-01-02 15:04")
	}
	logrus.Info("[func:StartPing] ", "(", logtime, ")Starting PingStorage ", Addr)
	sql := "INSERT INTO [pinglog] (logtime, target, maxdelay, mindelay, avgdelay, sendpk, revcpk, losspk) values(?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(logtime, target) DO UPDATE SET maxdelay=excluded.maxdelay, mindelay=excluded.mindelay, avgdelay=excluded.avgdelay, sendpk=excluded.sendpk, revcpk=excluded.revcpk, losspk=excluded.losspk"
	logrus.Debug("[func:StartPing] ", sql)
	g.DLock.Lock()
	if ctx.Err() != nil {
		g.DLock.Unlock()
		return
	}
	_, err := g.Db.ExecContext(ctx, sql, logtime, Addr,
		fmt.Sprintf("%.2f", pingres.MaxDelay),
		fmt.Sprintf("%.2f", pingres.MinDelay),
		fmt.Sprintf("%.2f", pingres.AvgDelay),
		pingres.SendPk, pingres.RevcPk, pingres.LossPk)
	if err != nil {
		logrus.Error("[func:StartPing] Sql Error ", err)
	}
	g.DLock.Unlock()
	logrus.Info("[func:StartPing] ", "(", logtime, ") Finish PingStorage  ", Addr)
}
