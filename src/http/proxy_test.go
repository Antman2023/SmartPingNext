package http

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"smartping/src/g"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func withProxyConfig(cfg g.Config, fn func()) {
	oldCfg := g.Cfg
	g.Cfg = cfg
	defer func() {
		g.Cfg = oldCfg
	}()
	fn()
}

func TestValidateProxyTargetAllowsConfiguredNodeAndApi(t *testing.T) {
	withProxyConfig(g.Config{
		Port: 8899,
		Network: map[string]g.NetworkMember{
			"10.0.0.1": {Addr: "10.0.0.1"},
			"10.0.0.2": {Addr: "10.0.0.2"},
		},
	}, func() {
		targetURL, err := validateProxyTarget("http://10.0.0.2:8899/api/ping.json?ip=10.0.0.1&starttime=2026-03-06+10%3A00&endtime=2026-03-06+11%3A00")
		if err != nil {
			t.Fatalf("validateProxyTarget returned error: %v", err)
		}
		if got := targetURL.String(); got != "http://10.0.0.2:8899/api/ping.json?endtime=2026-03-06+11%3A00&ip=10.0.0.1&starttime=2026-03-06+10%3A00" {
			t.Fatalf("validateProxyTarget normalized url = %q", got)
		}
	})
}

func TestValidateProxyTargetRejectsUnknownHost(t *testing.T) {
	withProxyConfig(g.Config{
		Port: 8899,
		Network: map[string]g.NetworkMember{
			"10.0.0.1": {Addr: "10.0.0.1"},
		},
	}, func() {
		if _, err := validateProxyTarget("http://10.0.0.99:8899/api/config.json"); err == nil {
			t.Fatalf("validateProxyTarget should reject unknown host")
		}
	})
}

func TestValidateProxyTargetRejectsDisallowedPath(t *testing.T) {
	withProxyConfig(g.Config{
		Port: 8899,
		Network: map[string]g.NetworkMember{
			"10.0.0.1": {Addr: "10.0.0.1"},
		},
	}, func() {
		if _, err := validateProxyTarget("http://10.0.0.1:8899/api/saveconfig.json"); err == nil {
			t.Fatalf("validateProxyTarget should reject disallowed api path")
		}
	})
}

func TestValidateProxyTargetRejectsDisallowedQueryKey(t *testing.T) {
	withProxyConfig(g.Config{
		Port: 8899,
		Network: map[string]g.NetworkMember{
			"10.0.0.1": {Addr: "10.0.0.1"},
		},
	}, func() {
		if _, err := validateProxyTarget("http://10.0.0.1:8899/api/config.json?debug=true"); err == nil {
			t.Fatalf("validateProxyTarget should reject unexpected query key")
		}
	})
}

func TestValidateProxyTargetRejectsMissingRequiredQuery(t *testing.T) {
	withProxyConfig(g.Config{
		Port: 8899,
		Network: map[string]g.NetworkMember{
			"10.0.0.1": {Addr: "10.0.0.1"},
		},
	}, func() {
		if _, err := validateProxyTarget("http://10.0.0.1:8899/api/tools.json"); err == nil {
			t.Fatalf("validateProxyTarget should reject missing required query")
		}
	})
}

func TestValidateProxyTargetRejectsUnexpectedPort(t *testing.T) {
	withProxyConfig(g.Config{
		Port: 8899,
		Network: map[string]g.NetworkMember{
			"10.0.0.1": {Addr: "10.0.0.1"},
		},
	}, func() {
		if _, err := validateProxyTarget("http://10.0.0.1:8080/api/config.json"); err == nil {
			t.Fatalf("validateProxyTarget should reject unexpected port")
		}
	})
}

func proxyTestConfig(t *testing.T, serverURL string) g.Config {
	t.Helper()
	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	port, err := strconv.Atoi(parsedURL.Port())
	if err != nil {
		t.Fatalf("parse test server port: %v", err)
	}
	return g.Config{
		Port: port,
		Base: map[string]int{"Timeout": 5},
		Network: map[string]g.NetworkMember{
			parsedURL.Hostname(): {Addr: parsedURL.Hostname()},
		},
	}
}

func proxyRequest(target string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/proxy.json?g="+url.QueryEscape(target), nil)
	req.RemoteAddr = "127.0.0.1:12345"
	return req
}

func TestHandleProxyUsesValidatedTarget(t *testing.T) {
	var requests atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer remote.Close()

	withProxyConfig(proxyTestConfig(t, remote.URL), func() {
		recorder := httptest.NewRecorder()
		handleProxy(recorder, proxyRequest(remote.URL+"/api/config.json"))

		if recorder.Code != http.StatusOK {
			t.Fatalf("handleProxy status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if requests.Load() != 1 {
			t.Fatalf("remote requests = %d, want 1", requests.Load())
		}
	})
}

func TestHandleProxyRejectsUnknownHost(t *testing.T) {
	var requests atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(`{"status":"unexpected"}`))
	}))
	defer remote.Close()

	cfg := proxyTestConfig(t, remote.URL)
	cfg.Network = map[string]g.NetworkMember{"192.0.2.1": {Addr: "192.0.2.1"}}
	withProxyConfig(cfg, func() {
		recorder := httptest.NewRecorder()
		handleProxy(recorder, proxyRequest(remote.URL+"/api/config.json"))

		if recorder.Code != http.StatusNotAcceptable {
			t.Fatalf("handleProxy status = %d, want %d", recorder.Code, http.StatusNotAcceptable)
		}
		if requests.Load() != 0 {
			t.Fatalf("unknown host received %d requests", requests.Load())
		}
	})
}

func TestHandleProxyDoesNotFollowRedirects(t *testing.T) {
	var redirectedRequests atomic.Int32
	redirected := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectedRequests.Add(1)
		_, _ = w.Write([]byte(`{"status":"unexpected"}`))
	}))
	defer redirected.Close()

	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirected.URL+"/api/config.json", http.StatusFound)
	}))
	defer remote.Close()

	withProxyConfig(proxyTestConfig(t, remote.URL), func() {
		recorder := httptest.NewRecorder()
		handleProxy(recorder, proxyRequest(remote.URL+"/api/config.json"))

		if recorder.Code != http.StatusFound {
			t.Fatalf("handleProxy status = %d, want %d", recorder.Code, http.StatusFound)
		}
		if redirectedRequests.Load() != 0 {
			t.Fatalf("redirect target received %d requests", redirectedRequests.Load())
		}
	})
}

func TestHandleProxyRejectsInvalidCustomTimeout(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"unexpected"}`))
	}))
	defer remote.Close()

	withProxyConfig(proxyTestConfig(t, remote.URL), func() {
		req := proxyRequest(remote.URL + "/api/config.json")
		req.URL.RawQuery += "&t=0"
		recorder := httptest.NewRecorder()
		handleProxy(recorder, req)

		if recorder.Code != http.StatusNotAcceptable {
			t.Fatalf("handleProxy status = %d, want %d", recorder.Code, http.StatusNotAcceptable)
		}
	})
}

func TestHandleProxyStopsAfterRemoteError(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"secret":"must not be appended"}`))
	}))
	defer remote.Close()

	withProxyConfig(proxyTestConfig(t, remote.URL), func() {
		recorder := httptest.NewRecorder()
		handleProxy(recorder, proxyRequest(remote.URL+"/api/config.json"))

		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("handleProxy status = %d, want %d", recorder.Code, http.StatusInternalServerError)
		}
		if strings.Contains(recorder.Body.String(), "must not be appended") {
			t.Fatalf("handleProxy appended remote error body: %q", recorder.Body.String())
		}
	})
}
