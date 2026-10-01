package funcs

import (
	"context"
	"encoding/json"
	"errors"
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

// ErrNoAlertSamples means the configured time window has no samples for the target.
var ErrNoAlertSamples = errors.New("no samples in alert window")

const alertTraceConcurrency = 4

type alertTraceJob struct {
	alert   g.AlertLog
	episode *g.AlertEpisode
}

func StartAlert() {
	StartAlertContext(context.Background())
}

func StartAlertContext(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	if !atomic.CompareAndSwapInt32(&alertRunning, 0, 1) {
		logrus.Warn("[func:StartAlert] Previous alert check still running, skip")
		return
	}
	defer atomic.StoreInt32(&alertRunning, 0)

	logrus.Info("[func:StartAlert] ", "starting run AlertCheck ")
	config := g.ConfigSnapshot()
	selfConfig := config.Network[config.Addr]
	pendingAlerts := make([]alertTraceJob, 0)
	for _, v := range selfConfig.Topology {
		if ctx.Err() != nil {
			// Let the canceled job runner return queued alerts for retry.
			break
		}
		if v["Addr"] != selfConfig.Addr {
			sFlag, err := CheckAlertStatusContext(ctx, v)
			if err != nil {
				if errors.Is(err, ErrNoAlertSamples) {
					continue
				}
				if ctx.Err() != nil {
					break
				}
				logrus.Error("[func:StartAlert] Check status error ", err)
				continue
			}
			episode := g.RecordAlertCheckEpisode(config.Addr, v, sFlag)

			if episode != nil {
				logrus.Debug("[func:StartAlert] ", v["Addr"]+" Alert!")
				pendingAlerts = append(pendingAlerts, alertTraceJob{episode: episode, alert: g.AlertLog{
					Fromname:   selfConfig.Name,
					Fromip:     selfConfig.Addr,
					Logtime:    time.Now().Format("2006-01-02 15:04"),
					Targetname: v["Name"],
					Targetip:   v["Addr"],
				}})
			}

		}
	}
	unprocessed := runAlertTraceJobsContext(ctx, pendingAlerts, alertTraceConcurrency, traceAndStoreAlertContext)
	for _, alert := range unprocessed {
		g.RetryAlertEpisode(alert.episode)
	}
	if ctx.Err() != nil {
		logrus.Info("[func:StartAlert] canceled")
		return
	}
	logrus.Info("[func:StartAlert] ", "AlertCheck finish ")
}

func runAlertTraceJobs(alerts []g.AlertLog, concurrency int, process func(g.AlertLog)) {
	_ = runAlertTraceJobsContext(context.Background(), alerts, concurrency, func(_ context.Context, item g.AlertLog) {
		process(item)
	})
}

func runAlertTraceJobsContext[T any](ctx context.Context, alerts []T, concurrency int, process func(context.Context, T)) []T {
	if concurrency < 1 {
		concurrency = 1
	}
	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var unprocessed []T
	for index, alert := range alerts {
		if ctx.Err() != nil {
			unprocessed = append(unprocessed, alerts[index:]...)
			break
		}
		wg.Add(1)
		select {
		case semaphore <- struct{}{}:
		case <-ctx.Done():
			wg.Done()
			unprocessed = append(unprocessed, alerts[index:]...)
			break
		}
		if len(unprocessed) > 0 {
			break
		}
		go func(item T) {
			defer wg.Done()
			defer func() { <-semaphore }()
			process(ctx, item)
		}(alert)
	}
	wg.Wait()
	return unprocessed
}

func traceAndStoreAlertContext(ctx context.Context, job alertTraceJob) {
	alert := job.alert
	hops, err := nettools.RunMtrContext(ctx, alert.Targetip, time.Second, 64, 6)
	if err != nil {
		if ctx.Err() != nil {
			g.RetryAlertEpisode(job.episode)
			return
		}
		logrus.Error("[func:StartAlert] Traceroute error ", err)
		alert.Tracert = err.Error()
	} else if encoded, marshalErr := json.Marshal(hops); marshalErr != nil {
		alert.Tracert = marshalErr.Error()
	} else {
		alert.Tracert = string(encoded)
	}
	if err := AlertStorageContext(ctx, alert); err != nil {
		g.RetryAlertEpisode(job.episode)
		if ctx.Err() == nil {
			logrus.Error("[func:StartAlert] Store alert error ", err)
		}
	}
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
	querysql := `SELECT count(1), coalesce(sum(CASE WHEN cast(avgdelay as double) > ? OR cast(losspk as double) >= ? THEN 1 ELSE 0 END), 0) FROM (
		SELECT avgdelay, losspk FROM pinglog
		WHERE target = ? AND logtime >= ? AND logtime <= ?
		ORDER BY logtime DESC LIMIT ?
	)`
	var samples, cnt int
	err = g.Db.QueryRowContext(
		ctx,
		querysql,
		v["Thdavgdelay"],
		v["Thdloss"],
		v["Addr"],
		windowStart.Format("2006-01-02 15:04"),
		windowEnd.Format("2006-01-02 15:04"),
		sampleCount,
	).Scan(&samples, &cnt)
	logrus.Debug("[func:StartAlert] ", querysql)
	if err != nil {
		return false, fmt.Errorf("query alert status for %s: %w", v["Addr"], err)
	}
	if samples == 0 {
		return false, fmt.Errorf("target %s: %w", v["Addr"], ErrNoAlertSamples)
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
	if err := AlertStorageContext(context.Background(), t); err != nil {
		logrus.Error("[func:AlertStorage] Sql Error ", err)
	}
}

func AlertStorageContext(ctx context.Context, t g.AlertLog) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	logrus.Info("[func:AlertStorage] ", "(", t.Logtime, ")Starting AlertStorage ", t.Targetname)
	sql := "INSERT INTO [alertlog] (logtime, targetip, targetname, tracert) values(?, ?, ?, ?) ON CONFLICT(logtime, targetip) DO UPDATE SET targetname=excluded.targetname, tracert=excluded.tracert"
	g.DLock.Lock()
	defer g.DLock.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := g.Db.ExecContext(ctx, sql, t.Logtime, t.Targetip, t.Targetname, t.Tracert)
	if err != nil {
		return fmt.Errorf("store alert for %s: %w", t.Targetip, err)
	}
	logrus.Info("[func:AlertStorage] ", "(", t.Logtime, ") AlertStorage on ", t.Targetname, " finish!")
	return nil
}
