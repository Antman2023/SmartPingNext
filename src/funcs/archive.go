package funcs

import (
	"smartping/src/g"
	"time"

	"github.com/sirupsen/logrus"
)

// clear timeout alert table
func ClearArchive() {
	logrus.Info("[func:ClearArchive] ", "starting run ClearArchive ")
	archiveDays := g.ConfigSnapshot().Base["Archive"]
	if archiveDays <= 0 {
		archiveDays = 30
	}
	cutoffDate := time.Now().AddDate(0, 0, -archiveDays).Format("2006-01-02")
	g.DLock.Lock()
	err := clearArchiveBefore(cutoffDate)
	g.DLock.Unlock()
	if err != nil {
		logrus.Error("[func:ClearArchive] ", err)
		return
	}
	logrus.Info("[func:ClearArchive] ", "ClearArchive Finish ")
}

func clearArchiveBefore(cutoffDate string) error {
	tx, err := g.Db.Begin()
	if err != nil {
		return err
	}
	for _, table := range []string{"alertlog", "mappinglog", "pinglog"} {
		if _, err := tx.Exec("delete from "+table+" where logtime < ?", cutoffDate); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}
