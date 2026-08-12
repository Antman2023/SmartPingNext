package funcs

import (
	"encoding/json"
	"fmt"
	"net"
	"smartping/src/g"
	"smartping/src/nettools"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	MapLock   = new(sync.Mutex)
	MapStatus map[string][]g.MapVal
)

const (
	defaultMappingProbeCount  = 3
	defaultMappingConcurrency = 8
)

var mappingRunning int32

func Mapping() {
	if !atomic.CompareAndSwapInt32(&mappingRunning, 0, 1) {
		logrus.Warn("[func:Mapping] Previous round still running, skip")
		return
	}
	defer atomic.StoreInt32(&mappingRunning, 0)

	var wg sync.WaitGroup
	config := g.ConfigSnapshot()
	workerLimit := boundedBaseInt(config, "MappingConcurrency", defaultMappingConcurrency, 1, 64)
	probeCount := boundedBaseInt(config, "MappingProbeCount", defaultMappingProbeCount, 1, 20)
	sem := make(chan struct{}, workerLimit)
	MapLock.Lock()
	MapStatus = map[string][]g.MapVal{}
	MapLock.Unlock()
	logrus.Debug("[func:Mapping]", config.Chinamap)
	for tel, provDetail := range config.Chinamap {
		for prov := range provDetail {
			ips := provDetail[prov]
			logrus.Debug("[func:Mapping]", ips)
			if len(ips) > 0 {
				wg.Add(1)
				sem <- struct{}{}
				go func(tel, prov string, ips []string) {
					defer func() { <-sem }()
					MappingTask(tel, prov, ips, probeCount, &wg)
				}(tel, prov, ips)
			}
		}
	}
	wg.Wait()
	MapPingStorage()
}

// ping main function
func MappingTask(tel string, prov string, ips []string, probeCount int, wg *sync.WaitGroup) {
	defer wg.Done()
	logrus.Info("Start MappingTask " + tel + " " + prov + "..")
	statMap := []g.PingSt{}
	for _, ip := range ips {
		logrus.Debug("[func:StartChinaMapPing]", ip)
		ipaddr, err := net.ResolveIPAddr("ip", ip)
		if err == nil {
			for i := 0; i < probeCount; i++ {
				stat := g.PingSt{}
				stat.MinDelay = -1
				stat.LossPk = 0
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
					logrus.Debug("[func:StartChinaMapPing IcmpPing] ID:", i, " IP:", ip)
				} else {
					logrus.Debug("[func:StartChinaMapPing IcmpPing] ID:", i, " IP:", ip, " | ", err)
					stat.LossPk = stat.LossPk + 1
				}
				stat.SendPk = stat.SendPk + 1
				stat.LossPk = int((float64(stat.LossPk) / float64(stat.SendPk)) * 100)
				if stat.RevcPk > 0 {
					stat.AvgDelay = stat.AvgDelay / float64(stat.RevcPk)
				} else {
					stat.AvgDelay = 2000
				}
				statMap = append(statMap, stat)
			}
		} else {
			stat := g.PingSt{}
			stat.AvgDelay = 2000.00
			stat.MinDelay = 2000.00
			stat.MaxDelay = 2000.00
			stat.SendPk = 0
			stat.RevcPk = 0
			stat.LossPk = 100
			statMap = append(statMap, stat)
		}
	}
	gMapVal := g.MapVal{}
	gMapVal.Name = tel
	gMapVal.Value = aggregateMappingDelay(statMap)
	MapLock.Lock()
	MapStatus[prov] = append(MapStatus[prov], gMapVal)
	MapLock.Unlock()
	logrus.Info("Finish MappingTask " + tel + " " + prov + "..")
}

func aggregateMappingDelay(stats []g.PingSt) float64 {
	if len(stats) == 0 {
		return 2000
	}

	// Ignore up to one quarter of failed probes before applying the 2000ms
	// failure penalty. Integer arithmetic keeps the intended ceil(n/4).
	failureAllowance := (len(stats) + 3) / 4
	failuresIgnored := 0
	totalDelay := 0.0
	effectiveCount := 0
	for _, stat := range stats {
		if (stat.LossPk == 100 || stat.RevcPk == 0) && failuresIgnored < failureAllowance {
			failuresIgnored++
			continue
		}
		totalDelay += stat.AvgDelay
		effectiveCount++
	}

	if effectiveCount == 0 {
		return 2000
	}
	value, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", totalDelay/float64(effectiveCount)), 64)
	return value
}

func MapPingStorage() {
	logrus.Info("Start MapPingStorage...")
	logrus.Debug(MapStatus)
	jdata, err := json.Marshal(MapStatus)
	if err != nil {
		logrus.Error("[func:MapPingStorage] Json Error ", err)
	}
	sql := "REPLACE INTO [mappinglog] (logtime, mapjson) values(?, ?)"
	g.DLock.Lock()
	_, err = g.Db.Exec(sql, time.Now().Format("2006-01-02 15:04"), string(jdata))
	logrus.Debug(sql)
	if err != nil {
		logrus.Error("[func:MapPingStorage] Sql Error ", err)
	}
	g.DLock.Unlock()
	logrus.Debug("[func:MapPingStorage] ", sql)
	logrus.Info("Finish MapPingStorage...")
}
