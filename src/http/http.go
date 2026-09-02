package http

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"smartping/src/g"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const maxPingRangeMinutes = 31 * 24 * 60

const (
	maxPasswordFormBytes  = 64 << 10
	maxConfigFormBytes    = 16 << 20
	apiCacheControl       = "no-store"
	passwordFailureLimit  = 5
	passwordFailureWindow = 5 * time.Minute
	maxPasswordClients    = 4096
)

type passwordAttempt struct {
	failures    int
	windowStart time.Time
}

type passwordAttemptTracker struct {
	mu       sync.Mutex
	attempts map[string]passwordAttempt
}

var configPasswordAttempts = newPasswordAttemptTracker()

func newPasswordAttemptTracker() *passwordAttemptTracker {
	return &passwordAttemptTracker{attempts: make(map[string]passwordAttempt)}
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

func passwordMatches(submitted, expected string) bool {
	submittedDigest := sha256.Sum256([]byte(submitted))
	expectedDigest := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(submittedDigest[:], expectedDigest[:]) == 1
}

func (t *passwordAttemptTracker) verify(remoteAddr, submitted, expected string, submittedPresent bool, now time.Time) (bool, time.Duration) {
	clientKey := parseRemoteIP(remoteAddr)
	if clientKey == "" {
		clientKey = strings.TrimSpace(remoteAddr)
	}
	if clientKey == "" {
		clientKey = "unknown"
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	attempt, exists := t.attempts[clientKey]
	if exists && (now.Before(attempt.windowStart) || !now.Before(attempt.windowStart.Add(passwordFailureWindow))) {
		delete(t.attempts, clientKey)
		attempt = passwordAttempt{}
		exists = false
	}
	if exists && attempt.failures >= passwordFailureLimit {
		return false, attempt.windowStart.Add(passwordFailureWindow).Sub(now)
	}
	if submittedPresent && passwordMatches(submitted, expected) {
		delete(t.attempts, clientKey)
		return true, 0
	}

	if !exists {
		t.makeRoomForClient()
		attempt.windowStart = now
	}
	attempt.failures++
	t.attempts[clientKey] = attempt
	if attempt.failures >= passwordFailureLimit {
		return false, attempt.windowStart.Add(passwordFailureWindow).Sub(now)
	}
	return false, 0
}

func (t *passwordAttemptTracker) makeRoomForClient() {
	if len(t.attempts) < maxPasswordClients {
		return
	}
	oldestKey := ""
	var oldestStart time.Time
	for key, attempt := range t.attempts {
		if oldestKey == "" || attempt.windowStart.Before(oldestStart) {
			oldestKey = key
			oldestStart = attempt.windowStart
		}
	}
	delete(t.attempts, oldestKey)
}

func withResponseHeaders(next http.Handler) http.Handler {
	if next == nil {
		next = http.DefaultServeMux
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", apiCacheControl)
			w.Header().Set("Pragma", "no-cache")
		}
		if isForbiddenCrossOriginRequest(r) {
			http.Error(w, "Cross-Origin Request Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isForbiddenCrossOriginRequest(r *http.Request) bool {
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")), "cross-site") &&
		strings.HasPrefix(r.URL.Path, "/api/") {
		return true
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}

	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return false
	}
	originURL, err := url.Parse(origin)
	if err != nil || originURL.Host == "" || originURL.User != nil {
		return true
	}
	return !strings.EqualFold(originURL.Host, r.Host)
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

func normalizeToolTarget(rawTarget string) (string, error) {
	target := strings.TrimSpace(rawTarget)
	if target == "" {
		return "", errors.New("target empty")
	}

	if parsedIP := net.ParseIP(strings.Trim(target, "[]")); parsedIP != nil {
		return parsedIP.String(), nil
	}

	parseTarget := target
	if !strings.Contains(target, "://") {
		parseTarget = "//" + target
	}
	parsedURL, err := url.Parse(parseTarget)
	if err != nil {
		return "", errors.New("invalid target")
	}
	if parsedURL.Scheme != "" && parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", errors.New("invalid target")
	}

	host := strings.TrimSuffix(parsedURL.Hostname(), ".")
	if host == "" {
		return "", errors.New("invalid target")
	}
	return host, nil
}

func resolveToolIPAddr(target string) (*net.IPAddr, error) {
	ipaddr, err := net.ResolveIPAddr("ip4", target)
	if err != nil || ipaddr == nil || ipaddr.IP.To4() == nil {
		return nil, errors.New("unable to resolve IPv4 destination host")
	}
	return ipaddr, nil
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

func resolveMappingDataKey(values url.Values, now time.Time, location *time.Location) (string, error) {
	if location == nil {
		location = time.Local
	}
	defaultKey := now.In(location).Add(-time.Minute).Format("2006-01-02 15:04")
	raw, exists := values["d"]
	if !exists {
		return defaultKey, nil
	}
	if len(raw) != 1 || raw[0] == "" {
		return "", errors.New("Invalid Mapping Time!")
	}
	if _, err := time.ParseInLocation("2006-01-02 15:04", raw[0], location); err != nil {
		return "", errors.New("Invalid Mapping Time!")
	}
	return raw[0], nil
}

func completedPingTimelineSize(lastcheck []string, populated []bool, now time.Time, location *time.Location) int {
	size := len(lastcheck)
	if size == 0 || len(populated) < size {
		return size
	}
	if location == nil {
		location = time.Local
	}
	currentMinute := now.In(location).Format("2006-01-02 15:04")
	if lastcheck[size-1] == currentMinute && !populated[size-1] {
		return size - 1
	}
	return size
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
	server := NewServer()
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Println("[StartHttp]", err)
	}
}

func NewServer() *http.Server {
	config := g.ConfigSnapshot()
	logrus.Info("[func:StartHttp] starting to listen on ", config.Port)
	return newHTTPServer(fmt.Sprintf(":%d", config.Port), newAppHandler())
}

func newAppHandler() http.Handler {
	mux := http.NewServeMux()
	configApiRoutes(mux)
	configIndexRoutes(mux)
	return mux
}

func newHTTPServer(address string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           withResponseHeaders(handler),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}
