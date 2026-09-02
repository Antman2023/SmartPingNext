package funcs

import (
	"context"
	"smartping/src/g"
	"sync/atomic"

	"github.com/sirupsen/logrus"
)

var cloudMonitorRunning int32

func StartCloudMonitor() {
	StartCloudMonitorContext(context.Background())
}

func StartCloudMonitorContext(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	if !atomic.CompareAndSwapInt32(&cloudMonitorRunning, 0, 1) {
		logrus.Warn("[func:StartCloudMonitor] Previous cloud monitor still running, skip")
		return
	}
	defer atomic.StoreInt32(&cloudMonitorRunning, 0)

	logrus.Info("[func:StartCloudMonitor] ", "starting run StartCloudMonitor ")
	config := g.ConfigSnapshot()
	endpoint := config.Mode["Endpoint"]
	if _, err := g.SaveCloudConfigContext(ctx, endpoint); err != nil {
		if ctx.Err() != nil {
			logrus.Info("[func:StartCloudMonitor] canceled")
			return
		}
		g.SetCloudStatus(endpoint, "false")
		logrus.Error("[func:StartCloudMonitor] Cloud Monitor Error", err)
		return
	}
	logrus.Info("[func:StartCloudMonitor] ", "StartCloudMonitor finish ")
}
