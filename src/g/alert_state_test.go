package g

import (
	"sync"
	"testing"
)

func alertStateConfig() Config {
	return Config{Addr: "127.0.0.1", Network: map[string]NetworkMember{
		"127.0.0.1": {Addr: "127.0.0.1", Topology: []map[string]string{{
			"Addr": "192.0.2.1", "Name": "target", "Thdchecksec": "600",
			"Thdloss": "30", "Thdavgdelay": "200", "Thdoccnum": "2",
		}}},
	}}
}

func TestRecordAlertCheckRejectsObsoleteConfig(t *testing.T) {
	changes := map[string]func(*Config){
		"removed rule": func(c *Config) {
			member := c.Network[c.Addr]
			member.Topology = nil
			c.Network[c.Addr] = member
		},
		"changed local node": func(c *Config) { c.Addr = "127.0.0.2" },
	}
	for _, key := range []string{"Thdchecksec", "Thdloss", "Thdavgdelay", "Thdoccnum"} {
		changes[key] = func(c *Config) { c.Network[c.Addr].Topology[0][key] = "1" }
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			withGlobalConfigState(t, func() {
				initial := alertStateConfig()
				SetConfig(initial)
				rule := initial.Network[initial.Addr].Topology[0]
				next := cloneConfig(initial)
				change(&next)
				SetConfig(next)
				for _, healthy := range []bool{false, true} {
					if RecordAlertCheck(initial.Addr, rule, healthy) {
						t.Error("obsolete check started a new alert")
					}
					if _, exists := AlertStatus[rule["Addr"]]; exists {
						t.Error("obsolete check recreated cleared alert state")
					}
				}
			})
		})
	}
}

func TestRecordAlertCheckTransitionsAndUnchangedRules(t *testing.T) {
	withGlobalConfigState(t, func() {
		config := alertStateConfig()
		SetConfig(config)
		rule := config.Network[config.Addr].Topology[0]
		for _, step := range []struct{ healthy, wantAlert bool }{
			{true, false}, {false, true}, {false, false}, {true, false}, {false, true},
		} {
			if got := RecordAlertCheck(config.Addr, rule, step.healthy); got != step.wantAlert {
				t.Fatalf("healthy=%v: new alert=%v, want %v", step.healthy, got, step.wantAlert)
			}
		}
		next := cloneConfig(config)
		next.Name = "renamed local node"
		next.Network[next.Addr].Topology[0]["Name"] = "renamed target"
		SetConfig(next)
		if RecordAlertCheck(config.Addr, rule, false) {
			t.Fatal("rename restarted an existing alert episode")
		}
		if RecordAlertCheck(config.Addr, rule, true) || !AlertStatus[rule["Addr"]] {
			t.Fatal("unchanged rule must still accept recovery after rename")
		}
	})
}

func TestRecordAlertCheckDoesNotRecoverNewRuleWithOldResult(t *testing.T) {
	withGlobalConfigState(t, func() {
		initial := alertStateConfig()
		SetConfig(initial)
		oldRule := initial.Network[initial.Addr].Topology[0]
		next := cloneConfig(initial)
		newRule := next.Network[next.Addr].Topology[0]
		newRule["Thdloss"] = "10"
		SetConfig(next)
		if !RecordAlertCheck(next.Addr, newRule, false) {
			t.Fatal("new rule must start its own alert episode")
		}
		RecordAlertCheck(initial.Addr, oldRule, true)
		if AlertStatus[newRule["Addr"]] {
			t.Fatal("old healthy result recovered the new rule's alert")
		}
		if RecordAlertCheck(next.Addr, newRule, false) {
			t.Fatal("old result caused a duplicate alert for the new rule")
		}
	})
}

func TestRecordAlertCheckConcurrentConfigChanges(t *testing.T) {
	withGlobalConfigState(t, func() {
		initial := alertStateConfig()
		SetConfig(initial)
		rule := initial.Network[initial.Addr].Topology[0]
		removed := Config{Addr: initial.Addr}
		var workers sync.WaitGroup
		workers.Add(2)
		go func() {
			defer workers.Done()
			for i := 0; i < 1000; i++ {
				SetConfig(initial)
				SetConfig(removed)
			}
		}()
		go func() {
			defer workers.Done()
			for i := 0; i < 1000; i++ {
				RecordAlertCheck(initial.Addr, rule, i%2 == 0)
			}
		}()
		workers.Wait()
		if len(AlertStatus) != 0 {
			t.Fatalf("removed rule left stale alert state: %v", AlertStatus)
		}
	})
}

func TestRetryAlertEpisodeOnlyResetsItsOwnEpisode(t *testing.T) {
	changes := map[string]func(Config, map[string]string){
		"changed rule": func(config Config, rule map[string]string) {
			rule["Thdloss"] = "10"
			SetConfig(config)
		},
		"removed and readded identical rule": func(config Config, _ map[string]string) {
			SetConfig(Config{Addr: config.Addr})
			SetConfig(config)
		},
		"recovered and failed again": func(config Config, rule map[string]string) {
			RecordAlertCheck(config.Addr, rule, true)
		},
		"changed local node and back": func(config Config, _ map[string]string) {
			SetConfig(Config{Addr: "127.0.0.2"})
			SetConfig(config)
		},
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			withGlobalConfigState(t, func() {
				config := alertStateConfig()
				SetConfig(config)
				rule := config.Network[config.Addr].Topology[0]
				oldEpisode := RecordAlertCheckEpisode(config.Addr, rule, false)
				if oldEpisode == nil {
					t.Fatal("old episode did not start")
				}
				change(config, rule)
				newEpisode := RecordAlertCheckEpisode(config.Addr, rule, false)
				if newEpisode == nil || newEpisode == oldEpisode {
					t.Fatal("replacement episode did not get a new identity")
				}
				RetryAlertEpisode(oldEpisode)
				if AlertStatus[rule["Addr"]] || RecordAlertCheck(config.Addr, rule, false) {
					t.Fatal("obsolete retry reset the newer alert")
				}
				RetryAlertEpisode(newEpisode)
				if !AlertStatus[rule["Addr"]] {
					t.Fatal("current episode was not made eligible for retry")
				}
				if !RecordAlertCheck(config.Addr, rule, false) {
					t.Fatal("current episode could not retry")
				}
				RetryAlertEpisode(newEpisode)
				if AlertStatus[rule["Addr"]] {
					t.Fatal("reusing a retry token reset another episode")
				}
			})
		})
	}
}

func TestRetryAlertEpisodeSurvivesUnchangedRuleConfigUpdates(t *testing.T) {
	withGlobalConfigState(t, func() {
		config := alertStateConfig()
		SetConfig(config)
		rule := config.Network[config.Addr].Topology[0]
		episode := RecordAlertCheckEpisode(config.Addr, rule, false)
		config.Name = "renamed local node"
		rule["Name"] = "renamed target"
		SetConfig(config)
		RetryAlertEpisode(episode)
		if !RecordAlertCheck(config.Addr, rule, false) {
			t.Fatal("renaming discarded the current episode's retry")
		}
		RetryAlertEpisode(nil)
	})
}

func TestRetryAlertEpisodeConcurrentConfigChanges(t *testing.T) {
	withGlobalConfigState(t, func() {
		config := alertStateConfig()
		SetConfig(config)
		rule := config.Network[config.Addr].Topology[0]
		episode := RecordAlertCheckEpisode(config.Addr, rule, false)
		var workers sync.WaitGroup
		workers.Add(2)
		go func() {
			defer workers.Done()
			for i := 0; i < 1000; i++ {
				SetConfig(Config{Addr: config.Addr})
				SetConfig(config)
				RecordAlertCheckEpisode(config.Addr, rule, false)
			}
		}()
		go func() {
			defer workers.Done()
			for i := 0; i < 1000; i++ {
				RetryAlertEpisode(episode)
			}
		}()
		workers.Wait()
		if RecordAlertCheck(config.Addr, rule, false) || AlertStatus[rule["Addr"]] {
			t.Fatal("obsolete retry reset the final config's episode")
		}
	})
}
