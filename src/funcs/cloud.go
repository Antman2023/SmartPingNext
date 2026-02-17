package funcs

import (
	"smartping/src/g"

	"github.com/sirupsen/logrus"
)

func StartCloudMonitor() {
	logrus.Info("[func:StartCloudMonitor] ", "starting run StartCloudMonitor ")
	_, err := g.SaveCloudConfig(g.Cfg.Mode["Endpoint"])
	if err != nil {
		logrus.Error("[func:StartCloudMonitor] Cloud Monitor Error", err)
		return
	}
	saveerr := g.SaveConfig()
	if saveerr != nil {
		logrus.Error("[func:StartCloudMonitor] Save Cloud Config Error", saveerr)
		return
	}
	logrus.Info("[func:StartCloudMonitor] ", "StartCloudMonitor finish ")

}
