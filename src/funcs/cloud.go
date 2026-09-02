package funcs

import (
	"context"
	"smartping/src/g"

	"github.com/sirupsen/logrus"
)

func StartCloudMonitor() {
	StartCloudMonitorContext(context.Background())
}

func StartCloudMonitorContext(ctx context.Context) {
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
