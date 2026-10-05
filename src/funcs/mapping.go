package funcs

import (
	"context"
	"encoding/json"
	"fmt"
	"smartping/src/g"
	"smartping/src/internal/contextlock"
	"smartping/src/nettools"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	MapLock   = new(contextlock.Mutex)
	MapStatus map[string][]g.MapVal
)

const (
	defaultMappingProbeCount  = 3
	defaultMappingConcurrency = 8
)

var mappingRunning int32

func Mapping() {
	MappingContext(context.Background())
}

func MappingContext(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	if !atomic.CompareAndSwapInt32(&mappingRunning, 0, 1) {
		logrus.Warn("[func:Mapping] Previous round still running, skip")
		return
	}
	defer atomic.StoreInt32(&mappingRunning, 0)

	var wg sync.WaitGroup
	config, err := g.ConfigSnapshotContext(ctx)
	if err != nil {
		return
	}
	workerLimit := boundedBaseInt(config, "MappingConcurrency", defaultMappingConcurrency, 1, 64)
	probeCount := boundedBaseInt(config, "MappingProbeCount", defaultMappingProbeCount, 1, 20)
	sem := make(chan struct{}, workerLimit)
	if err := MapLock.LockContext(ctx); err != nil {
		return
	}
	MapStatus = map[string][]g.MapVal{}
	MapLock.Unlock()
	logrus.Debug("[func:Mapping]", config.Chinamap)
loop:
	for province, carrierTargets := range config.Chinamap {
		for carrier, ips := range carrierTargets {
			logrus.Debug("[func:Mapping]", ips)
			if len(ips) > 0 {
				wg.Add(1)
				select {
				case sem <- struct{}{}:
				case <-ctx.Done():
					wg.Done()
					break loop
				}
				go func(carrier, province string, ips []string) {
					defer func() { <-sem }()
					MappingTaskContext(ctx, carrier, province, ips, probeCount, &wg)
				}(carrier, province, ips)
			}
		}
	}
	wg.Wait()
	if ctx.Err() == nil {
		if err := MapPingStorageContext(ctx); err != nil {
			logrus.Error("[func:Mapping] Store mapping result error ", err)
		}
	}
}

// ping main function
func MappingTask(carrier string, province string, ips []string, probeCount int, wg *sync.WaitGroup) {
	MappingTaskContext(context.Background(), carrier, province, ips, probeCount, wg)
}

func MappingTaskContext(ctx context.Context, carrier string, province string, ips []string, probeCount int, wg *sync.WaitGroup) {
	defer wg.Done()
	logrus.Info("Start MappingTask " + carrier + " " + province + "..")
	statMap := []g.PingSt{}
	for _, ip := range ips {
		if ctx.Err() != nil {
			return
		}
		logrus.Debug("[func:StartChinaMapPing]", ip)
		ipaddr, err := nettools.ResolveIPv4Context(ctx, ip)
		if err == nil {
			for i := 0; i < probeCount; i++ {
				if ctx.Err() != nil {
					return
				}
				stat := g.PingSt{}
				stat.MinDelay = -1
				stat.LossPk = 0
				delay, err := nettools.RunPingContext(ctx, ipaddr, 3*time.Second, 64, i)
				if ctx.Err() != nil {
					return
				}
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
				}
				stat.SendPk = stat.SendPk + 1
				stat.UpdateLoss()
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
	if err := storeMappingResultContext(ctx, carrier, province, aggregateMappingDelay(statMap)); err != nil {
		return
	}
	logrus.Info("Finish MappingTask " + carrier + " " + province + "..")
}

func storeMappingResult(carrier, province string, value float64) {
	_ = storeMappingResultContext(context.Background(), carrier, province, value)
}

func storeMappingResultContext(ctx context.Context, carrier, province string, value float64) error {
	if err := MapLock.LockContext(ctx); err != nil {
		return err
	}
	defer MapLock.Unlock()
	gMapVal := g.MapVal{Name: province, Value: value}
	if MapStatus == nil {
		MapStatus = make(map[string][]g.MapVal)
	}
	MapStatus[carrier] = append(MapStatus[carrier], gMapVal)
	return nil
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

func mappingStatusSnapshot() map[string][]g.MapVal {
	snapshot, _ := mappingStatusSnapshotContext(context.Background())
	return snapshot
}

func mappingStatusSnapshotContext(ctx context.Context) (map[string][]g.MapVal, error) {
	snapshot, err := copyMappingStatusContext(ctx)
	if err != nil {
		return nil, err
	}
	if err := sortMappingStatusSnapshotContext(ctx, snapshot); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func copyMappingStatusContext(ctx context.Context) (map[string][]g.MapVal, error) {
	if err := MapLock.LockContext(ctx); err != nil {
		return nil, err
	}
	defer MapLock.Unlock()
	snapshot := make(map[string][]g.MapVal, len(MapStatus))
	cancellable := ctx.Done() != nil
	for carrier, values := range MapStatus {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !cancellable || len(values) == 0 {
			// Keep the legacy nil representation for empty carrier slices.
			snapshot[carrier] = append([]g.MapVal(nil), values...)
			continue
		}
		copied := make([]g.MapVal, len(values))
		for start := 0; start < len(values); start += 256 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			copy(copied[start:min(start+256, len(values))], values[start:min(start+256, len(values))])
		}
		snapshot[carrier] = copied
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func sortMappingStatusSnapshotContext(ctx context.Context, snapshot map[string][]g.MapVal) error {
	for carrier := range snapshot {
		if err := ctx.Err(); err != nil {
			return err
		}
		sort.Slice(snapshot[carrier], func(i, j int) bool {
			return snapshot[carrier][i].Name < snapshot[carrier][j].Name
		})
	}
	// A standard sort cannot be interrupted; observe cancellation before the
	// next carrier or before returning the completed snapshot.
	return ctx.Err()
}

func MapPingStorage() {
	if err := MapPingStorageContext(context.Background()); err != nil {
		logrus.Error("[func:MapPingStorage] ", err)
	}
}

func MapPingStorageContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	logrus.Info("Start MapPingStorage...")
	snapshot, err := mappingStatusSnapshotContext(ctx)
	if err != nil {
		return err
	}
	logrus.Debug(snapshot)
	if err := ctx.Err(); err != nil {
		return err
	}
	jdata, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("encode mapping result: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	sql := "REPLACE INTO [mappinglog] (logtime, mapjson) values(?, ?)"
	if err := g.DLock.LockContext(ctx); err != nil {
		return err
	}
	defer g.DLock.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err = g.Db.ExecContext(ctx, sql, time.Now().Format("2006-01-02 15:04"), string(jdata))
	logrus.Debug(sql)
	if err != nil {
		return fmt.Errorf("store mapping result: %w", err)
	}
	logrus.Debug("[func:MapPingStorage] ", sql)
	logrus.Info("Finish MapPingStorage...")
	return nil
}
