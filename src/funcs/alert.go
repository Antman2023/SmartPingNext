package funcs

import (
	"context"
	"encoding/json"
	"fmt"
	"smartping/src/g"
	"smartping/src/nettools"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

var alertRunning int32

const alertTraceConcurrency = 4

func StartAlert() {
	if !atomic.CompareAndSwapInt32(&alertRunning, 0, 1) {
		logrus.Warn("[func:StartAlert] Previous alert check still running, skip")
		return
	}
	defer atomic.StoreInt32(&alertRunning, 0)

	logrus.Info("[func:StartAlert] ", "starting run AlertCheck ")
	config := g.ConfigSnapshot()
	selfConfig := config.Network[config.Addr]
	pendingAlerts := make([]g.AlertLog, 0)
	for _, v := range selfConfig.Topology {
		if v["Addr"] != selfConfig.Addr {
			sFlag, err := CheckAlertStatus(v)
			if err != nil {
				logrus.Error("[func:StartAlert] Check status error ", err)
				continue
			}
			g.AlertStatusLock.Lock()
			if sFlag {
				g.AlertStatus[v["Addr"]] = true
			}
			_, haskey := g.AlertStatus[v["Addr"]]
			shouldAlert := (!haskey && !sFlag) || (!sFlag && g.AlertStatus[v["Addr"]])
			if shouldAlert {
				g.AlertStatus[v["Addr"]] = false
			}
			g.AlertStatusLock.Unlock()

			if shouldAlert {
				logrus.Debug("[func:StartAlert] ", v["Addr"]+" Alert!")
				pendingAlerts = append(pendingAlerts, g.AlertLog{
					Fromname:   selfConfig.Name,
					Fromip:     selfConfig.Addr,
					Logtime:    time.Now().Format("2006-01-02 15:04"),
					Targetname: v["Name"],
					Targetip:   v["Addr"],
				})
			}

		}
	}
	runAlertTraceJobs(pendingAlerts, alertTraceConcurrency, traceAndStoreAlert)
	logrus.Info("[func:StartAlert] ", "AlertCheck finish ")
}

func runAlertTraceJobs(alerts []g.AlertLog, concurrency int, process func(g.AlertLog)) {
	if concurrency < 1 {
		concurrency = 1
	}
	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for _, alert := range alerts {
		semaphore <- struct{}{}
		wg.Add(1)
		go func(item g.AlertLog) {
			defer wg.Done()
			defer func() { <-semaphore }()
			process(item)
		}(alert)
	}
	wg.Wait()
}

func traceAndStoreAlert(alert g.AlertLog) {
	hops, err := nettools.RunMtr(alert.Targetip, time.Second, 64, 6)
	if err != nil {
		logrus.Error("[func:StartAlert] Traceroute error ", err)
		alert.Tracert = err.Error()
	} else if encoded, marshalErr := json.Marshal(hops); marshalErr != nil {
		alert.Tracert = marshalErr.Error()
	} else {
		alert.Tracert = string(encoded)
	}
	AlertStorage(alert)
}

func CheckAlertStatus(v map[string]string) (bool, error) {
	return CheckAlertStatusContext(context.Background(), v)
}

func CheckAlertStatusContext(ctx context.Context, v map[string]string) (bool, error) {
	return checkAlertStatusAtContext(ctx, v, time.Now())
}

func checkAlertStatusAt(v map[string]string, now time.Time) (bool, error) {
	return checkAlertStatusAtContext(context.Background(), v, now)
}

func checkAlertStatusAtContext(ctx context.Context, v map[string]string, now time.Time) (bool, error) {
	Thdchecksec, err := strconv.Atoi(v["Thdchecksec"])
	if err != nil || Thdchecksec <= 0 {
		return false, fmt.Errorf("invalid Thdchecksec %q", v["Thdchecksec"])
	}
	Thdoccnum, err := strconv.Atoi(v["Thdoccnum"])
	if err != nil || Thdoccnum <= 0 {
		return false, fmt.Errorf("invalid Thdoccnum %q", v["Thdoccnum"])
	}

	sampleCount := Thdchecksec / 60
	if sampleCount < 1 {
		sampleCount = 1
	}
	windowEnd := now.Truncate(time.Minute)
	// A ping round is timestamped at its start but can finish just after the next
	// minute begins. Include that boundary minute, then cap the query to the
	// configured number of samples so the grace period cannot count stale data.
	windowStart := alertWindowStart(now, Thdchecksec).Add(-time.Minute)
	querysql := `SELECT count(1) FROM (
		SELECT avgdelay, losspk FROM pinglog
		WHERE target = ? AND logtime >= ? AND logtime <= ?
		ORDER BY logtime DESC LIMIT ?
	) WHERE cast(avgdelay as double) > ? OR cast(losspk as double) >= ?`
	var cnt int
	err = g.Db.QueryRowContext(
		ctx,
		querysql,
		v["Addr"],
		windowStart.Format("2006-01-02 15:04"),
		windowEnd.Format("2006-01-02 15:04"),
		sampleCount,
		v["Thdavgdelay"],
		v["Thdloss"],
	).Scan(&cnt)
	logrus.Debug("[func:StartAlert] ", querysql)
	if err != nil {
		return false, fmt.Errorf("query alert status for %s: %w", v["Addr"], err)
	}
	return cnt < Thdoccnum, nil
}

func alertWindowStart(now time.Time, windowSeconds int) time.Time {
	samples := windowSeconds / 60
	if samples < 1 {
		samples = 1
	}
	return now.Truncate(time.Minute).Add(-time.Duration(samples-1) * time.Minute)
}

func AlertStorage(t g.AlertLog) {
	logrus.Info("[func:AlertStorage] ", "(", t.Logtime, ")Starting AlertStorage ", t.Targetname)
	sql := "INSERT INTO [alertlog] (logtime, targetip, targetname, tracert) values(?, ?, ?, ?) ON CONFLICT(logtime, targetip) DO UPDATE SET targetname=excluded.targetname, tracert=excluded.tracert"
	g.DLock.Lock()
	_, err := g.Db.Exec(sql, t.Logtime, t.Targetip, t.Targetname, t.Tracert)
	if err != nil {
		logrus.Error("[func:AlertStorage] Sql Error ", err)
	}
	g.DLock.Unlock()
	logrus.Info("[func:AlertStorage] ", "(", t.Logtime, ") AlertStorage on ", t.Targetname, " finish!")
}
