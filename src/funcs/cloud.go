package funcs

import (
	"smartping/src/g"

	"github.com/sirupsen/logrus"
)

func StartCloudMonitor() {
	logrus.Info("[func:StartCloudMonitor] ", "starting run StartCloudMonitor ")
	config := g.ConfigSnapshot()
	endpoint := config.Mode["Endpoint"]
	if _, err := g.SaveCloudConfig(endpoint); err != nil {
		g.SetCloudStatus(endpoint, "false")
		logrus.Error("[func:StartCloudMonitor] Cloud Monitor Error", err)
		return
	}
	logrus.Info("[func:StartCloudMonitor] ", "StartCloudMonitor finish ")
}
