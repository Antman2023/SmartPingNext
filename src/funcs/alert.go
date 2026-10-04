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

// Short windows keep the cheaper aggregate query. For long windows, SQLite
// can stop as soon as the configured number of bad samples has been found.
const minEarlyExitAlertSamples = 600

// Apply the bad-sample predicate after taking the latest samples, so neither
// the grace minute nor older failures can extend the configured sample cap.
// The first existence check preserves unknown as distinct from healthy.
const alertStatusEarlyExitQuery = `SELECT CASE
	WHEN NOT EXISTS (SELECT 1 FROM pinglog WHERE target = ?3 AND logtime >= ?4 AND logtime <= ?5) THEN -1
	WHEN EXISTS (
		SELECT 1 FROM (
			SELECT avgdelay, losspk FROM pinglog
			WHERE target = ?3 AND logtime >= ?4 AND logtime <= ?5
			ORDER BY logtime DESC LIMIT ?6
		)
		WHERE cast(avgdelay as double) > ?1 OR cast(losspk as double) >= ?2
		LIMIT 1 OFFSET ?7
	) THEN 0
	ELSE 1
END`

type alertTraceJob struct {
	g.AlertLog
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
	localAddr, selfConfig := g.LocalNetworkSnapshot()
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
			episode := g.RecordAlertCheckEpisode(localAddr, v, sFlag)

			if episode != nil {
				logrus.Debug("[func:StartAlert] ", v["Addr"]+" Alert!")
				pendingAlerts = append(pendingAlerts, alertTraceJob{episode: episode, AlertLog: g.AlertLog{
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
		alert.episode.Retry()
	}
	if ctx.Err() != nil {
		logrus.Info("[func:StartAlert] canceled")
		return
	}
	logrus.Info("[func:StartAlert] ", "AlertCheck finish ")
}

func runAlertTraceJobsContext(ctx context.Context, alerts []alertTraceJob, concurrency int, process func(context.Context, alertTraceJob)) []alertTraceJob {
	if concurrency < 1 {
		concurrency = 1
	}
	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var unprocessed []alertTraceJob
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
		go func(item alertTraceJob) {
			defer wg.Done()
			defer func() { <-semaphore }()
			process(ctx, item)
		}(alert)
	}
	wg.Wait()
	return unprocessed
}

func traceAndStoreAlertContext(ctx context.Context, job alertTraceJob) {
	alert := job.AlertLog
	hops, err := nettools.RunMtrContext(ctx, alert.Targetip, time.Second, 64, 6)
	if err != nil {
		if ctx.Err() != nil {
			job.episode.Retry()
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
		job.episode.Retry()
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
	if sampleCount >= minEarlyExitAlertSamples {
		var state int
		err := g.Db.QueryRowContext(ctx, alertStatusEarlyExitQuery,
			v["Thdavgdelay"], v["Thdloss"], v["Addr"],
			windowStart.Format("2006-01-02 15:04"), windowEnd.Format("2006-01-02 15:04"),
			sampleCount, Thdoccnum-1,
		).Scan(&state)
		logrus.Debug("[func:StartAlert] ", alertStatusEarlyExitQuery)
		if err != nil {
			return false, fmt.Errorf("query alert status for %s: %w", v["Addr"], err)
		}
		if state < 0 {
			return false, fmt.Errorf("target %s: %w", v["Addr"], ErrNoAlertSamples)
		}
		return state == 1, nil
	}
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
	if err := g.DLock.LockContext(ctx); err != nil {
		return err
	}
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
