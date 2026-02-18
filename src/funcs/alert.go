package funcs

import (
	"encoding/json"
	"smartping/src/g"
	"smartping/src/nettools"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

func StartAlert() {
	logrus.Info("[func:StartAlert] ", "starting run AlertCheck ")
	for _, v := range g.SelfCfg.Topology {
		if v["Addr"] != g.SelfCfg.Addr {
			sFlag := CheckAlertStatus(v)
			g.AlertStatusLock.Lock()
			if sFlag {
				g.AlertStatus[v["Addr"]] = true
			}
			_, haskey := g.AlertStatus[v["Addr"]]
			shouldAlert := (!haskey && !sFlag) || (!sFlag && g.AlertStatus[v["Addr"]])
			if shouldAlert {
				g.AlertStatus[v["Addr"]] = false
			}
			g.AlertStatusLock.Unlock()

			if shouldAlert {
				logrus.Debug("[func:StartAlert] ", v["Addr"]+" Alert!")
				l := g.AlertLog{}
				l.Fromname = g.SelfCfg.Name
				l.Fromip = g.SelfCfg.Addr
				l.Logtime = time.Unix(time.Now().Unix(), 0).Format("2006-01-02 15:04")
				l.Targetname = v["Name"]
				l.Targetip = v["Addr"]
				mtrString := ""
				hops, err := nettools.RunMtr(v["Addr"], time.Second, 64, 6)
				if nil != err {
					logrus.Error("[func:StartAlert] Traceroute error ", err)
					mtrString = err.Error()
				} else {
					jHops, err := json.Marshal(hops)
					if err != nil {
						mtrString = err.Error()
					} else {
						mtrString = string(jHops)
					}
				}
				l.Tracert = mtrString
				go AlertStorage(l)
			}

		}
	}
	logrus.Info("[func:StartAlert] ", "AlertCheck finish ")
}

func CheckAlertStatus(v map[string]string) bool {
	Thdchecksec, _ := strconv.Atoi(v["Thdchecksec"])
	timeStartStr := time.Unix((time.Now().Unix() - int64(Thdchecksec)), 0).Format("2006-01-02 15:04")
	querysql := "SELECT count(1) cnt FROM `pinglog` where logtime > ? and target = ? and (cast(avgdelay as double) > ? or cast(losspk as double) > ?)"
	var cnt int
	err := g.Db.QueryRow(querysql, timeStartStr, v["Addr"], v["Thdavgdelay"], v["Thdloss"]).Scan(&cnt)
	logrus.Debug("[func:StartAlert] ", querysql)
	if err != nil {
		logrus.Error("[func:StartAlert] Query Error ", err)
		return false
	}
	Thdoccnum, _ := strconv.Atoi(v["Thdoccnum"])
	return cnt <= Thdoccnum
}

func AlertStorage(t g.AlertLog) {
	logrus.Info("[func:AlertStorage] ", "(", t.Logtime, ")Starting AlertStorage ", t.Targetname)
	sql := "INSERT INTO [alertlog] (logtime, targetip, targetname, tracert) values(?, ?, ?, ?)"
	g.DLock.Lock()
	_, err := g.Db.Exec(sql, t.Logtime, t.Targetip, t.Targetname, t.Tracert)
	if err != nil {
		logrus.Error("[func:AlertStorage] Sql Error ", err)
	}
	g.DLock.Unlock()
	logrus.Info("[func:AlertStorage] ", "(", t.Logtime, ") AlertStorage on ", t.Targetname, " finish!")
}
