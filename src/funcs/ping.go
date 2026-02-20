package funcs

import (
	"fmt"
	"net"
	"smartping/src/g"
	"smartping/src/nettools"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	defaultPingCount      = 20
	defaultPingIntervalMs = 3000
)

var pingRunning int32

func Ping() {
	if !atomic.CompareAndSwapInt32(&pingRunning, 0, 1) {
		logrus.Warn("[func:Ping] Previous round still running, skip")
		return
	}
	defer atomic.StoreInt32(&pingRunning, 0)

	var wg sync.WaitGroup
	for _, target := range g.SelfCfg.Ping {
		wg.Add(1)
		go PingTask(g.Cfg.Network[target], &wg)
	}
	wg.Wait()
	go StartAlert()
}

// ping main function
func PingTask(t g.NetworkMember, wg *sync.WaitGroup) {
	logrus.Info("Start Ping " + t.Addr + "..")
	stat := g.PingSt{}
	stat.MinDelay = -1
	lossPK := 0
	pingCount := g.GetBaseInt("PingCount", defaultPingCount)
	pingInterval := time.Duration(g.GetBaseInt("PingIntervalMs", defaultPingIntervalMs)) * time.Millisecond
	ipaddr, err := net.ResolveIPAddr("ip", t.Addr)
	if err == nil {
		for i := 0; i < pingCount; i++ {
			starttime := time.Now().UnixNano()
			delay, err := nettools.RunPing(ipaddr, 3*time.Second, 64, i)
			if err == nil {
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
				logrus.Debug("[func:StartPing IcmpPing] ID:", i, " IP:", t.Addr, "| err:", err)
				lossPK = lossPK + 1
			}
			stat.SendPk = stat.SendPk + 1
			stat.LossPk = int((float64(lossPK) / float64(stat.SendPk)) * 100)
			duringtime := time.Now().UnixNano() - starttime
			sleepFor := pingInterval - time.Duration(duringtime)*time.Nanosecond
			if sleepFor > 0 {
				time.Sleep(sleepFor)
			}
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
	PingStorage(stat, t.Addr)
	wg.Done()
	logrus.Info("Finish Ping " + t.Addr + "..")
}

// storage ping data
func PingStorage(pingres g.PingSt, Addr string) {
	logtime := time.Now().Format("2006-01-02 15:04")
	logrus.Info("[func:StartPing] ", "(", logtime, ")Starting PingStorage ", Addr)
	sql := "INSERT INTO [pinglog] (logtime, target, maxdelay, mindelay, avgdelay, sendpk, revcpk, losspk) values(?, ?, ?, ?, ?, ?, ?, ?)"
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
