package http

import (
	"net/http"
	"net/http/httptest"
	"net/url"
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
