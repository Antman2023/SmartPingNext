package funcs

import (
	"smartping/src/g"
	"time"

	"github.com/sirupsen/logrus"
)

// clear timeout alert table
func ClearArchive() {
	logrus.Info("[func:ClearArchive] ", "starting run ClearArchive ")
	archiveDays := g.Cfg.Base["Archive"]
	if archiveDays <= 0 {
		archiveDays = 30
	}
	cutoffDate := time.Now().AddDate(0, 0, -archiveDays).Format("2006-01-02")
	g.DLock.Lock()
	g.Db.Exec("delete from alertlog where logtime < ?", cutoffDate)
	g.Db.Exec("delete from mappinglog where logtime < ?", cutoffDate)
	g.Db.Exec("delete from pinglog where logtime < ?", cutoffDate)
	g.DLock.Unlock()
	logrus.Info("[func:ClearArchive] ", "ClearArchive Finish ")
}
