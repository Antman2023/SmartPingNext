package g

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func cloneConfig(c Config) Config {
	out := c
	if c.Mode != nil {
		out.Mode = make(map[string]string, len(c.Mode))
		for k, v := range c.Mode {
			out.Mode[k] = v
		}
	}
	if c.Base != nil {
		out.Base = make(map[string]int, len(c.Base))
		for k, v := range c.Base {
			out.Base[k] = v
		}
	}
	if c.Topology != nil {
		out.Topology = make(map[string]string, len(c.Topology))
		for k, v := range c.Topology {
			out.Topology[k] = v
		}
	}
	if c.Network != nil {
		out.Network = make(map[string]NetworkMember, len(c.Network))
		for k, v := range c.Network {
			out.Network[k] = v
		}
	}
	if c.Chinamap != nil {
		out.Chinamap = make(map[string]map[string][]string, len(c.Chinamap))
		for k, v := range c.Chinamap {
			out.Chinamap[k] = v
		}
	}
	return out
}

func cloneBoolMapForConfig(in map[string]bool) map[string]bool {
	if in == nil {
		return nil
	}
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func withGlobalConfigState(t *testing.T, fn func()) {
	t.Helper()
	oldRoot := Root
	oldCfg := cloneConfig(Cfg)
	oldSelf := SelfCfg
	oldUser := cloneBoolMapForConfig(AuthUserIpMap)
	oldAgent := cloneBoolMapForConfig(AuthAgentIpMap)
	oldClient := HttpClient
	oldTZ := LocalTimezone

	defer func() {
		Root = oldRoot
		Cfg = oldCfg
		SelfCfg = oldSelf
		AuthUserIpMap = oldUser
		AuthAgentIpMap = oldAgent
		HttpClient = oldClient
		LocalTimezone = oldTZ
	}()

	fn()
}

func TestIsExist(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "a.txt")

	if IsExist(fp) {
		t.Fatalf("IsExist should return false for missing file")
	}

	if err := os.WriteFile(fp, []byte("ok"), 0644); err != nil {
		t.Fatalf("write temp file failed: %v", err)
	}

	if !IsExist(fp) {
		t.Fatalf("IsExist should return true for existing file")
	}
}

func TestGetBaseInt(t *testing.T) {
	withGlobalConfigState(t, func() {
		Cfg = Config{
			Base: map[string]int{
				"PingCount": 15,
				"Bad":       0,
			},
		}
		if got := GetBaseInt("PingCount", 20); got != 15 {
			t.Fatalf("GetBaseInt existing key = %d, want 15", got)
		}
		if got := GetBaseInt("NotExist", 20); got != 20 {
			t.Fatalf("GetBaseInt missing key = %d, want 20", got)
		}
		if got := GetBaseInt("Bad", 20); got != 20 {
			t.Fatalf("GetBaseInt non-positive value = %d, want 20", got)
		}
	})
}

func TestSaveAuth(t *testing.T) {
	withGlobalConfigState(t, func() {
		Cfg = Config{
			Authiplist: " 127.0.0.1, ::1 ",
			Network: map[string]NetworkMember{
				"127.0.0.1": {Addr: "127.0.0.1"},
				"::1":       {Addr: "::1"},
			},
		}

		saveAuth()

		if Cfg.Authiplist != "127.0.0.1,::1" {
			t.Fatalf("saveAuth normalized Authiplist = %q", Cfg.Authiplist)
		}
		if !AuthUserIpMap["127.0.0.1"] || !AuthUserIpMap["::1"] {
			t.Fatalf("saveAuth did not populate AuthUserIpMap correctly: %#v", AuthUserIpMap)
		}
		if !AuthAgentIpMap["127.0.0.1"] || !AuthAgentIpMap["::1"] {
			t.Fatalf("saveAuth did not populate AuthAgentIpMap correctly: %#v", AuthAgentIpMap)
		}
	})
}

func TestSaveCloudConfigSuccess(t *testing.T) {
	withGlobalConfigState(t, func() {
		respCfg := Config{
			Name: "cloud-name",
			Addr: "8.8.8.8",
			Mode: map[string]string{
				"Type": "cloud",
			},
			Network: map[string]NetworkMember{
				"127.0.0.1": {Name: "local", Addr: "127.0.0.1"},
				"8.8.8.8":   {Name: "cloud", Addr: "8.8.8.8"},
			},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(respCfg)
		}))
		defer srv.Close()

		Cfg = Config{
			Name:     "local-name",
			Addr:     "127.0.0.1",
			Ver:      "vtest",
			Port:     8899,
			Password: "pwd",
			Mode: map[string]string{
				"Endpoint": srv.URL,
				"Type":     "local",
			},
			Network: map[string]NetworkMember{
				"127.0.0.1": {Name: "local", Addr: "127.0.0.1"},
			},
		}
		HttpClient = srv.Client()

		got, err := SaveCloudConfig(srv.URL)
		if err != nil {
			t.Fatalf("SaveCloudConfig returned error: %v", err)
		}

		if got.Name != "cloud-name" {
			t.Fatalf("SaveCloudConfig returned config.Name = %q, want cloud-name", got.Name)
		}
		if Cfg.Name != "local-name" || Cfg.Addr != "127.0.0.1" {
			t.Fatalf("local identity should be preserved, got Name=%q Addr=%q", Cfg.Name, Cfg.Addr)
		}
		if Cfg.Ver != "vtest" || Cfg.Port != 8899 || Cfg.Password != "pwd" {
			t.Fatalf("local runtime fields should be preserved, got Ver=%q Port=%d Password=%q", Cfg.Ver, Cfg.Port, Cfg.Password)
		}
		if Cfg.Mode["Type"] != "cloud" || Cfg.Mode["Status"] != "true" || Cfg.Mode["Endpoint"] != srv.URL {
			t.Fatalf("cloud mode fields not set as expected: %#v", Cfg.Mode)
		}
		if _, err := time.Parse("2006-01-02 15:04:05", Cfg.Mode["LastSuccTime"]); err != nil {
			t.Fatalf("LastSuccTime format invalid: %q", Cfg.Mode["LastSuccTime"])
		}
		if SelfCfg.Addr != "127.0.0.1" {
			t.Fatalf("SelfCfg should point to local addr, got %q", SelfCfg.Addr)
		}
	})
}

func TestSaveCloudConfigInvalidJSON(t *testing.T) {
	withGlobalConfigState(t, func() {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("not-json"))
		}))
		defer srv.Close()

		Cfg = Config{
			Name: "local-name",
			Addr: "127.0.0.1",
			Mode: map[string]string{"Endpoint": srv.URL},
			Network: map[string]NetworkMember{
				"127.0.0.1": {Addr: "127.0.0.1"},
			},
		}
		HttpClient = srv.Client()

		got, err := SaveCloudConfig(srv.URL)
		if err == nil {
			t.Fatalf("SaveCloudConfig should fail on invalid json")
		}
		if got.Name != "not-json" {
			t.Fatalf("on invalid json, returned config.Name = %q, want raw body", got.Name)
		}
	})
}

func TestSaveConfig(t *testing.T) {
	withGlobalConfigState(t, func() {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "conf"), 0755); err != nil {
			t.Fatalf("create conf dir failed: %v", err)
		}

		Root = root
		Cfg = Config{
			Name:       "node-1",
			Addr:       "127.0.0.1",
			Authiplist: "127.0.0.1",
			Mode:       map[string]string{},
			Network: map[string]NetworkMember{
				"127.0.0.1": {Addr: "127.0.0.1"},
			},
		}

		if err := SaveConfig(); err != nil {
			t.Fatalf("SaveConfig returned error: %v", err)
		}

		content, err := os.ReadFile(filepath.Join(root, "conf", "config.json"))
		if err != nil {
			t.Fatalf("read saved config failed: %v", err)
		}
		if len(content) == 0 {
			t.Fatalf("saved config file should not be empty")
		}

		var saved Config
		if err := json.Unmarshal(content, &saved); err != nil {
			t.Fatalf("saved config is not valid json: %v", err)
		}
		if saved.Name != "node-1" {
			t.Fatalf("saved config Name = %q, want node-1", saved.Name)
		}
	})
}
