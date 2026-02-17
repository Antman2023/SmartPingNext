package funcs

import (
	"smartping/src/g"

	"github.com/sirupsen/logrus"
)

func StartCloudMonitor() {
	logrus.Info("[func:StartCloudMonitor] ", "starting run StartCloudMonitor ")
	if _, err := g.SaveCloudConfig(g.Cfg.Mode["Endpoint"]); err != nil {
		logrus.Error("[func:StartCloudMonitor] Cloud Monitor Error", err)
		return
	}
	if err := g.SaveConfig(); err != nil {
		logrus.Error("[func:StartCloudMonitor] Save Cloud Config Error", err)
		return
	}
	logrus.Info("[func:StartCloudMonitor] ", "StartCloudMonitor finish ")
}
