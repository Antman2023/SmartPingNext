package funcs

import (
	"fmt"
	"net"
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

func resolvePingRoundConfig() (int, time.Duration, time.Duration, time.Duration) {
	pingCount := g.GetBaseInt("PingCount", defaultPingCount)
	pingInterval := time.Duration(g.GetBaseInt("PingIntervalMs", defaultPingIntervalMs)) * time.Millisecond
	pingTimeout := time.Duration(g.GetBaseInt("PingTimeoutMs", defaultPingTimeoutMs)) * time.Millisecond
	pingStagger := time.Duration(g.GetBaseInt("PingStaggerMs", defaultPingStaggerMs)) * time.Millisecond

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

func Ping() {
	roundTime := time.Now().Truncate(time.Minute)
	runPingRound(roundTime)
}

func runPingRound(roundTime time.Time) {
	pingCount, pingInterval, pingTimeout, pingStagger := resolvePingRoundConfig()
	logtime := roundTime.Format("2006-01-02 15:04")

	var wg sync.WaitGroup
	validIndex := 0
	for _, target := range g.SelfCfg.Ping {
		t, ok := g.Cfg.Network[target]
		if !ok || strings.TrimSpace(t.Addr) == "" {
			logrus.Warnf("[func:Ping] Skip invalid ping target: %q", target)
			continue
		}
		targetOffset := time.Duration(validIndex) * pingStagger
		validIndex++
		wg.Add(1)
		go PingTask(t, pingCount, pingInterval, pingTimeout, targetOffset, roundTime, logtime, &wg)
	}
	wg.Wait()
	go StartAlert()
}

// ping main function
func PingTask(t g.NetworkMember, pingCount int, pingInterval time.Duration, pingTimeout time.Duration, targetOffset time.Duration, roundTime time.Time, logtime string, wg *sync.WaitGroup) {
	defer wg.Done()

	logrus.Info("Start Ping " + t.Addr + "..")
	stat := g.PingSt{}
	stat.MinDelay = -1
	lossPK := 0
	ipaddr, err := net.ResolveIPAddr("ip", t.Addr)
	roundStart := roundTime.Add(targetOffset)
	if err == nil {
		for i := 0; i < pingCount; i++ {
			nextTick := roundStart.Add(time.Duration(i) * pingInterval)
			sleepFor := time.Until(nextTick)
			if sleepFor > 0 {
				time.Sleep(sleepFor)
			}

			delay, pingErr := nettools.RunPing(ipaddr, pingTimeout, 64, i)
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
	PingStorage(stat, t.Addr, logtime)
	logrus.Info("Finish Ping " + t.Addr + "..")
}

// storage ping data
func PingStorage(pingres g.PingSt, Addr string, logtime string) {
	if strings.TrimSpace(logtime) == "" {
		logtime = time.Now().Format("2006-01-02 15:04")
	}
	logrus.Info("[func:StartPing] ", "(", logtime, ")Starting PingStorage ", Addr)
	sql := "INSERT INTO [pinglog] (logtime, target, maxdelay, mindelay, avgdelay, sendpk, revcpk, losspk) values(?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(logtime, target) DO UPDATE SET maxdelay=excluded.maxdelay, mindelay=excluded.mindelay, avgdelay=excluded.avgdelay, sendpk=excluded.sendpk, revcpk=excluded.revcpk, losspk=excluded.losspk"
	logrus.Debug("[func:StartPing] ", sql)
	g.DLock.Lock()
	_, err := g.Db.Exec(sql, logtime, Addr,
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
