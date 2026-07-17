package funcs

import (
	"smartping/src/g"

	"github.com/sirupsen/logrus"
)

func StartCloudMonitor() {
	logrus.Info("[func:StartCloudMonitor] ", "starting run StartCloudMonitor ")
	config := g.ConfigSnapshot()
	if _, err := g.SaveCloudConfig(config.Mode["Endpoint"]); err != nil {
		current := g.ConfigSnapshot()
		mode := make(map[string]string, len(current.Mode)+1)
		for key, value := range current.Mode {
			mode[key] = value
		}
		mode["Status"] = "false"
		current.Mode = mode
		g.SetConfig(current)
		logrus.Error("[func:StartCloudMonitor] Cloud Monitor Error", err)
		return
	}
	if err := g.SaveConfig(); err != nil {
		logrus.Error("[func:StartCloudMonitor] Save Cloud Config Error", err)
		return
	}
	logrus.Info("[func:StartCloudMonitor] ", "StartCloudMonitor finish ")
}
