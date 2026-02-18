package http

import (
	"net/http"
	"net/http/httptest"
	"smartping/src/g"
	"strings"
	"testing"
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

func TestValidIP4(t *testing.T) {
	cases := []struct {
		name string
		ip   string
		want bool
	}{
		{name: "valid", ip: "192.168.1.1", want: true},
		{name: "valid with spaces", ip: " 10.0.0.1 ", want: true},
		{name: "invalid v6", ip: "::1", want: false},
		{name: "invalid range", ip: "256.1.1.1", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidIP4(tc.ip)
			if got != tc.want {
				t.Fatalf("ValidIP4(%q) = %v, want %v", tc.ip, got, tc.want)
			}
		})
	}
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
