package funcs

import (
	"smartping/src/g"

	"github.com/cihub/seelog"
)

func StartCloudMonitor() {
	seelog.Info("[func:StartCloudMonitor] ", "starting run StartCloudMonitor ")
	_, err := g.SaveCloudConfig(g.Cfg.Mode["Endpoint"])
	if err != nil {
		seelog.Error("[func:StartCloudMonitor] Cloud Monitor Error", err)
		return
	}
	saveerr := g.SaveConfig()
	if saveerr != nil {
		seelog.Error("[func:StartCloudMonitor] Save Cloud Config Error", saveerr)
		return
	}
	seelog.Info("[func:StartCloudMonitor] ", "StartCloudMonitor finish ")

}
