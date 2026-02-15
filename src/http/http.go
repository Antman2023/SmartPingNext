package http

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"smartping/src/g"
	"strings"

	"github.com/cihub/seelog"
)

func ValidIP4(ipAddress string) bool {
	ipAddress = strings.Trim(ipAddress, " ")
	re, _ := regexp.Compile(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)
	return re.MatchString(ipAddress)
}

func RenderJson(w http.ResponseWriter, v any) {
	bs, err := json.Marshal(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Write(bs)
}

func AuthUserIp(RemoteAddr string) bool {
	g.AuthIpLock.RLock()
	defer g.AuthIpLock.RUnlock()
	if len(g.AuthUserIpMap) == 0 {
		return true
	}
	ips := strings.Split(RemoteAddr, ":")
	if len(ips) == 2 {
		if _, ok := g.AuthUserIpMap[ips[0]]; ok {
			return true
		}
	}
	return false
}

func AuthAgentIp(RemoteAddr string, drt bool) bool {
	g.AuthIpLock.RLock()
	defer g.AuthIpLock.RUnlock()
	if drt {
		if len(g.AuthUserIpMap) == 0 {
			return true
		}
	}
	if len(g.AuthAgentIpMap) == 0 {
		return true
	}
	ips := strings.Split(RemoteAddr, ":")
	if len(ips) == 2 {
		if _, ok := g.AuthAgentIpMap[ips[0]]; ok {
			return true
		}
	}
	return false
}

func StartHttp() {
	configApiRoutes()
	configIndexRoutes()
	seelog.Info("[func:StartHttp] starting to listen on ", g.Cfg.Port)
	s := fmt.Sprintf(":%d", g.Cfg.Port)
	err := http.ListenAndServe(s, nil)
	if err != nil {
		log.Fatalln("[StartHttp]", err)
	}
	os.Exit(0)
}
