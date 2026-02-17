package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"smartping/src/funcs"
	"smartping/src/g"
	"smartping/src/nettools"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

func configApiRoutes() {

	//配置文件API
	http.HandleFunc("/api/config.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthUserIp(r.RemoteAddr) && !AuthAgentIp(r.RemoteAddr, true) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, http.StatusUnauthorized)
			return
		}
		r.ParseForm()
		nconf := g.Config{}
		cfgJson, _ := json.Marshal(g.Cfg)
		json.Unmarshal(cfgJson, &nconf)
		nconf.Password = ""
		onconf, _ := json.Marshal(nconf)
		var out bytes.Buffer
		json.Indent(&out, onconf, "", "\t")
		o := out.String()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, o)
	})

	//Ping数据API
	http.HandleFunc("/api/ping.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthUserIp(r.RemoteAddr) && !AuthAgentIp(r.RemoteAddr, true) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, http.StatusUnauthorized)
			return
		}
		r.ParseForm()
		if len(r.Form["ip"]) == 0 {
			o := "Missing Param !"
			http.Error(w, o, http.StatusNotAcceptable)
			return
		}
		var tableip string
		var timeStart int64
		var timeEnd int64
		var timeStartStr string
		var timeEndStr string
		tableip = r.Form["ip"][0]
		if len(r.Form["starttime"]) > 0 && len(r.Form["endtime"]) > 0 {
			timeStartStr = r.Form["starttime"][0]
			if timeStartStr != "" {
				tms, _ := time.Parse("2006-01-02 15:04", timeStartStr)
				timeStart = tms.Unix() - 8*60*60
			} else {
				timeStart = time.Now().Unix() - 6*60*60
				timeStartStr = time.Unix(timeStart, 0).Format("2006-01-02 15:04")
			}
			timeEndStr = r.Form["endtime"][0]
			if timeEndStr != "" {
				tmn, _ := time.Parse("2006-01-02 15:04", timeEndStr)
				timeEnd = tmn.Unix() - 8*60*60
			} else {
				timeEnd = time.Now().Unix()
				timeEndStr = time.Unix(timeEnd, 0).Format("2006-01-02 15:04")
			}
		} else {
			timeStart = time.Now().Unix() - 6*60*60
			timeStartStr = time.Unix(timeStart, 0).Format("2006-01-02 15:04")
			timeEnd = time.Now().Unix()
			timeEndStr = time.Unix(timeEnd, 0).Format("2006-01-02 15:04")
		}
		cnt := int((timeEnd - timeStart) / 60)
		var lastcheck []string
		var maxdelay []string
		var mindelay []string
		var avgdelay []string
		var losspk []string
		timwwnum := map[string]int{}
		for i := range cnt + 1 {
			ntime := time.Unix(timeStart, 0).Format("2006-01-02 15:04")
			timwwnum[ntime] = i
			lastcheck = append(lastcheck, ntime)
			maxdelay = append(maxdelay, "0")
			mindelay = append(mindelay, "0")
			avgdelay = append(avgdelay, "0")
			losspk = append(losspk, "0")
			timeStart = timeStart + 60
		}
		querySql := "SELECT logtime,maxdelay,mindelay,avgdelay,losspk FROM `pinglog` where target=? and logtime between ? and ?"
		rows, err := g.Db.Query(querySql, tableip, timeStartStr, timeEndStr)
		logrus.Debug("[func:/api/ping.json] Query ", querySql)
		if err != nil {
			logrus.Error("[func:/api/ping.json] Query ", err)
		} else {
			start := time.Now()
			// Use map for O(1) lookup instead of linear search
			timeIndexMap := make(map[string]int, len(lastcheck))
			for i, t := range lastcheck {
				timeIndexMap[t] = i
			}

			for rows.Next() {
				l := new(g.PingLog)
				err := rows.Scan(&l.Logtime, &l.Maxdelay, &l.Mindelay, &l.Avgdelay, &l.Losspk)
				if err != nil {
					logrus.Error("[/api/ping.json] Rows", err)
					continue
				}

				if idx, exists := timeIndexMap[l.Logtime]; exists {
					maxdelay[idx] = l.Maxdelay
					mindelay[idx] = l.Mindelay
					avgdelay[idx] = l.Avgdelay
					losspk[idx] = l.Losspk
				}
			}
			elapsed := time.Since(start)
			logrus.Info("[func:/api/ping.json] Query ", elapsed)
			rows.Close()
		}
		preout := map[string][]string{
			"lastcheck": lastcheck,
			"maxdelay":  maxdelay,
			"mindelay":  mindelay,
			"avgdelay":  avgdelay,
			"losspk":    losspk,
		}
		w.Header().Set("Content-Type", "application/json")
		RenderJson(w, preout)
	})

	//Ping拓扑API
	http.HandleFunc("/api/topology.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthUserIp(r.RemoteAddr) && !AuthAgentIp(r.RemoteAddr, true) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, http.StatusUnauthorized)
			return
		}
		preout := make(map[string]string)
		for _, v := range g.SelfCfg.Topology {
			if funcs.CheckAlertStatus(v) {
				preout[v["Addr"]] = "true"
			} else {
				preout[v["Addr"]] = "false"
			}
		}
		w.Header().Set("Content-Type", "application/json")
		RenderJson(w, preout)
	})

	//报警API
	http.HandleFunc("/api/alert.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthUserIp(r.RemoteAddr) && !AuthAgentIp(r.RemoteAddr, true) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, http.StatusUnauthorized)
			return
		}
		type DateList struct {
			Ldate string
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		r.ParseForm()
		dtb := time.Unix(time.Now().Unix(), 0).Format("2006-01-02")
		if len(r.Form["date"]) > 0 {
			dtb = strings.Replace(r.Form["date"][0], "alertlog-", "", -1)
		}
		listpreout := []string{}
		datapreout := []g.AlertLog{}
		querySql := "select date(logtime) as ldate from alertlog group by date(logtime) order by logtime desc"
		rows, err := g.Db.Query(querySql)
		logrus.Debug("[func:/api/alert.json] Query ", querySql)
		if err != nil {
			logrus.Error("[func:/api/alert.json] Query ", err)
		} else {
			for rows.Next() {
				l := new(DateList)
				err := rows.Scan(&l.Ldate)
				if err != nil {
					logrus.Error("[/api/alert.json] Rows", err)
					continue
				}
				listpreout = append(listpreout, l.Ldate)
			}
			rows.Close()
		}
		querySql = "select logtime,targetname,targetip,tracert from alertlog where logtime between ? and ?"
		rows, err = g.Db.Query(querySql, dtb+" 00:00:00", dtb+" 23:59:59")
		logrus.Debug("[func:/api/alert.json] Query ", querySql)
		if err != nil {
			logrus.Error("[func:/api/alert.json] Query ", err)
		} else {
			for rows.Next() {
				l := new(g.AlertLog)
				err := rows.Scan(&l.Logtime, &l.Targetname, &l.Targetip, &l.Tracert)
				l.Fromname = g.Cfg.Name
				l.Fromip = g.Cfg.Addr
				if err != nil {
					logrus.Error("[/api/alert.json] Rows", err)
					continue
				}
				datapreout = append(datapreout, *l)
			}
			rows.Close()
		}
		lout, _ := json.Marshal(listpreout)
		dout, _ := json.Marshal(datapreout)
		fmt.Fprintln(w, "["+string(lout)+","+string(dout)+"]")
	})

	//全国延迟API
	http.HandleFunc("/api/mapping.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthUserIp(r.RemoteAddr) && !AuthAgentIp(r.RemoteAddr, true) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, http.StatusUnauthorized)
			return
		}
		m, _ := time.ParseDuration("-1m")
		dataKey := time.Now().Add(m).Format("2006-01-02 15:04")
		r.ParseForm()
		if len(r.Form["d"]) > 0 {
			dataKey = r.Form["d"][0]
		}
		type Mapjson struct {
			Mapjson string
		}
		chinaMp := g.ChinaMp{}
		chinaMp.Text = g.Cfg.Name
		chinaMp.Subtext = dataKey
		chinaMp.Avgdelay = map[string][]g.MapVal{}
		chinaMp.Avgdelay["ctcc"] = []g.MapVal{}
		chinaMp.Avgdelay["cucc"] = []g.MapVal{}
		chinaMp.Avgdelay["cmcc"] = []g.MapVal{}
		g.DLock.Lock()
		querySql := "select mapjson from mappinglog where logtime = ?"
		rows, err := g.Db.Query(querySql, dataKey)
		g.DLock.Unlock()
		logrus.Debug("[func:/api/mapping.json] Query ", querySql)
		if err != nil {
			logrus.Error("[func:/api/mapping.json] Query ", err)
		} else {
			for rows.Next() {
				l := new(Mapjson)
				err := rows.Scan(&l.Mapjson)
				if err != nil {
					logrus.Error("[/api/mapping.json] Rows", err)
					continue
				}
				json.Unmarshal([]byte(l.Mapjson), &chinaMp.Avgdelay)
			}
			rows.Close()
		}
		w.Header().Set("Content-Type", "application/json")
		RenderJson(w, chinaMp)
	})

	//检测工具API
	http.HandleFunc("/api/tools.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthUserIp(r.RemoteAddr) && !AuthAgentIp(r.RemoteAddr, true) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, http.StatusUnauthorized)
			return
		}
		preout := g.ToolsRes{}
		preout.Status = "false"
		r.ParseForm()
		if len(r.Form["t"]) == 0 {
			preout.Error = "target empty!"
			RenderJson(w, preout)
			return
		}
		nowtime := int(time.Now().Unix())
		g.ToolLimitLock.Lock()
		if _, ok := g.ToolLimit[r.RemoteAddr]; ok {
			if (nowtime - g.ToolLimit[r.RemoteAddr]) <= g.Cfg.Toollimit {
				g.ToolLimitLock.Unlock()
				preout.Error = "Time Limit Exceeded!"
				RenderJson(w, preout)
				return
			}
		}
		g.ToolLimit[r.RemoteAddr] = nowtime
		g.ToolLimitLock.Unlock()
		target := strings.Replace(strings.Replace(r.Form["t"][0], "https://", "", -1), "http://", "", -1)
		preout.Ping = g.PingSt{}
		preout.Ping.MinDelay = -1
		lossPK := 0
		ipaddr, err := net.ResolveIPAddr("ip", target)
		if err != nil {
			preout.Error = "Unable to resolve destination host"
			RenderJson(w, preout)
			return
		}
		preout.Ip = ipaddr.String()
		var channel chan float64 = make(chan float64, 5)
		var wg sync.WaitGroup
		for i := range 5 {
			wg.Add(1)
			go func() {
				delay, err := nettools.RunPing(ipaddr, 3*time.Second, 64, i)
				if err != nil {
					channel <- -1.00
				} else {
					channel <- delay
				}
				wg.Done()
			}()
			time.Sleep(time.Duration(100 * time.Millisecond))
		}
		wg.Wait()
		for range 5 {
			delay := <-channel
			if delay != -1.00 {
				preout.Ping.AvgDelay = preout.Ping.AvgDelay + delay
				if preout.Ping.MaxDelay < delay {
					preout.Ping.MaxDelay = delay
				}
				if preout.Ping.MinDelay == -1 || preout.Ping.MinDelay > delay {
					preout.Ping.MinDelay = delay
				}
				preout.Ping.RevcPk = preout.Ping.RevcPk + 1
			} else {
				lossPK = lossPK + 1
			}
			preout.Ping.SendPk = preout.Ping.SendPk + 1
			preout.Ping.LossPk = int((float64(lossPK) / float64(preout.Ping.SendPk)) * 100)
		}
		if preout.Ping.RevcPk > 0 {
			preout.Ping.AvgDelay = preout.Ping.AvgDelay / float64(preout.Ping.RevcPk)
		} else {
			preout.Ping.AvgDelay = 3000
			preout.Ping.MinDelay = 3000
			preout.Ping.MaxDelay = 3000
		}
		// Format delay values to 2 decimal places
		preout.Ping.AvgDelay = float64(int(preout.Ping.AvgDelay*100+0.5)) / 100
		preout.Ping.MinDelay = float64(int(preout.Ping.MinDelay*100+0.5)) / 100
		preout.Ping.MaxDelay = float64(int(preout.Ping.MaxDelay*100+0.5)) / 100
		preout.Status = "true"
		w.Header().Set("Content-Type", "application/json")
		RenderJson(w, preout)
	})

	//验证密码
	http.HandleFunc("/api/verify-password.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthUserIp(r.RemoteAddr) && !AuthAgentIp(r.RemoteAddr, true) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, http.StatusUnauthorized)
			return
		}
		preout := make(map[string]string)
		r.ParseForm()
		preout["status"] = "false"
		if len(r.Form["password"]) == 0 || r.Form["password"][0] != g.Cfg.Password {
			preout["info"] = "密码错误!"
			RenderJson(w, preout)
			return
		}
		preout["status"] = "true"
		RenderJson(w, preout)
	})

	//保存配置文件
	http.HandleFunc("/api/saveconfig.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthUserIp(r.RemoteAddr) && !AuthAgentIp(r.RemoteAddr, true) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, http.StatusUnauthorized)
			return
		}
		preout := make(map[string]string)
		r.ParseForm()
		preout["status"] = "false"
		if len(r.Form["password"]) == 0 || r.Form["password"][0] != g.Cfg.Password {
			preout["info"] = "密码错误!"
			RenderJson(w, preout)
			return
		}
		if len(r.Form["config"]) == 0 {
			preout["info"] = "参数错误!"
			RenderJson(w, preout)
			return
		}
		nconfig := g.Config{}
		err := json.Unmarshal([]byte(r.Form["config"][0]), &nconfig)
		if err != nil {
			preout["info"] = "配置文件解析错误!" + err.Error()
			RenderJson(w, preout)
			return
		}
		if nconfig.Name == "" {
			preout["info"] = "本机节点名称为空!"
			RenderJson(w, preout)
			return
		}
		if !ValidIP4(nconfig.Addr) {
			preout["info"] = "非法本机节点IP!"
			RenderJson(w, preout)
			return
		}
		//Base
		if _, ok := nconfig.Base["Timeout"]; !ok || nconfig.Base["Timeout"] <= 0 {
			preout["info"] = "非法超时时间!(>0)"
			RenderJson(w, preout)
			return
		}
		if _, ok := nconfig.Base["Archive"]; !ok || nconfig.Base["Archive"] <= 0 {
			preout["info"] = "非法存档天数!(>0)"
			RenderJson(w, preout)
			return
		}
		if _, ok := nconfig.Base["Refresh"]; !ok || nconfig.Base["Refresh"] <= 0 {
			preout["info"] = "非法刷新频率!(>0)"
			RenderJson(w, preout)
			return
		}
		//Topology
		if _, ok := nconfig.Topology["Tline"]; !ok || nconfig.Topology["Tline"] <= "0" {
			preout["info"] = "非法拓扑连线粗细(>0)"
			RenderJson(w, preout)
			return
		}
		if _, ok := nconfig.Topology["Tsymbolsize"]; !ok || nconfig.Topology["Tsymbolsize"] <= "0" {
			preout["info"] = "非法拓扑形状大小!(>0)"
			RenderJson(w, preout)
			return
		}
		if nconfig.Toollimit < 0 {
			preout["info"] = "非法检测工具限定频率!(>=0)"
			RenderJson(w, preout)
			return
		}
		//Network
		for k, network := range nconfig.Network {
			if !ValidIP4(network.Addr) || !ValidIP4(k) {
				preout["info"] = "Ping节点测试网络信息错误!(非法节点IP地址 " + k + ")"
				RenderJson(w, preout)
				return
			}
			if network.Name == "" {
				preout["info"] = "Ping节点测试网络信息错误!( " + k + " 节点名称为空)"
				RenderJson(w, preout)
				return
			}
			for _, topology := range network.Topology {
				if _, ok := topology["Thdchecksec"]; !ok {
					preout["info"] = "Ping节点测试网络信息错误!( " + k + "->" + topology["Addr"] + " 非法拓扑报警规则，秒) "
					RenderJson(w, preout)
					return
				} else {
					Thdchecksec, err := strconv.Atoi(topology["Thdchecksec"])
					if err != nil || Thdchecksec <= 0 {
						preout["info"] = "Ping节点测试网络信息错误!( " + k + "->" + topology["Addr"] + " 非法拓扑报警规则，>0 秒  ) "
						RenderJson(w, preout)
						return
					}
				}
				if _, ok := topology["Thdloss"]; !ok {
					preout["info"] = "Ping节点测试网络信息错误!( " + k + "->" + topology["Addr"] + " 非法拓扑报警规则，%) "
					RenderJson(w, preout)
					return
				} else {
					Thdloss, err := strconv.Atoi(topology["Thdloss"])
					if err != nil || (Thdloss < 0 || Thdloss > 100) {
						preout["info"] = "Ping节点测试网络信息错误!( " + k + "->" + topology["Addr"] + " 非法拓扑报警规则，0 <= % <=100  ) "
						RenderJson(w, preout)
						return
					}
				}
				if _, ok := topology["Thdavgdelay"]; !ok {
					preout["info"] = "Ping节点测试网络信息错误!( " + k + "->" + topology["Addr"] + " 非法拓扑报警规则，ms) "
					RenderJson(w, preout)
					return
				} else {
					Thdavgdelay, err := strconv.Atoi(topology["Thdavgdelay"])
					if err != nil || Thdavgdelay <= 0 {
						preout["info"] = "Ping节点测试网络信息错误!( " + k + "->" + topology["Addr"] + " 非法拓扑报警规则，> 0 ms  ) "
						RenderJson(w, preout)
						return
					}
				}
				if _, ok := topology["Thdoccnum"]; !ok {
					preout["info"] = "Ping节点测试网络信息错误!( " + k + "->" + topology["Addr"] + " 非法拓扑报警规则，次) "
					RenderJson(w, preout)
					return
				} else {
					Thdoccnum, err := strconv.Atoi(topology["Thdoccnum"])
					if err != nil || Thdoccnum <= 0 {
						preout["info"] = "Ping节点测试网络信息错误!( " + k + "->" + topology["Addr"] + " 非法拓扑报警规则，> 0 次  ) "
						RenderJson(w, preout)
						return
					}
				}
			}
		}
		//ChinaMap
		for _, provVal := range nconfig.Chinamap {
			for _, telcomVal := range provVal {
				for _, ip := range telcomVal {
					if ip != "" && !ValidIP4(ip) {
						preout["info"] = "Mapping Ip illegal!"
						RenderJson(w, preout)
						return
					}
				}
			}
		}
		nconfig.Ver = g.Cfg.Ver
		nconfig.Port = g.Cfg.Port
		nconfig.Password = g.Cfg.Password
		g.Cfg = nconfig
		g.SelfCfg = g.Cfg.Network[g.Cfg.Addr]
		saveerr := g.SaveConfig()
		if saveerr != nil {
			preout["info"] = saveerr.Error()
			RenderJson(w, preout)
			return
		}
		preout["status"] = "true"
		RenderJson(w, preout)
	})

	//发送测试邮件
	//代理访问
	http.HandleFunc("/api/proxy.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthUserIp(r.RemoteAddr) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		r.ParseForm()
		if len(r.Form["g"]) == 0 {
			o := "Url Param Error!"
			http.Error(w, o, http.StatusNotAcceptable)
			return
		}
		to := strconv.Itoa(g.Cfg.Base["Timeout"])
		if len(r.Form["t"]) > 0 {
			to = r.Form["t"][0]
		}
		url := strings.Replace(strings.Replace(r.Form["g"][0], "%26", "&", -1), " ", "%20", -1)
		defaultto, err := strconv.Atoi(to)
		if err != nil {
			o := "Timeout Param Error!"
			http.Error(w, o, http.StatusNotAcceptable)
			return
		}
		timeout := time.Duration(time.Duration(defaultto) * time.Second)
		client := http.Client{
			Timeout: timeout,
		}
		resp, err := client.Get(url)
		if err != nil {
			o := "Request Remote Data Error:" + err.Error()
			http.Error(w, o, http.StatusServiceUnavailable)
			return
		}
		defer resp.Body.Close()
		resCode := resp.StatusCode
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			o := "Read Remote Data Error:" + err.Error()
			http.Error(w, o, http.StatusServiceUnavailable)
			return
		}
		if resCode != 200 {
			o := "Get Remote Data Status Error"
			http.Error(w, o, resCode)
		}
		var out bytes.Buffer
		json.Indent(&out, body, "", "\t")
		o := out.String()
		fmt.Fprintln(w, o)
	})

}
