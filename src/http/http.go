package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"smartping/src/g"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

var validIP4Regexp = regexp.MustCompile(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)

const maxPingRangeMinutes = 31 * 24 * 60

const (
	maxPasswordFormBytes = 64 << 10
	maxConfigFormBytes   = 16 << 20
)

func ValidIP4(ipAddress string) bool {
	ipAddress = strings.TrimSpace(ipAddress)
	return validIP4Regexp.MatchString(ipAddress)
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

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	w.Header().Set("Allow", method)
	http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	return false
}

func parseFormLimited(w http.ResponseWriter, r *http.Request, maxBytes int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	if err := r.ParseForm(); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			http.Error(w, "Request Body Too Large", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "Invalid Form Data", http.StatusBadRequest)
		}
		return false
	}
	return true
}

func AuthUserIp(RemoteAddr string) bool {
	g.AuthIpLock.RLock()
	defer g.AuthIpLock.RUnlock()
	if len(g.AuthUserIpMap) == 0 {
		return true
	}

	ip := parseRemoteIP(RemoteAddr)
	if ip == "" {
		return false
	}

	if _, ok := g.AuthUserIpMap[ip]; ok {
		return true
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

	ip := parseRemoteIP(RemoteAddr)
	if ip == "" {
		return false
	}

	if _, ok := g.AuthAgentIpMap[ip]; ok {
		return true
	}
	return false
}

func parseRemoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err != nil {
		host = strings.TrimSpace(remoteAddr)
	}

	host = strings.Trim(host, "[]")
	if host == "" {
		return ""
	}

	parsed := net.ParseIP(host)
	if parsed == nil {
		return host
	}

	if v4 := parsed.To4(); v4 != nil {
		return v4.String()
	}
	return parsed.String()
}

func resolvePingTimeRange(values url.Values, now time.Time, location *time.Location) (time.Time, time.Time, error) {
	if location == nil {
		location = time.Local
	}
	end := now.In(location).Truncate(time.Minute)
	start := end.Add(-6 * time.Hour)
	startRaw, hasStart := values["starttime"]
	endRaw, hasEnd := values["endtime"]
	if !hasStart && !hasEnd {
		return start, end, nil
	}
	if !hasStart || !hasEnd || len(startRaw) == 0 || len(endRaw) == 0 || startRaw[0] == "" || endRaw[0] == "" {
		return time.Time{}, time.Time{}, errors.New("Invalid Time Range!")
	}

	parsedStart, err := time.ParseInLocation("2006-01-02 15:04", startRaw[0], location)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("Invalid Time Range!")
	}
	parsedEnd, err := time.ParseInLocation("2006-01-02 15:04", endRaw[0], location)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("Invalid Time Range!")
	}
	delta := parsedEnd.Unix() - parsedStart.Unix()
	if delta < 0 || delta > int64(maxPingRangeMinutes*60) {
		return time.Time{}, time.Time{}, errors.New("Invalid Time Range!")
	}
	return parsedStart, parsedEnd, nil
}

func allowToolRequest(remoteAddr string, now int, limit int) bool {
	if limit <= 0 {
		return true
	}

	clientKey := parseRemoteIP(remoteAddr)
	if clientKey == "" {
		clientKey = remoteAddr
	}
	retention := limit
	if retention < 60 {
		retention = 60
	}

	g.ToolLimitLock.Lock()
	defer g.ToolLimitLock.Unlock()
	if g.ToolLimit == nil {
		g.ToolLimit = make(map[string]int)
	}
	for key, lastSeen := range g.ToolLimit {
		if now-lastSeen > retention {
			delete(g.ToolLimit, key)
		}
	}
	if lastSeen, ok := g.ToolLimit[clientKey]; ok && now-lastSeen < limit {
		return false
	}
	g.ToolLimit[clientKey] = now
	return true
}

func StartHttp() {
	configApiRoutes()
	configIndexRoutes()
	config := g.ConfigSnapshot()
	logrus.Info("[func:StartHttp] starting to listen on ", config.Port)
	server := newHTTPServer(fmt.Sprintf(":%d", config.Port), nil)
	err := server.ListenAndServe()
	if err != nil {
		log.Fatalln("[StartHttp]", err)
	}
}

func newHTTPServer(address string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}
