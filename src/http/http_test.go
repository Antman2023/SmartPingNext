package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"smartping/src/g"
	"strings"
	"testing"
	"time"
)

func cloneBoolMap(in map[string]bool) map[string]bool {
	if in == nil {
		return nil
	}
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func withAuthMaps(userMap map[string]bool, agentMap map[string]bool, fn func()) {
	g.AuthIpLock.Lock()
	oldUser := cloneBoolMap(g.AuthUserIpMap)
	oldAgent := cloneBoolMap(g.AuthAgentIpMap)
	g.AuthUserIpMap = cloneBoolMap(userMap)
	g.AuthAgentIpMap = cloneBoolMap(agentMap)
	g.AuthIpLock.Unlock()

	defer func() {
		g.AuthIpLock.Lock()
		g.AuthUserIpMap = oldUser
		g.AuthAgentIpMap = oldAgent
		g.AuthIpLock.Unlock()
	}()

	fn()
}

func TestParseRemoteIP(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "ipv4 hostport", in: "127.0.0.1:8899", want: "127.0.0.1"},
		{name: "ipv6 hostport", in: "[::1]:8899", want: "::1"},
		{name: "v4 mapped ipv6", in: "[::ffff:127.0.0.1]:8899", want: "127.0.0.1"},
		{name: "raw host", in: "localhost", want: "localhost"},
		{name: "empty", in: "", want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseRemoteIP(tc.in)
			if got != tc.want {
				t.Fatalf("parseRemoteIP(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeToolTarget(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		want    string
		wantErr bool
	}{
		{name: "hostname", target: "example.com", want: "example.com"},
		{name: "hostname and port", target: "example.com:443", want: "example.com"},
		{name: "url with path", target: "https://example.com/status", want: "example.com"},
		{name: "ipv4 with trailing slash", target: "http://127.0.0.1/", want: "127.0.0.1"},
		{name: "ipv6", target: "2001:db8::1", want: "2001:db8::1"},
		{name: "bracketed ipv6 and port", target: "[2001:db8::1]:443", want: "2001:db8::1"},
		{name: "unsupported scheme", target: "ftp://example.com/file", wantErr: true},
		{name: "empty", target: "  ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeToolTarget(tt.target)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("normalizeToolTarget(%q) should fail", tt.target)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeToolTarget(%q) returned error: %v", tt.target, err)
			}
			if got != tt.want {
				t.Fatalf("normalizeToolTarget(%q) = %q, want %q", tt.target, got, tt.want)
			}
		})
	}
}

func TestResolveToolIPAddrUsesIPv4(t *testing.T) {
	ipaddr, err := resolveToolIPAddr("127.0.0.1")
	if err != nil {
		t.Fatalf("resolveToolIPAddr returned error: %v", err)
	}
	if got := ipaddr.IP.String(); got != "127.0.0.1" {
		t.Fatalf("resolveToolIPAddr IP = %q, want 127.0.0.1", got)
	}

	if _, err := resolveToolIPAddr("2001:db8::1"); err == nil {
		t.Fatalf("resolveToolIPAddr should reject an IPv6-only target")
	}
}

func TestAuthUserIpSupportsIPv6(t *testing.T) {
	withAuthMaps(
		map[string]bool{"::1": true},
		nil,
		func() {
			if !AuthUserIp("[::1]:34567") {
				t.Fatalf("AuthUserIp should allow IPv6 loopback")
			}
			if AuthUserIp("127.0.0.1:34567") {
				t.Fatalf("AuthUserIp should reject address not in whitelist")
			}
		},
	)
}

func TestAuthAgentIpSupportsIPv6(t *testing.T) {
	withAuthMaps(
		map[string]bool{"10.0.0.1": true},
		map[string]bool{"::1": true},
		func() {
			if !AuthAgentIp("[::1]:40000", false) {
				t.Fatalf("AuthAgentIp should allow IPv6 agent address")
			}
			if AuthAgentIp("127.0.0.1:40000", false) {
				t.Fatalf("AuthAgentIp should reject address not in agent whitelist")
			}
		},
	)
}

func TestAuthAgentIpBypassConditions(t *testing.T) {
	withAuthMaps(
		map[string]bool{},
		map[string]bool{"1.1.1.1": true},
		func() {
			if !AuthAgentIp("not-a-hostport", true) {
				t.Fatalf("AuthAgentIp should allow when drt=true and user whitelist is empty")
			}
		},
	)

	withAuthMaps(
		map[string]bool{"10.0.0.1": true},
		map[string]bool{},
		func() {
			if !AuthAgentIp("not-a-hostport", false) {
				t.Fatalf("AuthAgentIp should allow when agent whitelist is empty")
			}
		},
	)
}

func TestRenderJsonSuccess(t *testing.T) {
	rec := httptest.NewRecorder()
	RenderJson(rec, map[string]string{"status": "ok"})

	if rec.Code != http.StatusOK {
		t.Fatalf("RenderJson status code = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("RenderJson content-type = %q, want contains application/json", ct)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"status":"ok"`) {
		t.Fatalf("RenderJson body = %q, want contains serialized json", body)
	}
}

func TestRenderJsonMarshalError(t *testing.T) {
	rec := httptest.NewRecorder()
	RenderJson(rec, map[string]any{"bad": make(chan int)})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("RenderJson status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestRequireMethod(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/saveconfig.json", nil)
	if requireMethod(recorder, request, http.MethodPost) {
		t.Fatalf("requireMethod should reject unexpected method")
	}
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
	if allow := recorder.Header().Get("Allow"); allow != http.MethodPost {
		t.Fatalf("Allow header = %q, want POST", allow)
	}
}

func TestPasswordMatches(t *testing.T) {
	tests := []struct {
		name      string
		submitted string
		expected  string
		want      bool
	}{
		{name: "matching password", submitted: "smartping", expected: "smartping", want: true},
		{name: "different password", submitted: "smartpong", expected: "smartping"},
		{name: "different length", submitted: "smartping-extra", expected: "smartping"},
		{name: "matching unicode password", submitted: "监控密码", expected: "监控密码", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := passwordMatches(tt.submitted, tt.expected); got != tt.want {
				t.Fatalf("passwordMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPasswordAttemptTrackerRateLimitsAndResets(t *testing.T) {
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	tracker := newPasswordAttemptTracker()

	for attempt := 1; attempt < passwordFailureLimit; attempt++ {
		matched, retryAfter := tracker.verify("192.0.2.10:1000", "wrong", "correct", true, now)
		if matched || retryAfter != 0 {
			t.Fatalf("attempt %d = matched %v, retry %v; want ordinary failure", attempt, matched, retryAfter)
		}
	}

	matched, retryAfter := tracker.verify("192.0.2.10:2000", "wrong", "correct", true, now)
	if matched || retryAfter != passwordFailureWindow {
		t.Fatalf("limit attempt = matched %v, retry %v; want retry %v", matched, retryAfter, passwordFailureWindow)
	}
	matched, retryAfter = tracker.verify("192.0.2.10:3000", "correct", "correct", true, now.Add(time.Minute))
	if matched || retryAfter != 4*time.Minute {
		t.Fatalf("locked correct attempt = matched %v, retry %v; want retry %v", matched, retryAfter, 4*time.Minute)
	}
	matched, retryAfter = tracker.verify("192.0.2.10:4000", "correct", "correct", true, now.Add(passwordFailureWindow))
	if !matched || retryAfter != 0 {
		t.Fatalf("attempt after window = matched %v, retry %v; want success", matched, retryAfter)
	}

	tracker.verify("192.0.2.10:5000", "wrong", "correct", true, now.Add(passwordFailureWindow))
	matched, retryAfter = tracker.verify("192.0.2.10:5000", "correct", "correct", true, now.Add(passwordFailureWindow))
	if !matched || retryAfter != 0 {
		t.Fatalf("correct password should clear failures: matched %v, retry %v", matched, retryAfter)
	}
	for attempt := 1; attempt < passwordFailureLimit; attempt++ {
		matched, retryAfter = tracker.verify("192.0.2.10:5000", "wrong", "correct", true, now.Add(passwordFailureWindow))
		if matched || retryAfter != 0 {
			t.Fatalf("post-reset attempt %d = matched %v, retry %v; want ordinary failure", attempt, matched, retryAfter)
		}
	}
}

func TestPasswordAttemptTrackerRejectsMissingEmptyPassword(t *testing.T) {
	tracker := newPasswordAttemptTracker()
	matched, retryAfter := tracker.verify("192.0.2.11:1000", "", "", false, time.Now())
	if matched || retryAfter != 0 {
		t.Fatalf("missing password = matched %v, retry %v; want ordinary failure", matched, retryAfter)
	}

	matched, retryAfter = tracker.verify("192.0.2.11:1000", "", "", true, time.Now())
	if !matched || retryAfter != 0 {
		t.Fatalf("submitted empty password = matched %v, retry %v; want success", matched, retryAfter)
	}
}

func TestPasswordAttemptTrackerBoundsClientState(t *testing.T) {
	tracker := newPasswordAttemptTracker()
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	for client := 0; client <= maxPasswordClients; client++ {
		remoteAddr := fmt.Sprintf("client-%d", client)
		tracker.verify(remoteAddr, "wrong", "correct", true, now.Add(time.Duration(client)*time.Second))
	}
	if got := len(tracker.attempts); got != maxPasswordClients {
		t.Fatalf("tracked clients = %d, want %d", got, maxPasswordClients)
	}
	if _, exists := tracker.attempts["client-0"]; exists {
		t.Fatalf("oldest client was not evicted")
	}
}

func TestResponseHeaders(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	server := newHTTPServer(":8899", handler)

	tests := []struct {
		name             string
		path             string
		wantCacheControl string
		wantPragma       string
	}{
		{
			name:             "API responses are not cached",
			path:             "/api/config.json",
			wantCacheControl: apiCacheControl,
			wantPragma:       "no-cache",
		},
		{
			name: "static responses preserve route cache policy",
			path: "/assets/index-abc123.js",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			server.Handler.ServeHTTP(recorder, request)

			if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
			}
			if got := recorder.Header().Get("Referrer-Policy"); got != "no-referrer" {
				t.Fatalf("Referrer-Policy = %q, want no-referrer", got)
			}
			if got := recorder.Header().Get("Cache-Control"); got != tt.wantCacheControl {
				t.Fatalf("Cache-Control = %q, want %q", got, tt.wantCacheControl)
			}
			if got := recorder.Header().Get("Pragma"); got != tt.wantPragma {
				t.Fatalf("Pragma = %q, want %q", got, tt.wantPragma)
			}
		})
	}
}

func TestResponseHeadersRejectCrossOriginWrites(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		path          string
		origin        string
		fetchSite     string
		wantStatus    int
		wantCallCount int
	}{
		{
			name:          "same-origin POST",
			method:        http.MethodPost,
			origin:        "https://monitor.example",
			wantStatus:    http.StatusNoContent,
			wantCallCount: 1,
		},
		{
			name:       "cross-origin POST",
			method:     http.MethodPost,
			origin:     "https://attacker.example",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "cross-site fetch metadata",
			method:     http.MethodPost,
			fetchSite:  "cross-site",
			wantStatus: http.StatusForbidden,
		},
		{
			name:          "GET with only an Origin header remains readable by the handler",
			method:        http.MethodGet,
			origin:        "https://attacker.example",
			wantStatus:    http.StatusNoContent,
			wantCallCount: 1,
		},
		{
			name:       "cross-site API GET",
			method:     http.MethodGet,
			fetchSite:  "cross-site",
			wantStatus: http.StatusForbidden,
		},
		{
			name:          "cross-site static GET",
			method:        http.MethodGet,
			path:          "/assets/index.js",
			fetchSite:     "cross-site",
			wantStatus:    http.StatusNoContent,
			wantCallCount: 1,
		},
		{
			name:          "CLI POST without browser headers",
			method:        http.MethodPost,
			wantStatus:    http.StatusNoContent,
			wantCallCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				callCount++
				w.WriteHeader(http.StatusNoContent)
			})
			server := newHTTPServer(":8899", handler)
			recorder := httptest.NewRecorder()
			path := tt.path
			if path == "" {
				path = "/api/saveconfig.json"
			}
			request := httptest.NewRequest(tt.method, path, nil)
			request.Host = "monitor.example"
			if tt.origin != "" {
				request.Header.Set("Origin", tt.origin)
			}
			if tt.fetchSite != "" {
				request.Header.Set("Sec-Fetch-Site", tt.fetchSite)
			}
			server.Handler.ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
			if callCount != tt.wantCallCount {
				t.Fatalf("handler call count = %d, want %d", callCount, tt.wantCallCount)
			}
		})
	}
}

func TestAppHandlerServesSanitizedConfig(t *testing.T) {
	oldConfig := g.ConfigSnapshot()
	g.SetConfig(g.Config{
		Name:       "local-node",
		Addr:       "127.0.0.1",
		Password:   "must-not-leak",
		Authiplist: "",
		Network: map[string]g.NetworkMember{
			"127.0.0.1": {Name: "local-node", Addr: "127.0.0.1"},
		},
	})
	defer g.SetConfig(oldConfig)

	server := newHTTPServer(":8899", newAppHandler())
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/config.json", nil)
	server.Handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("config endpoint status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Cache-Control"); got != apiCacheControl {
		t.Fatalf("Cache-Control = %q, want %q", got, apiCacheControl)
	}
	var config g.Config
	if err := json.Unmarshal(recorder.Body.Bytes(), &config); err != nil {
		t.Fatalf("decode config response: %v", err)
	}
	if config.Name != "local-node" {
		t.Fatalf("config name = %q, want local-node", config.Name)
	}
	if config.Password != "" {
		t.Fatalf("config endpoint leaked password %q", config.Password)
	}
}

func TestAppHandlerRejectsWrongConfigMethod(t *testing.T) {
	server := newHTTPServer(":8899", newAppHandler())
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/config.json", nil)
	server.Handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("config endpoint status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
	if got := recorder.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("Allow = %q, want GET", got)
	}
}

func TestAppHandlerRejectsInvalidMappingTime(t *testing.T) {
	server := newHTTPServer(":8899", newAppHandler())
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/mapping.json?d=not-a-time", nil)
	server.Handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotAcceptable {
		t.Fatalf("mapping endpoint status = %d, want %d", recorder.Code, http.StatusNotAcceptable)
	}
	if !strings.Contains(recorder.Body.String(), "Invalid Mapping Time") {
		t.Fatalf("mapping endpoint body = %q, want validation error", recorder.Body.String())
	}
}

func TestAppHandlerSavesConfigAndProtectsServerFields(t *testing.T) {
	oldConfig := g.ConfigSnapshot()
	oldRoot := g.Root
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "conf"), 0755); err != nil {
		t.Fatalf("create config directory: %v", err)
	}
	current := validTestConfig()
	current.Ver = "v1.7.24"
	current.Password = "current-password"
	g.Root = root
	g.SetConfig(current)
	defer func() {
		g.Root = oldRoot
		g.SetConfig(oldConfig)
	}()

	candidate := validTestConfig()
	candidate.Name = "updated-node"
	candidate.Port = 9999
	candidate.Ver = "client-version"
	candidate.Password = "client-password"
	candidateJSON, err := json.Marshal(candidate)
	if err != nil {
		t.Fatalf("encode candidate config: %v", err)
	}
	handler := newHTTPServer(":8899", newAppHandler()).Handler
	configPath := filepath.Join(root, "conf", "config.json")

	t.Run("wrong password does not write config", func(t *testing.T) {
		form := url.Values{
			"password": {"wrong-password"},
			"config":   {string(candidateJSON)},
		}
		request := httptest.NewRequest(http.MethodPost, "/api/saveconfig.json", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("save status = %d, want %d", recorder.Code, http.StatusOK)
		}
		var result map[string]string
		if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
			t.Fatalf("decode save response: %v", err)
		}
		if result["status"] != "false" {
			t.Fatalf("save status payload = %q, want false", result["status"])
		}
		if _, err := os.Stat(configPath); !os.IsNotExist(err) {
			t.Fatalf("wrong password created config file: %v", err)
		}
	})

	t.Run("valid password saves sanitized config", func(t *testing.T) {
		form := url.Values{
			"password": {current.Password},
			"config":   {string(candidateJSON)},
		}
		request := httptest.NewRequest(http.MethodPost, "/api/saveconfig.json", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("save status = %d, want %d", recorder.Code, http.StatusOK)
		}
		var result map[string]string
		if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
			t.Fatalf("decode save response: %v", err)
		}
		if result["status"] != "true" {
			t.Fatalf("save status payload = %q, want true: %s", result["status"], recorder.Body.String())
		}

		saved := g.ConfigSnapshot()
		if saved.Name != candidate.Name {
			t.Fatalf("saved name = %q, want %q", saved.Name, candidate.Name)
		}
		if saved.Port != current.Port || saved.Ver != current.Ver || saved.Password != current.Password {
			t.Fatalf("protected fields changed: port=%d version=%q password=%q", saved.Port, saved.Ver, saved.Password)
		}
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("read saved config: %v", err)
		}
		var diskConfig g.Config
		if err := json.Unmarshal(data, &diskConfig); err != nil {
			t.Fatalf("decode saved config: %v", err)
		}
		if diskConfig.Name != candidate.Name || diskConfig.Port != current.Port || diskConfig.Password != current.Password {
			t.Fatalf("disk config does not match applied config: %#v", diskConfig)
		}
	})
}

func TestConfigPasswordRateLimitIsSharedAcrossEndpoints(t *testing.T) {
	oldConfig := g.ConfigSnapshot()
	oldAttempts := configPasswordAttempts
	configPasswordAttempts = newPasswordAttemptTracker()
	g.SetConfig(g.Config{Password: "correct-password"})
	defer func() {
		configPasswordAttempts = oldAttempts
		g.SetConfig(oldConfig)
	}()

	handler := newHTTPServer(":8899", newAppHandler()).Handler
	remoteAddr := "192.0.2.25:1234"
	for attempt := 1; attempt <= passwordFailureLimit; attempt++ {
		form := url.Values{"password": {"wrong-password"}}
		request := httptest.NewRequest(http.MethodPost, "/api/verify-password.json", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.RemoteAddr = remoteAddr
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		wantStatus := http.StatusOK
		if attempt == passwordFailureLimit {
			wantStatus = http.StatusTooManyRequests
		}
		if recorder.Code != wantStatus {
			t.Fatalf("attempt %d status = %d, want %d: %s", attempt, recorder.Code, wantStatus, recorder.Body.String())
		}
		if attempt == passwordFailureLimit && recorder.Header().Get("Retry-After") == "" {
			t.Fatalf("rate-limited response is missing Retry-After")
		}
	}

	form := url.Values{"password": {"correct-password"}}
	request := httptest.NewRequest(http.MethodPost, "/api/saveconfig.json", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.RemoteAddr = remoteAddr
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("shared save limit status = %d, want %d: %s", recorder.Code, http.StatusTooManyRequests, recorder.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode rate-limit response: %v", err)
	}
	if result["error"] != "password_rate_limited" {
		t.Fatalf("rate-limit error = %v, want password_rate_limited", result["error"])
	}
}

func TestSetStaticCacheHeaders(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		spaFallback bool
		want        string
	}{
		{name: "hashed asset", path: "/assets/index-AbCd1234.js", want: immutableAssetCacheControl},
		{name: "regular static file", path: "/favicon.ico", want: staticFileCacheControl},
		{name: "index file", path: "/index.html", want: indexCacheControl},
		{name: "root", path: "/", want: indexCacheControl},
		{name: "SPA route", path: "/topology", spaFallback: true, want: indexCacheControl},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			setStaticCacheHeaders(recorder, tt.path, tt.spaFallback)
			if got := recorder.Header().Get("Cache-Control"); got != tt.want {
				t.Fatalf("Cache-Control = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseFormLimited(t *testing.T) {
	t.Run("accepts form within limit", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("password=ok"))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if !parseFormLimited(recorder, request, 64) {
			t.Fatalf("parseFormLimited should accept small form: %s", recorder.Body.String())
		}
		if got := request.Form.Get("password"); got != "ok" {
			t.Fatalf("password = %q, want ok", got)
		}
	})

	t.Run("rejects oversized form", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("password=too-long"))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if parseFormLimited(recorder, request, 8) {
			t.Fatalf("parseFormLimited should reject oversized form")
		}
		if recorder.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusRequestEntityTooLarge)
		}
	})
}

func TestResolvePingTimeRange(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 34, 45, 0, time.UTC)

	t.Run("default six hours", func(t *testing.T) {
		start, end, err := resolvePingTimeRange(url.Values{}, now, time.UTC)
		if err != nil {
			t.Fatalf("resolvePingTimeRange returned error: %v", err)
		}
		if got := end.Sub(start); got != 6*time.Hour {
			t.Fatalf("default range = %v, want %v", got, 6*time.Hour)
		}
		if end.Second() != 0 {
			t.Fatalf("default end should be minute-aligned: %v", end)
		}
	})

	t.Run("valid custom range", func(t *testing.T) {
		values := url.Values{
			"starttime": {"2026-07-10 12:00"},
			"endtime":   {"2026-07-17 12:00"},
		}
		if _, _, err := resolvePingTimeRange(values, now, time.UTC); err != nil {
			t.Fatalf("valid range returned error: %v", err)
		}
	})

	cases := map[string]url.Values{
		"missing end": {
			"starttime": {"2026-07-10 12:00"},
		},
		"invalid date": {
			"starttime": {"invalid"},
			"endtime":   {"2026-07-17 12:00"},
		},
		"reversed": {
			"starttime": {"2026-07-17 12:00"},
			"endtime":   {"2026-07-10 12:00"},
		},
		"over limit": {
			"starttime": {"2026-06-01 12:00"},
			"endtime":   {"2026-07-17 12:00"},
		},
	}
	for name, values := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, err := resolvePingTimeRange(values, now, time.UTC); err == nil {
				t.Fatalf("resolvePingTimeRange should reject %s", name)
			}
		})
	}
}

func TestResolveMappingDataKey(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, 9, 2, 4, 30, 45, 0, time.UTC)

	key, err := resolveMappingDataKey(url.Values{}, now, location)
	if err != nil || key != "2026-09-02 12:29" {
		t.Fatalf("default key = %q, err %v; want 2026-09-02 12:29", key, err)
	}

	key, err = resolveMappingDataKey(url.Values{"d": {"2026-09-01 08:05"}}, now, location)
	if err != nil || key != "2026-09-01 08:05" {
		t.Fatalf("explicit key = %q, err %v; want preserved value", key, err)
	}

	invalidValues := []url.Values{
		{"d": {""}},
		{"d": {"2026-09-01"}},
		{"d": {"2026-09-01 08:05", "2026-09-01 08:06"}},
	}
	for _, values := range invalidValues {
		if _, err := resolveMappingDataKey(values, now, location); err == nil {
			t.Fatalf("resolveMappingDataKey should reject %#v", values)
		}
	}
}

func TestCompletedPingTimelineSize(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, 8, 13, 6, 25, 30, 0, time.UTC)

	tests := []struct {
		name       string
		lastcheck  []string
		populated  []bool
		wantLength int
	}{
		{
			name:       "removes missing current minute in server timezone",
			lastcheck:  []string{"2026-08-13 14:24", "2026-08-13 14:25"},
			populated:  []bool{true, false},
			wantLength: 1,
		},
		{
			name:       "keeps populated current minute even when values may be zero",
			lastcheck:  []string{"2026-08-13 14:25"},
			populated:  []bool{true},
			wantLength: 1,
		},
		{
			name:       "keeps a missing historical minute",
			lastcheck:  []string{"2026-08-13 14:23", "2026-08-13 14:24"},
			populated:  []bool{true, false},
			wantLength: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := completedPingTimelineSize(tt.lastcheck, tt.populated, now, location); got != tt.wantLength {
				t.Fatalf("completedPingTimelineSize() = %d, want %d", got, tt.wantLength)
			}
		})
	}
}

func TestDecodeMappingDataNormalizesResponseShape(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want map[string]int
	}{
		{
			name: "complete data",
			raw:  `{"ctcc":[{"value":12.5,"name":"北京"}],"cucc":[],"cmcc":[]}`,
			want: map[string]int{"ctcc": 1, "cucc": 0, "cmcc": 0},
		},
		{
			name: "null document",
			raw:  `null`,
			want: map[string]int{"ctcc": 0, "cucc": 0, "cmcc": 0},
		},
		{
			name: "missing and null carriers",
			raw:  `{"ctcc":null,"unknown":[{"value":1,"name":"ignored"}]}`,
			want: map[string]int{"ctcc": 0, "cucc": 0, "cmcc": 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeMappingData(tt.raw)
			if err != nil {
				t.Fatalf("decodeMappingData returned error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("carrier count = %d, want %d: %#v", len(got), len(tt.want), got)
			}
			for carrier, wantLength := range tt.want {
				values, ok := got[carrier]
				if !ok || values == nil || len(values) != wantLength {
					t.Fatalf("carrier %s = %#v, want non-nil length %d", carrier, values, wantLength)
				}
			}
		})
	}

	if _, err := decodeMappingData(`[]`); err == nil {
		t.Fatal("decodeMappingData should reject non-object data")
	}
}

func TestAllowToolRequestUsesClientIPAndCleansExpiredEntries(t *testing.T) {
	g.ToolLimitLock.Lock()
	oldToolLimit := g.ToolLimit
	g.ToolLimit = map[string]int{"expired": 800}
	g.ToolLimitLock.Unlock()
	defer func() {
		g.ToolLimitLock.Lock()
		g.ToolLimit = oldToolLimit
		g.ToolLimitLock.Unlock()
	}()

	if !allowToolRequest("192.0.2.1:10001", 1000, 30) {
		t.Fatalf("first request should be allowed")
	}
	if allowToolRequest("192.0.2.1:10002", 1001, 30) {
		t.Fatalf("same client IP should be rate-limited across source ports")
	}
	g.ToolLimitLock.RLock()
	_, expiredExists := g.ToolLimit["expired"]
	_, ipExists := g.ToolLimit["192.0.2.1"]
	g.ToolLimitLock.RUnlock()
	if expiredExists {
		t.Fatalf("expired rate-limit entry should be removed")
	}
	if !ipExists {
		t.Fatalf("rate-limit map should use normalized client IP")
	}
}

func TestAllowToolRequestAllowsZeroLimitAndExactBoundary(t *testing.T) {
	g.ToolLimitLock.Lock()
	oldToolLimit := g.ToolLimit
	g.ToolLimit = map[string]int{}
	g.ToolLimitLock.Unlock()
	defer func() {
		g.ToolLimitLock.Lock()
		g.ToolLimit = oldToolLimit
		g.ToolLimitLock.Unlock()
	}()

	if !allowToolRequest("192.0.2.1:10001", 1000, 0) || !allowToolRequest("192.0.2.1:10002", 1000, 0) {
		t.Fatalf("zero limit should disable rate limiting")
	}
	if len(g.ToolLimit) != 0 {
		t.Fatalf("disabled rate limiting should not retain client entries: %#v", g.ToolLimit)
	}

	if !allowToolRequest("192.0.2.1:10001", 1000, 30) {
		t.Fatalf("first request should be allowed")
	}
	if allowToolRequest("192.0.2.1:10002", 1029, 30) {
		t.Fatalf("request before the boundary should be rate-limited")
	}
	if !allowToolRequest("192.0.2.1:10003", 1030, 30) {
		t.Fatalf("request at the exact boundary should be allowed")
	}
}

func TestNewHTTPServerTimeouts(t *testing.T) {
	server := newHTTPServer(":8899", http.NewServeMux())
	if server.ReadHeaderTimeout <= 0 || server.ReadTimeout <= 0 || server.WriteTimeout <= 0 || server.IdleTimeout <= 0 {
		t.Fatalf("server timeouts must all be positive: %#v", server)
	}
	if server.MaxHeaderBytes <= 0 {
		t.Fatalf("MaxHeaderBytes must be positive")
	}
}

func TestNewServerUsesConfiguredPort(t *testing.T) {
	g.CfgLock.Lock()
	oldPort := g.Cfg.Port
	g.Cfg.Port = 19091
	g.CfgLock.Unlock()
	defer func() {
		g.CfgLock.Lock()
		g.Cfg.Port = oldPort
		g.CfgLock.Unlock()
	}()

	server := NewServer()
	if server.Addr != ":19091" {
		t.Fatalf("server address = %q, want :19091", server.Addr)
	}
	if server.Handler == nil {
		t.Fatal("server handler must be configured")
	}
}
