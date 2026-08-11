package g

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
	oldSelf := cloneNetworkMember(SelfCfg)
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

func TestSetConfigUpdatesAuth(t *testing.T) {
	withGlobalConfigState(t, func() {
		SetConfig(Config{
			Authiplist: " 127.0.0.1, ::1, 2001:0db8:0:0:0:0:0:1 ",
			Network: map[string]NetworkMember{
				"127.0.0.1": {Addr: "127.0.0.1"},
				"::1":       {Addr: "::1"},
			},
		})

		if Cfg.Authiplist != "127.0.0.1,::1,2001:db8::1" {
			t.Fatalf("SetConfig normalized Authiplist = %q", Cfg.Authiplist)
		}
		if !AuthUserIpMap["127.0.0.1"] || !AuthUserIpMap["::1"] {
			t.Fatalf("SetConfig did not populate AuthUserIpMap correctly: %#v", AuthUserIpMap)
		}
		if !AuthUserIpMap["2001:db8::1"] {
			t.Fatalf("SetConfig did not normalize IPv6 auth address: %#v", AuthUserIpMap)
		}
		if !AuthAgentIpMap["127.0.0.1"] || !AuthAgentIpMap["::1"] {
			t.Fatalf("SetConfig did not populate AuthAgentIpMap correctly: %#v", AuthAgentIpMap)
		}
	})
}

func TestSetConfigNormalizesOptionalCollections(t *testing.T) {
	withGlobalConfigState(t, func() {
		input := Config{
			Addr: "127.0.0.1",
			Network: map[string]NetworkMember{
				"127.0.0.1": {Name: "local", Addr: "127.0.0.1"},
			},
		}
		SetConfig(input)

		inputMember := input.Network["127.0.0.1"]
		if inputMember.Ping != nil || inputMember.Topology != nil {
			t.Fatal("SetConfig mutated caller-owned network members")
		}

		snapshot := ConfigSnapshot()
		if snapshot.Mode == nil {
			t.Fatal("SetConfig left Mode nil")
		}
		if snapshot.Chinamap == nil {
			t.Fatal("SetConfig left Chinamap nil")
		}
		member := snapshot.Network["127.0.0.1"]
		if member.Ping == nil {
			t.Fatal("SetConfig left NetworkMember.Ping nil")
		}
		if member.Topology == nil {
			t.Fatal("SetConfig left NetworkMember.Topology nil")
		}
	})
}

func TestConfigStateDoesNotShareMutableReferences(t *testing.T) {
	withGlobalConfigState(t, func() {
		input := Config{
			Addr:     "127.0.0.1",
			Mode:     map[string]string{"Type": "local"},
			Base:     map[string]int{"Timeout": 5},
			Topology: map[string]string{"Tline": "1"},
			Network: map[string]NetworkMember{
				"127.0.0.1": {
					Addr:     "127.0.0.1",
					Ping:     []string{"192.0.2.1"},
					Topology: []map[string]string{{"Addr": "192.0.2.1"}},
				},
			},
			Chinamap: map[string]map[string][]string{
				"ctcc": {"江苏": {"192.0.2.2"}},
			},
		}
		SetConfig(input)

		input.Base["Timeout"] = 30
		input.Network["127.0.0.1"].Ping[0] = "198.51.100.1"
		input.Chinamap["ctcc"]["江苏"][0] = "198.51.100.2"
		if Cfg.Base["Timeout"] != 5 || Cfg.Network["127.0.0.1"].Ping[0] != "192.0.2.1" || Cfg.Chinamap["ctcc"]["江苏"][0] != "192.0.2.2" {
			t.Fatalf("SetConfig retained references to caller-owned data: %#v", Cfg)
		}

		snapshot := ConfigSnapshot()
		snapshot.Mode["Type"] = "cloud"
		snapshot.Network["127.0.0.1"].Topology[0]["Addr"] = "203.0.113.1"
		snapshot.Chinamap["ctcc"]["江苏"][0] = "203.0.113.2"
		if Cfg.Mode["Type"] != "local" || Cfg.Network["127.0.0.1"].Topology[0]["Addr"] != "192.0.2.1" || Cfg.Chinamap["ctcc"]["江苏"][0] != "192.0.2.2" {
			t.Fatalf("ConfigSnapshot exposed mutable global state: %#v", Cfg)
		}
	})
}

func TestCloneConfigPreservesEmptySlices(t *testing.T) {
	config := Config{
		Network: map[string]NetworkMember{
			"127.0.0.1": {Ping: []string{}, Topology: []map[string]string{}},
		},
		Chinamap: map[string]map[string][]string{
			"ctcc": {"江苏": []string{}},
		},
	}
	cloned := cloneConfig(config)
	member := cloned.Network["127.0.0.1"]
	if member.Ping == nil || member.Topology == nil || cloned.Chinamap["ctcc"]["江苏"] == nil {
		t.Fatalf("cloneConfig changed empty slices to nil: %#v", cloned)
	}
}

func TestSaveCloudConfigSuccess(t *testing.T) {
	withGlobalConfigState(t, func() {
		Root = t.TempDir()
		if err := os.MkdirAll(filepath.Join(Root, "conf"), 0755); err != nil {
			t.Fatalf("create conf dir failed: %v", err)
		}
		respCfg := Config{
			Name: "cloud-name",
			Addr: "8.8.8.8",
			Base: map[string]int{
				"Timeout": 5,
				"Refresh": 1,
				"Archive": 30,
			},
			Topology: map[string]string{
				"Tline":       "1",
				"Tsymbolsize": "70",
			},
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
				"Type":     "cloud",
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

func TestSaveCloudConfigSkipsWriteWhenOnlyRuntimeStatusChanges(t *testing.T) {
	withGlobalConfigState(t, func() {
		downloaded := Config{
			Base: map[string]int{
				"Timeout": 5,
				"Refresh": 1,
				"Archive": 30,
			},
			Topology: map[string]string{
				"Tline":       "1",
				"Tsymbolsize": "70",
			},
			Mode: map[string]string{"Type": "cloud"},
			Network: map[string]NetworkMember{
				"127.0.0.1": {Name: "local", Addr: "127.0.0.1"},
			},
		}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(downloaded)
		}))
		defer srv.Close()

		current := cloneConfig(downloaded)
		current.Name = "local"
		current.Addr = "127.0.0.1"
		current.Ver = "vtest"
		current.Port = 8899
		current.Password = "pwd"
		current.Mode = map[string]string{
			"Type":         "cloud",
			"Endpoint":     srv.URL,
			"Status":       "false",
			"LastSuccTime": "2026-01-01 00:00:00",
		}
		SetConfig(current)
		Root = filepath.Join(t.TempDir(), "missing")
		HttpClient = srv.Client()

		if _, err := SaveCloudConfig(srv.URL); err != nil {
			t.Fatalf("unchanged cloud config should not require a writable config directory: %v", err)
		}
		got := ConfigSnapshot()
		if got.Mode["Status"] != "true" || got.Mode["LastSuccTime"] == current.Mode["LastSuccTime"] {
			t.Fatalf("runtime cloud status was not refreshed: %#v", got.Mode)
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

func TestSaveCloudConfigRejectsInvalidConfig(t *testing.T) {
	withGlobalConfigState(t, func() {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(Config{
				Name:    "invalid-cloud",
				Addr:    "127.0.0.1",
				Network: map[string]NetworkMember{},
			})
		}))
		defer srv.Close()

		Cfg = Config{
			Name:     "local-name",
			Addr:     "127.0.0.1",
			Port:     8899,
			Password: "pwd",
			Mode:     map[string]string{"Endpoint": srv.URL, "Type": "cloud"},
			Network: map[string]NetworkMember{
				"127.0.0.1": {Name: "local", Addr: "127.0.0.1"},
			},
		}
		HttpClient = srv.Client()

		if _, err := SaveCloudConfig(srv.URL); err == nil {
			t.Fatalf("SaveCloudConfig should reject invalid cloud config")
		}
		if Cfg.Name != "local-name" || Cfg.Addr != "127.0.0.1" {
			t.Fatalf("invalid cloud config replaced current identity: %#v", Cfg)
		}
	})
}

func TestReadCloudConfigBodyRejectsOversizedBody(t *testing.T) {
	body := bytes.NewReader(make([]byte, maxCloudConfigBytes+1))
	if _, err := readCloudConfigBody(body); err == nil {
		t.Fatalf("readCloudConfigBody should reject oversized body")
	}
}

func TestSaveCloudConfigRejectsNonOKStatus(t *testing.T) {
	withGlobalConfigState(t, func() {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		}))
		defer srv.Close()
		HttpClient = srv.Client()

		if _, err := SaveCloudConfig(srv.URL); err == nil {
			t.Fatalf("SaveCloudConfig should reject non-200 response")
		}
	})
}

func TestSaveCloudConfigDoesNotOverwriteNewerLocalConfig(t *testing.T) {
	withGlobalConfigState(t, func() {
		started := make(chan struct{})
		release := make(chan struct{})
		respCfg := Config{
			Base: map[string]int{"Timeout": 5, "Refresh": 1, "Archive": 30},
			Topology: map[string]string{
				"Tline":       "1",
				"Tsymbolsize": "70",
			},
			Mode: map[string]string{"Type": "cloud"},
			Network: map[string]NetworkMember{
				"127.0.0.1": {Name: "local", Addr: "127.0.0.1"},
			},
		}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(started)
			<-release
			_ = json.NewEncoder(w).Encode(respCfg)
		}))
		defer srv.Close()

		Root = t.TempDir()
		if err := os.MkdirAll(filepath.Join(Root, "conf"), 0755); err != nil {
			t.Fatalf("create conf dir failed: %v", err)
		}
		SetConfig(Config{
			Name: "cloud-runtime",
			Addr: "127.0.0.1",
			Mode: map[string]string{"Type": "cloud", "Endpoint": srv.URL},
			Network: map[string]NetworkMember{
				"127.0.0.1": {Name: "cloud-runtime", Addr: "127.0.0.1"},
			},
		})
		HttpClient = srv.Client()

		errCh := make(chan error, 1)
		go func() {
			_, err := SaveCloudConfig(srv.URL)
			errCh <- err
		}()
		<-started
		SetConfig(Config{
			Name: "new-local",
			Addr: "127.0.0.1",
			Mode: map[string]string{"Type": "local"},
			Network: map[string]NetworkMember{
				"127.0.0.1": {Name: "new-local", Addr: "127.0.0.1"},
			},
		})
		close(release)

		if err := <-errCh; err == nil {
			t.Fatalf("stale cloud request should be rejected")
		}
		got := ConfigSnapshot()
		if got.Name != "new-local" || got.Mode["Type"] != "local" {
			t.Fatalf("stale cloud request overwrote newer config: %#v", got)
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

		Cfg.Name = "node-2"
		if err := SaveConfig(); err != nil {
			t.Fatalf("second SaveConfig returned error: %v", err)
		}
		content, err = os.ReadFile(filepath.Join(root, "conf", "config.json"))
		if err != nil {
			t.Fatalf("read replaced config failed: %v", err)
		}
		if err := json.Unmarshal(content, &saved); err != nil {
			t.Fatalf("replaced config is not valid json: %v", err)
		}
		if saved.Name != "node-2" {
			t.Fatalf("replaced config Name = %q, want node-2", saved.Name)
		}

		tempFiles, err := filepath.Glob(filepath.Join(root, "conf", ".config.json.tmp-*"))
		if err != nil {
			t.Fatalf("list temporary files failed: %v", err)
		}
		if len(tempFiles) != 0 {
			t.Fatalf("temporary config files were not cleaned up: %v", tempFiles)
		}
	})
}

func TestApplyConfigWriteFailureDoesNotPublish(t *testing.T) {
	withGlobalConfigState(t, func() {
		original := Config{
			Name:       "original",
			Addr:       "127.0.0.1",
			Authiplist: "127.0.0.1",
			Network: map[string]NetworkMember{
				"127.0.0.1": {Name: "original", Addr: "127.0.0.1"},
			},
		}
		SetConfig(original)
		Root = filepath.Join(t.TempDir(), "missing")

		candidate := cloneConfig(original)
		candidate.Name = "candidate"
		if err := ApplyConfig(candidate); err == nil {
			t.Fatalf("ApplyConfig should fail when the config directory is missing")
		}

		if got := ConfigSnapshot().Name; got != "original" {
			t.Fatalf("failed ApplyConfig published Name = %q, want original", got)
		}
		if !AuthUserIpMap["127.0.0.1"] {
			t.Fatalf("failed ApplyConfig changed authorization state: %#v", AuthUserIpMap)
		}
	})
}

func TestApplyConfigPersistsNormalizedConfig(t *testing.T) {
	withGlobalConfigState(t, func() {
		Root = t.TempDir()
		if err := os.MkdirAll(filepath.Join(Root, "conf"), 0755); err != nil {
			t.Fatalf("create conf dir failed: %v", err)
		}

		candidate := Config{
			Name:       "node",
			Addr:       "127.0.0.1",
			Authiplist: " 127.0.0.1, 2001:0db8:0:0:0:0:0:1 ",
			Network: map[string]NetworkMember{
				"127.0.0.1": {Name: "node", Addr: "127.0.0.1"},
			},
		}
		if err := ApplyConfig(candidate); err != nil {
			t.Fatalf("ApplyConfig returned error: %v", err)
		}

		content, err := os.ReadFile(filepath.Join(Root, "conf", "config.json"))
		if err != nil {
			t.Fatalf("read applied config failed: %v", err)
		}
		var saved Config
		if err := json.Unmarshal(content, &saved); err != nil {
			t.Fatalf("decode applied config failed: %v", err)
		}
		const wantAuth = "127.0.0.1,2001:db8::1"
		if saved.Authiplist != wantAuth || ConfigSnapshot().Authiplist != wantAuth {
			t.Fatalf("persisted/runtime auth lists differ: saved=%q runtime=%q", saved.Authiplist, ConfigSnapshot().Authiplist)
		}
	})
}
