package g

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestCloudApplyCancelsDuringSnapshot(t *testing.T) {
	for _, scenario := range []struct {
		largeMap   bool
		downloaded bool
	}{{false, false}, {true, false}, {false, true}, {true, true}} {
		t.Run(fmt.Sprintf("largeMap=%v/downloaded=%v", scenario.largeMap, scenario.downloaded), func(t *testing.T) {
			withGlobalConfigState(t, func() {
				const endpoint = "http://example.test/config"
				Root = t.TempDir()
				if err := os.Mkdir(filepath.Join(Root, "conf"), 0755); err != nil {
					t.Fatal(err)
				}
				original := Config{
					Name: "local", Addr: "192.0.0.0", Port: 8899,
					Mode:     map[string]string{"Type": "cloud", "Endpoint": endpoint, "Status": "false"},
					Base:     map[string]int{"Timeout": 5, "Refresh": 1, "Archive": 30},
					Topology: map[string]string{"Tline": "1", "Tsymbolsize": "70"},
					Network:  make(map[string]NetworkMember),
				}
				nodes := 1024
				if scenario.largeMap {
					nodes = 1
					addresses := make([]string, 8192)
					for i := range addresses {
						addresses[i] = "192.0.2.1"
					}
					original.Chinamap = map[string]map[string][]string{"province": {"ctcc": addresses}}
				}
				for i := 0; i < nodes; i++ {
					addr := fmt.Sprintf("192.0.%d.%d", i/256, i%256)
					original.Network[addr] = NetworkMember{Name: addr, Addr: addr}
				}
				if err := ValidateConfig(original); err != nil {
					t.Fatal(err)
				}
				downloaded := cloneConfig(original)
				downloaded.Base["Archive"] = 60
				if scenario.downloaded {
					// Keep the current snapshot small so cancellation is reached
					// only while preparing the larger downloaded configuration.
					original.Network = map[string]NetworkMember{original.Addr: original.Network[original.Addr]}
					original.Chinamap = nil
				}
				if err := ApplyConfig(original); err != nil {
					t.Fatal(err)
				}
				before := ConfigSnapshot()
				filename := filepath.Join(Root, "conf", "config.json")
				diskBefore, err := os.ReadFile(filename)
				if err != nil {
					t.Fatal(err)
				}
				base, cancel := context.WithCancel(context.Background())
				defer cancel()
				ctx := &snapshotCancelOnCheckContext{Context: base, cancel: cancel}
				err = applyCloudConfigContext(ctx, downloaded, endpoint)
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cloud apply did not cancel during snapshot: checks=%d error=%v", ctx.checks, err)
				}
				if !CfgLock.TryLock() {
					t.Fatal("canceled cloud snapshot retained config lock")
				}
				CfgLock.Unlock()
				lockCtx, stop := context.WithTimeout(context.Background(), time.Second)
				defer stop()
				if err := configSaveLock.LockContext(lockCtx); err != nil {
					t.Fatalf("canceled cloud snapshot retained save lock: %v", err)
				}
				configSaveLock.Unlock()
				if !reflect.DeepEqual(before, ConfigSnapshot()) {
					t.Fatal("canceled cloud snapshot published configuration")
				}
				diskAfter, err := os.ReadFile(filename)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(diskBefore, diskAfter) {
					t.Fatal("canceled cloud snapshot changed disk")
				}
				if err := applyCloudConfigContext(context.Background(), downloaded, endpoint); err != nil {
					t.Fatalf("subsequent apply failed: %v", err)
				}
				got := ConfigSnapshot()
				if got.Base["Archive"] != 60 || got.Mode["Status"] != "true" {
					t.Fatal("subsequent apply did not publish configuration")
				}
				data, err := os.ReadFile(filename)
				if err != nil || bytes.Equal(data, diskBefore) {
					t.Fatalf("subsequent apply did not persist the change: %v", err)
				}
			})
		})
	}
}
