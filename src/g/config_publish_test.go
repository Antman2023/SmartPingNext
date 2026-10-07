package g

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestApplyConfigRetainsPrivateNestedData(t *testing.T) {
	withGlobalConfigState(t, func() {
		Root = t.TempDir()
		if err := os.Mkdir(filepath.Join(Root, "conf"), 0755); err != nil {
			t.Fatal(err)
		}
		candidate := snapshotContextFixture(3, 2, 4)
		inputBefore, _ := json.Marshal(candidate)
		if err := ApplyConfig(candidate); err != nil {
			t.Fatal(err)
		}
		inputAfter, _ := json.Marshal(candidate)
		if string(inputAfter) != string(inputBefore) {
			t.Fatal("applying config mutated caller data")
		}
		want := ConfigSnapshot()
		candidate.Mode["Type"] = "changed"
		candidate.Base["Timeout"] = 99
		candidate.Topology["Tline"] = "99"
		candidate.Network[candidate.Addr].Ping[0] = "changed"
		candidate.Network[candidate.Addr].Topology[0]["Thdloss"] = "99"
		candidate.Chinamap["江苏"]["ctcc"][0] = "changed"
		delete(candidate.Network, candidate.Addr)
		delete(candidate.Chinamap, "江苏")
		if !reflect.DeepEqual(want, ConfigSnapshot()) {
			t.Fatal("published configuration aliases caller data")
		}
		CfgLock.Lock()
		selfMatches := reflect.DeepEqual(SelfCfg, want.Network[want.Addr])
		SelfCfg.Ping[0] = "self-only"
		SelfCfg.Topology[0]["Thdloss"] = "self-only"
		CfgLock.Unlock()
		if !selfMatches || !reflect.DeepEqual(want, ConfigSnapshot()) {
			t.Fatal("local member copy is not independent")
		}
		data, err := os.ReadFile(filepath.Join(Root, "conf", "config.json"))
		if err != nil {
			t.Fatal(err)
		}
		var saved Config
		if err := json.Unmarshal(data, &saved); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(want, saved) {
			t.Fatal("saved config differs from published private snapshot")
		}
	})
}

func applyConfigWithRepeatedCopy(config Config) error {
	configSaveLock.Lock()
	defer configSaveLock.Unlock()
	config = normalizeConfig(cloneConfig(config))
	if err := saveConfigFile(config); err != nil {
		return err
	}
	SetConfig(config)
	return nil
}

func BenchmarkApplyConfigPublication(b *testing.B) {
	previous, previousRoot := ConfigSnapshot(), Root
	b.Cleanup(func() { SetConfig(previous); Root = previousRoot })
	Root = b.TempDir()
	if err := os.Mkdir(filepath.Join(Root, "conf"), 0755); err != nil {
		b.Fatal(err)
	}
	for _, nodes := range []int{1, 128, 1024} {
		config := snapshotContextFixture(nodes, 4, nodes*8)
		for _, scenario := range []struct {
			name  string
			apply func(Config) error
		}{
			{"RepeatedCopy", applyConfigWithRepeatedCopy}, {"PreparedCopy", ApplyConfig},
		} {
			b.Run(fmt.Sprintf("%dNodes/%s", nodes, scenario.name), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					if err := scenario.apply(config); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
