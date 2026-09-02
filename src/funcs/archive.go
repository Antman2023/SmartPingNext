package funcs

import (
	"smartping/src/g"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

const archiveDeleteBatchSize = 1000

var archiveRunning int32

// clear timeout alert table
func ClearArchive() {
	if !atomic.CompareAndSwapInt32(&archiveRunning, 0, 1) {
		logrus.Warn("[func:ClearArchive] Previous archive cleanup still running, skip")
		return
	}
	defer atomic.StoreInt32(&archiveRunning, 0)

	logrus.Info("[func:ClearArchive] ", "starting run ClearArchive ")
	archiveDays := g.ConfigSnapshot().Base["Archive"]
	if archiveDays <= 0 {
		archiveDays = 30
	}
	cutoffDate := time.Now().AddDate(0, 0, -archiveDays).Format("2006-01-02")
	err := clearArchiveBefore(cutoffDate)
	if err != nil {
		logrus.Error("[func:ClearArchive] ", err)
		return
	}
	logrus.Info("[func:ClearArchive] ", "ClearArchive Finish ")
}

func clearArchiveBefore(cutoffDate string) error {
	for _, table := range []string{"alertlog", "mappinglog", "pinglog"} {
		if err := clearArchiveTableBefore(table, cutoffDate); err != nil {
			return err
		}
	}
	return nil
}

func clearArchiveTableBefore(table, cutoffDate string) error {
	query := "delete from " + table + " where rowid in (select rowid from " + table + " where logtime < ? order by logtime limit ?)"
	for {
		g.DLock.Lock()
		result, err := g.Db.Exec(query, cutoffDate, archiveDeleteBatchSize)
		g.DLock.Unlock()
		if err != nil {
			return err
		}
		deleted, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if deleted < archiveDeleteBatchSize {
			return nil
		}
	}
}
