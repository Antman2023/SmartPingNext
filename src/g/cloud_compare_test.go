package g

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

// Retain the previous implementation as a semantic and benchmark reference.
func cloudConfigEqualDeepCopy(left, right Config) bool {
	left = normalizeConfig(cloneConfig(left))
	right = normalizeConfig(cloneConfig(right))
	for _, config := range []*Config{&left, &right} {
		delete(config.Mode, "LastSuccTime")
		delete(config.Mode, "Status")
	}
	return reflect.DeepEqual(left, right)
}

func TestCloudConfigComparisonNormalizationAndInputPreservation(t *testing.T) {
	for _, scenario := range []struct {
		name        string
		left, right Config
		equal       bool
	}{
		{"mode nil empty", Config{}, Config{Mode: map[string]string{}}, true},
		{"runtime only", Config{Mode: map[string]string{"Status": "false"}}, Config{Mode: map[string]string{"Status": "true", "LastSuccTime": "now"}}, true},
		{"mode extension", Config{}, Config{Mode: map[string]string{"extension": ""}}, false},
		{"base nil empty", Config{}, Config{Base: map[string]int{}}, false},
		{"topology nil empty", Config{}, Config{Topology: map[string]string{}}, false},
		{"network nil empty", Config{}, Config{Network: map[string]NetworkMember{}}, false},
		{"member nil empty lists", Config{Network: map[string]NetworkMember{"a": {}}}, Config{Network: map[string]NetworkMember{"a": {Ping: []string{}, Topology: []map[string]string{}}}}, true},
		{"rule nil empty", Config{Network: map[string]NetworkMember{"a": {Topology: []map[string]string{nil}}}}, Config{Network: map[string]NetworkMember{"a": {Topology: []map[string]string{{}}}}}, false},
		{"mapping nil empty", Config{}, Config{Chinamap: map[string]map[string][]string{}}, true},
		{"provider nil empty", Config{Chinamap: map[string]map[string][]string{"p": nil}}, Config{Chinamap: map[string]map[string][]string{"p": {}}}, true},
		{"addresses nil empty", Config{Chinamap: map[string]map[string][]string{"p": {"ctcc": nil}}}, Config{Chinamap: map[string]map[string][]string{"p": {"ctcc": {}}}}, false},
		{"auth normalized", Config{Authiplist: " , ::ffff:192.0.2.1,2001:0DB8::1,, "}, Config{Authiplist: "192.0.2.1,2001:db8::1"}, true},
		{"auth order", Config{Authiplist: "192.0.2.1,192.0.2.2"}, Config{Authiplist: "192.0.2.2,192.0.2.1"}, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			before, err := json.Marshal([]Config{scenario.left, scenario.right})
			if err != nil {
				t.Fatal(err)
			}
			for _, compare := range []func(Config, Config) bool{cloudConfigEqualDeepCopy, cloudConfigEqual} {
				if got := compare(scenario.left, scenario.right); got != scenario.equal {
					t.Fatalf("comparison = %v, want %v", got, scenario.equal)
				}
				if got := compare(scenario.right, scenario.left); got != scenario.equal {
					t.Fatalf("reverse comparison = %v, want %v", got, scenario.equal)
				}
			}
			after, err := json.Marshal([]Config{scenario.left, scenario.right})
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatal("comparison mutated its inputs")
			}
		})
	}
}

func TestCloudConfigComparisonDetectsPersistedChanges(t *testing.T) {
	base := snapshotContextFixture(3, 2, 4)
	for name, change := range map[string]func(*Config){
		"identity": func(c *Config) { c.Name += "new" },
		"password": func(c *Config) { c.Password += "new" },
		"port":     func(c *Config) { c.Port++ },
		"base":     func(c *Config) { c.Base["Timeout"]++ },
		"mode":     func(c *Config) { c.Mode["Endpoint"] = "https://example.test" },
		"topology": func(c *Config) { c.Topology["Tline"] = "3" },
		"node":     func(c *Config) { m := c.Network[c.Addr]; m.Smartping = !m.Smartping; c.Network[c.Addr] = m },
		"ping":     func(c *Config) { c.Network[c.Addr].Ping[0] = "192.0.2.2" },
		"rule":     func(c *Config) { c.Network[c.Addr].Topology[0]["Thdloss"] = "99" },
		"map":      func(c *Config) { c.Chinamap["江苏"]["ctcc"][0] = "192.0.2.2" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := cloneConfig(base)
			if !cloudConfigEqual(base, candidate) {
				t.Fatal("equal independent copies differ")
			}
			change(&candidate)
			before, _ := json.Marshal([]Config{base, candidate})
			if cloudConfigEqualDeepCopy(base, candidate) || cloudConfigEqual(base, candidate) {
				t.Fatal("persisted change was ignored")
			}
			after, _ := json.Marshal([]Config{base, candidate})
			if string(before) != string(after) {
				t.Fatal("comparison mutated nested data")
			}
		})
	}
}

func BenchmarkCloudConfigComparison(b *testing.B) {
	for _, nodes := range []int{1, 128, 1024} {
		b.Run(fmt.Sprintf("%dNodes", nodes), func(b *testing.B) {
			left := snapshotContextFixture(nodes, 4, nodes*8)
			right := cloneConfig(left)
			right.Mode["Status"] = "true"
			right.Mode["LastSuccTime"] = "2026-10-07 10:00:00"
			for _, scenario := range []struct {
				name    string
				compare func(Config, Config) bool
			}{
				{"DeepCopy", cloudConfigEqualDeepCopy}, {"SelectiveCopy", cloudConfigEqual},
			} {
				b.Run(scenario.name, func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						if !scenario.compare(left, right) {
							b.Fatal("equivalent cloud configuration differs")
						}
					}
				})
			}
		})
	}
}
