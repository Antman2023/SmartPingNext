package funcs

import (
	"smartping/src/g"
	"strconv"

	"github.com/sirupsen/logrus"
)

// clear timeout alert table
func ClearArchive() {
	logrus.Info("[func:ClearArchive] ", "starting run ClearArchive ")
	g.DLock.Lock()
	g.Db.Exec("delete from alertlog where logtime < date('now','start of day','-" + strconv.Itoa(g.Cfg.Base["Archive"]) + " day')")
	g.Db.Exec("delete from mappinglog where logtime < date('now','start of day','-" + strconv.Itoa(g.Cfg.Base["Archive"]) + " day')")
	g.Db.Exec("delete from pinglog where logtime < date('now','start of day','-" + strconv.Itoa(g.Cfg.Base["Archive"]) + " day')")
	g.DLock.Unlock()
	logrus.Info("[func:ClearArchive] ", "ClearArchive Finish ")
}
