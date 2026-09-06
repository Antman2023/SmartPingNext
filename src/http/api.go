package http

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
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

func configApiRoutes(mux *http.ServeMux) {

	//配置文件API
	mux.HandleFunc("/api/config.json", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		if !AuthUserIp(r.RemoteAddr) && !AuthAgentIp(r.RemoteAddr, true) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, http.StatusUnauthorized)
			return
		}
		nconf := g.ConfigSnapshot()
		nconf.Password = ""
		RenderJson(w, nconf)
	})

	//Ping数据API
	mux.HandleFunc("/api/ping.json", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		if !AuthUserIp(r.RemoteAddr) && !AuthAgentIp(r.RemoteAddr, true) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, http.StatusUnauthorized)
			return
		}
		form := r.URL.Query()
		if len(form["ip"]) == 0 {
			o := "Missing Param !"
			http.Error(w, o, http.StatusNotAcceptable)
			return
		}
		var tableip string
		tableip = form["ip"][0]
		requestTime := time.Now()
		timeStartValue, timeEndValue, err := resolvePingTimeRange(form, requestTime, g.LocalTimezone)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotAcceptable)
			return
		}
		timeStart := timeStartValue.Unix()
		timeEnd := timeEndValue.Unix()
		timeStartStr := timeStartValue.Format("2006-01-02 15:04")
		timeEndStr := timeEndValue.Format("2006-01-02 15:04")
		cnt := int((timeEnd - timeStart) / 60)
		size := cnt + 1
		lastcheck := make([]string, size)
		maxdelay := make([]string, size)
		mindelay := make([]string, size)
		avgdelay := make([]string, size)
		losspk := make([]string, size)
		populated := make([]bool, size)
		cursor := timeStart
		for i := 0; i < size; i++ {
			ntime := time.Unix(cursor, 0).In(timeStartValue.Location()).Format("2006-01-02 15:04")
			lastcheck[i] = ntime
			// Missing samples must remain gaps, not healthy zero-valued measurements.
			maxdelay[i] = "-"
			mindelay[i] = "-"
			avgdelay[i] = "-"
			losspk[i] = "-"
			cursor += 60
		}
		querySql := "SELECT logtime,maxdelay,CASE WHEN cast(mindelay as double) < 0 THEN '0' ELSE mindelay END,avgdelay,losspk FROM `pinglog` where target=? and logtime between ? and ?"
		rows, err := g.Db.QueryContext(r.Context(), querySql, tableip, timeStartStr, timeEndStr)
		logrus.Debug("[func:/api/ping.json] Query ", querySql)
		if err != nil {
			logrus.Error("[func:/api/ping.json] Query ", err)
			http.Error(w, "Query ping data failed", http.StatusInternalServerError)
			return
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
					rows.Close()
					http.Error(w, "Read ping data failed", http.StatusInternalServerError)
					return
				}

				if idx, exists := timeIndexMap[l.Logtime]; exists {
					maxdelay[idx] = l.Maxdelay
					mindelay[idx] = l.Mindelay
					avgdelay[idx] = l.Avgdelay
					losspk[idx] = l.Losspk
					populated[idx] = true
				}
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				logrus.Error("[/api/ping.json] Rows", err)
				http.Error(w, "Read ping data failed", http.StatusInternalServerError)
				return
			}
			elapsed := time.Since(start)
			logrus.Info("[func:/api/ping.json] Query ", elapsed)
			rows.Close()
		}
		completedSize := completedPingTimelineSize(lastcheck, populated, requestTime, g.LocalTimezone)
		lastcheck = lastcheck[:completedSize]
		maxdelay = maxdelay[:completedSize]
		mindelay = mindelay[:completedSize]
		avgdelay = avgdelay[:completedSize]
		losspk = losspk[:completedSize]
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
	mux.HandleFunc("/api/topology.json", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		if !AuthUserIp(r.RemoteAddr) && !AuthAgentIp(r.RemoteAddr, true) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, http.StatusUnauthorized)
			return
		}
		preout := make(map[string]string)
		config := g.ConfigSnapshot()
		selfConfig := config.Network[config.Addr]
		for _, v := range selfConfig.Topology {
			healthy, err := funcs.CheckAlertStatusContext(r.Context(), v)
			if errors.Is(err, funcs.ErrNoAlertSamples) {
				preout[v["Addr"]] = "unknown"
				continue
			}
			if err != nil {
				logrus.Error("[/api/topology.json] Check status ", err)
				http.Error(w, "Check topology status failed", http.StatusInternalServerError)
				return
			}
			if healthy {
				preout[v["Addr"]] = "true"
			} else {
				preout[v["Addr"]] = "false"
			}
		}
		w.Header().Set("Content-Type", "application/json")
		RenderJson(w, preout)
	})

	//报警API
	mux.HandleFunc("/api/alert.json", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		if !AuthUserIp(r.RemoteAddr) && !AuthAgentIp(r.RemoteAddr, true) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, http.StatusUnauthorized)
			return
		}
		type DateList struct {
			Ldate string
		}
		config := g.ConfigSnapshot()
		form := r.URL.Query()
		dtb := time.Now().Format("2006-01-02")
		if len(form["date"]) > 0 {
			dtb = strings.Replace(form["date"][0], "alertlog-", "", -1)
		}
		selectedDate, err := time.ParseInLocation("2006-01-02", dtb, g.LocalTimezone)
		if err != nil {
			http.Error(w, "Invalid date", http.StatusNotAcceptable)
			return
		}
		dayStart := selectedDate.Format("2006-01-02 15:04")
		dayEnd := selectedDate.AddDate(0, 0, 1).Format("2006-01-02 15:04")
		listpreout := []string{}
		datapreout := []g.AlertLog{}
		querySql := "select date(logtime) as ldate from alertlog group by date(logtime) order by date(logtime) desc"
		rows, err := g.Db.QueryContext(r.Context(), querySql)
		logrus.Debug("[func:/api/alert.json] Query ", querySql)
		if err != nil {
			logrus.Error("[func:/api/alert.json] Query ", err)
			http.Error(w, "Query alert dates failed", http.StatusInternalServerError)
			return
		} else {
			for rows.Next() {
				l := new(DateList)
				err := rows.Scan(&l.Ldate)
				if err != nil {
					logrus.Error("[/api/alert.json] Rows", err)
					rows.Close()
					http.Error(w, "Read alert dates failed", http.StatusInternalServerError)
					return
				}
				listpreout = append(listpreout, l.Ldate)
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				logrus.Error("[/api/alert.json] Rows", err)
				http.Error(w, "Read alert dates failed", http.StatusInternalServerError)
				return
			}
			rows.Close()
		}
		querySql = "select logtime,targetname,targetip,tracert from alertlog where logtime >= ? and logtime < ?"
		rows, err = g.Db.QueryContext(r.Context(), querySql, dayStart, dayEnd)
		logrus.Debug("[func:/api/alert.json] Query ", querySql)
		if err != nil {
			logrus.Error("[func:/api/alert.json] Query ", err)
			http.Error(w, "Query alert data failed", http.StatusInternalServerError)
			return
		} else {
			for rows.Next() {
				l := new(g.AlertLog)
				err := rows.Scan(&l.Logtime, &l.Targetname, &l.Targetip, &l.Tracert)
				l.Fromname = config.Name
				l.Fromip = config.Addr
				if err != nil {
					logrus.Error("[/api/alert.json] Rows", err)
					rows.Close()
					http.Error(w, "Read alert data failed", http.StatusInternalServerError)
					return
				}
				datapreout = append(datapreout, *l)
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				logrus.Error("[/api/alert.json] Rows", err)
				http.Error(w, "Read alert data failed", http.StatusInternalServerError)
				return
			}
			rows.Close()
		}
		RenderJson(w, []any{listpreout, datapreout})
	})

	//全国延迟API
	mux.HandleFunc("/api/mapping.json", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		if !AuthUserIp(r.RemoteAddr) && !AuthAgentIp(r.RemoteAddr, true) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, http.StatusUnauthorized)
			return
		}
		form := r.URL.Query()
		dataKey, err := resolveMappingDataKey(form, time.Now(), g.LocalTimezone)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotAcceptable)
			return
		}
		type Mapjson struct {
			Mapjson string
		}
		chinaMp := g.ChinaMp{}
		chinaMp.Text = g.ConfigSnapshot().Name
		chinaMp.Subtext = dataKey
		chinaMp.Avgdelay = emptyMappingData()
		querySql := "select mapjson from mappinglog where logtime = ?"
		mapRow := new(Mapjson)
		err = g.Db.QueryRowContext(r.Context(), querySql, dataKey).Scan(&mapRow.Mapjson)
		logrus.Debug("[func:/api/mapping.json] Query ", querySql)
		if err != nil {
			if err != sql.ErrNoRows {
				logrus.Error("[func:/api/mapping.json] Query ", err)
				http.Error(w, "Query mapping data failed", http.StatusInternalServerError)
				return
			}
		} else {
			chinaMp.Avgdelay, err = decodeMappingData(mapRow.Mapjson)
			if err != nil {
				logrus.Error("[/api/mapping.json] Json", err)
				http.Error(w, "Invalid mapping data", http.StatusInternalServerError)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		RenderJson(w, chinaMp)
	})

	//检测工具API
	mux.HandleFunc("/api/tools.json", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		if !AuthUserIp(r.RemoteAddr) && !AuthAgentIp(r.RemoteAddr, true) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, http.StatusUnauthorized)
			return
		}
		preout := g.ToolsRes{}
		preout.Status = "false"
		form := r.URL.Query()
		if len(form["t"]) == 0 {
			preout.Error = "target empty!"
			RenderJson(w, preout)
			return
		}
		target, err := normalizeToolTarget(form["t"][0])
		if err != nil {
			preout.Error = "Unable to resolve destination host"
			RenderJson(w, preout)
			return
		}
		if !acquireToolRequest() {
			preout.Error = "Too Many Tool Requests"
			w.Header().Set("Retry-After", "1")
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusTooManyRequests)
			RenderJson(w, preout)
			return
		}
		defer releaseToolRequest()
		nowtime := int(time.Now().Unix())
		if !allowToolRequest(r.RemoteAddr, nowtime, g.ConfigSnapshot().Toollimit) {
			preout.Error = "Time Limit Exceeded!"
			RenderJson(w, preout)
			return
		}
		preout.Ping = g.PingSt{}
		preout.Ping.MinDelay = -1
		lossPK := 0
		ipaddr, err := resolveToolIPAddrContext(r.Context(), target)
		if err != nil {
			if r.Context().Err() != nil {
				return
			}
			preout.Error = "Unable to resolve destination host"
			RenderJson(w, preout)
			return
		}
		preout.Ip = ipaddr.String()
		type pingResult struct {
			seq   int
			delay float64
		}
		const toolsPingCount = 5
		channel := make(chan pingResult, toolsPingCount)
		var wg sync.WaitGroup
	launchPings:
		for i := 0; i < toolsPingCount; i++ {
			if err := r.Context().Err(); err != nil {
				break
			}
			wg.Add(1)
			go func(seq int) {
				delay, err := nettools.RunPingContext(r.Context(), ipaddr, 3*time.Second, 64, seq)
				if err != nil {
					channel <- pingResult{seq: seq, delay: -1.00}
				} else {
					channel <- pingResult{seq: seq, delay: delay}
				}
				wg.Done()
			}(i)
			if i < toolsPingCount-1 {
				timer := time.NewTimer(100 * time.Millisecond)
				select {
				case <-timer.C:
				case <-r.Context().Done():
					timer.Stop()
					break launchPings
				}
			}
		}
		wg.Wait()
		if r.Context().Err() != nil {
			return
		}
		close(channel)
		delays := make([]float64, toolsPingCount)
		for res := range channel {
			delays[res.seq] = res.delay
		}
		for _, delay := range delays {
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
	mux.HandleFunc("/api/verify-password.json", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		if !AuthUserIp(r.RemoteAddr) && !AuthAgentIp(r.RemoteAddr, true) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, http.StatusUnauthorized)
			return
		}
		preout := make(map[string]string)
		if !parseFormLimited(w, r, maxPasswordFormBytes) {
			return
		}
		if !requireConfigPassword(w, r, g.ConfigSnapshot().Password) {
			return
		}
		preout["status"] = "true"
		RenderJson(w, preout)
	})

	//保存配置文件
	mux.HandleFunc("/api/saveconfig.json", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		if !AuthUserIp(r.RemoteAddr) && !AuthAgentIp(r.RemoteAddr, true) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, http.StatusUnauthorized)
			return
		}
		preout := make(map[string]string)
		if !parseFormLimited(w, r, maxConfigFormBytes) {
			return
		}
		preout["status"] = "false"
		currentConfig := g.ConfigSnapshot()
		if !requireConfigPassword(w, r, currentConfig.Password) {
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
		if err := validateConfig(nconfig); err != nil {
			preout["info"] = err.Error()
			RenderJson(w, preout)
			return
		}
		nconfig.Ver = currentConfig.Ver
		nconfig.Port = currentConfig.Port
		nconfig.Password = currentConfig.Password
		if err := g.ApplyConfig(nconfig); err != nil {
			preout["info"] = err.Error()
			RenderJson(w, preout)
			return
		}
		preout["status"] = "true"
		RenderJson(w, preout)
	})

	//代理访问
	mux.HandleFunc("/api/proxy.json", handleProxy)

}

func requireConfigPassword(w http.ResponseWriter, r *http.Request, expected string) bool {
	submitted := ""
	values := r.Form["password"]
	if len(values) > 0 {
		submitted = values[0]
	}
	matched, retryAfter := configPasswordAttempts.verify(r.RemoteAddr, submitted, expected, len(values) > 0, time.Now())
	if matched {
		return true
	}
	if retryAfter > 0 {
		retrySeconds := int((retryAfter + time.Second - 1) / time.Second)
		w.Header().Set("Retry-After", strconv.Itoa(retrySeconds))
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusTooManyRequests)
		RenderJson(w, map[string]any{
			"status":     "false",
			"error":      "password_rate_limited",
			"retryAfter": retrySeconds,
		})
		return false
	}
	RenderJson(w, map[string]string{"status": "false", "info": "密码错误!"})
	return false
}

func emptyMappingData() map[string][]g.MapVal {
	return map[string][]g.MapVal{
		"ctcc": {},
		"cucc": {},
		"cmcc": {},
	}
}

func decodeMappingData(raw string) (map[string][]g.MapVal, error) {
	stored := make(map[string][]g.MapVal)
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return nil, err
	}
	normalized := emptyMappingData()
	for carrier := range normalized {
		if values := stored[carrier]; values != nil {
			normalized[carrier] = values
		}
	}
	return normalized, nil
}

func handleProxy(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if !AuthUserIp(r.RemoteAddr) {
		o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
		http.Error(w, o, http.StatusUnauthorized)
		return
	}
	form := r.URL.Query()
	if len(form["g"]) == 0 {
		o := "Url Param Error!"
		http.Error(w, o, http.StatusNotAcceptable)
		return
	}
	defaultto := normalizeProxyTimeout(g.GetBaseInt("Timeout", 10))
	if len(form["t"]) > 0 {
		customTimeout, err := strconv.Atoi(form["t"][0])
		if err != nil || customTimeout < 1 || customTimeout > maxProxyTimeoutSeconds {
			o := "Timeout Param Error!"
			http.Error(w, o, http.StatusNotAcceptable)
			return
		}
		defaultto = customTimeout
	}
	rawTarget := form["g"][0]
	targetURL, err := validateProxyTarget(rawTarget)
	if err != nil {
		o := err.Error()
		http.Error(w, o, http.StatusNotAcceptable)
		return
	}
	if !acquireProxyRequest() {
		w.Header().Set("Retry-After", "1")
		http.Error(w, "Too Many Proxy Requests", http.StatusTooManyRequests)
		return
	}
	defer releaseProxyRequest()
	client := &http.Client{
		Timeout: time.Duration(defaultto) * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	outboundRequest, err := http.NewRequestWithContext(r.Context(), http.MethodGet, targetURL.String(), nil)
	if err != nil {
		http.Error(w, "Create Remote Request Error:"+err.Error(), http.StatusServiceUnavailable)
		return
	}
	resp, err := client.Do(outboundRequest)
	if err != nil {
		o := "Request Remote Data Error:" + err.Error()
		http.Error(w, o, http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()
	resCode := resp.StatusCode
	if resCode != http.StatusOK {
		// Other successful statuses do not provide the complete JSON response
		// required by the proxy API and must not look successful to clients.
		if resCode >= 200 && resCode < 300 {
			resCode = http.StatusBadGateway
		}
		o := "Get Remote Data Status Error"
		http.Error(w, o, resCode)
		return
	}
	body, err := readProxyHTTPResponseBody(resp)
	if err != nil {
		o := "Read Remote Data Error:" + err.Error()
		http.Error(w, o, http.StatusServiceUnavailable)
		return
	}
	var out bytes.Buffer
	out.Grow(len(body) + 1)
	if err := json.Indent(&out, body, "", "\t"); err != nil {
		http.Error(w, "Invalid Remote JSON Response", http.StatusBadGateway)
		return
	}
	_ = out.WriteByte('\n')
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	if _, err := w.Write(out.Bytes()); err != nil {
		logrus.Debug("[func:/api/proxy.json] Write response: ", err)
	}
}
