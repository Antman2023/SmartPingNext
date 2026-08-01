package http

import (
	"encoding/json"
	"testing"

	"smartping/src/g"
	"smartping/src/static"
)

func validTestConfig() g.Config {
	return g.Config{
		Port: 8899,
		Name: "local",
		Addr: "127.0.0.1",
		Mode: map[string]string{"Type": "local"},
		Base: map[string]int{
			"Timeout": 5,
			"Refresh": 1,
			"Archive": 30,
		},
		Topology: map[string]string{
			"Tline":       "1",
			"Tsymbolsize": "70",
		},
		Network: map[string]g.NetworkMember{
			"127.0.0.1": {Name: "local", Addr: "127.0.0.1"},
		},
	}
}

func TestValidateConfigAcceptsValidConfig(t *testing.T) {
	if err := validateConfig(validTestConfig()); err != nil {
		t.Fatalf("validateConfig returned error: %v", err)
	}
}

func TestValidateConfigAcceptsEmbeddedDefaultConfig(t *testing.T) {
	data, err := static.Files.ReadFile("conf/config-base.json")
	if err != nil {
		t.Fatalf("read embedded config: %v", err)
	}
	var config g.Config
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("decode embedded config: %v", err)
	}
	if err := validateConfig(config); err != nil {
		t.Fatalf("embedded default config should be valid: %v", err)
	}
}

func TestValidateConfigRejectsInvalidReferencesAndLimits(t *testing.T) {
	tests := map[string]func(*g.Config){
		"non numeric topology size": func(config *g.Config) {
			config.Topology["Tsymbolsize"] = "large"
		},
		"non finite topology size": func(config *g.Config) {
			config.Topology["Tsymbolsize"] = "NaN"
		},
		"address with whitespace": func(config *g.Config) {
			config.Addr = " 127.0.0.1"
		},
		"network key mismatch": func(config *g.Config) {
			config.Network["127.0.0.1"] = g.NetworkMember{Name: "local", Addr: "127.0.0.2"}
		},
		"missing self node": func(config *g.Config) {
			config.Addr = "127.0.0.2"
		},
		"missing ping target": func(config *g.Config) {
			member := config.Network["127.0.0.1"]
			member.Ping = []string{"192.0.2.1"}
			config.Network["127.0.0.1"] = member
		},
		"invalid auth IP": func(config *g.Config) {
			config.Authiplist = "127.0.0.1,invalid"
		},
		"excessive ping count": func(config *g.Config) {
			config.Base["PingCount"] = 1000000
		},
		"invalid cloud endpoint": func(config *g.Config) {
			config.Mode = map[string]string{"Type": "cloud", "Endpoint": "file:///etc/passwd"}
		},
		"zero port": func(config *g.Config) {
			config.Port = 0
		},
		"port above maximum": func(config *g.Config) {
			config.Port = 65536
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			config := validTestConfig()
			mutate(&config)
			if err := validateConfig(config); err == nil {
				t.Fatalf("validateConfig should reject %s", name)
			}
		})
	}
}

func TestValidateConfigAcceptsValidTopologyRule(t *testing.T) {
	config := validTestConfig()
	config.Network["192.0.2.1"] = g.NetworkMember{Name: "remote", Addr: "192.0.2.1"}
	member := config.Network["127.0.0.1"]
	member.Ping = []string{"192.0.2.1"}
	member.Topology = []map[string]string{{
		"Name":        "remote",
		"Addr":        "192.0.2.1",
		"Thdchecksec": "900",
		"Thdloss":     "30",
		"Thdavgdelay": "200",
		"Thdoccnum":   "3",
	}}
	config.Network["127.0.0.1"] = member

	if err := validateConfig(config); err != nil {
		t.Fatalf("validateConfig returned error: %v", err)
	}
}

func TestValidateConfigRejectsNonMinuteTopologyWindow(t *testing.T) {
	config := validTestConfig()
	config.Network["192.0.2.1"] = g.NetworkMember{Name: "remote", Addr: "192.0.2.1"}
	member := config.Network["127.0.0.1"]
	member.Topology = []map[string]string{{
		"Name":        "remote",
		"Addr":        "192.0.2.1",
		"Thdchecksec": "61",
		"Thdloss":     "30",
		"Thdavgdelay": "200",
		"Thdoccnum":   "3",
	}}
	config.Network["127.0.0.1"] = member

	if err := validateConfig(config); err == nil {
		t.Fatalf("validateConfig should reject a topology window that is not a multiple of 60 seconds")
	}
}
