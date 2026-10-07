package g

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestUnchangedCloudSyncDoesNotWaitForAuthorizationReaders(t *testing.T) {
	withGlobalConfigState(t, func() {
		const endpoint = "http://example.test/config"
		original := Config{Name: "local", Addr: "192.0.2.1", Port: 8899,
			Base:       map[string]int{"Timeout": 5, "Refresh": 1, "Archive": 30},
			Topology:   map[string]string{"Tline": "1", "Tsymbolsize": "70"},
			Mode:       map[string]string{"Type": "cloud", "Endpoint": endpoint, "Status": "false", "LastSuccTime": "old", "extension": "keep"},
			Authiplist: "192.0.2.2",
			Network:    map[string]NetworkMember{"192.0.2.1": {Name: "local", Addr: "192.0.2.1"}},
		}
		rule := map[string]string{"Addr": "192.0.2.2", "Name": "remote", "Thdchecksec": "60", "Thdoccnum": "1", "Thdloss": "30", "Thdavgdelay": "200"}
		local := original.Network[original.Addr]
		local.Topology = []map[string]string{rule}
		original.Network[original.Addr] = local
		original.Network["192.0.2.2"] = NetworkMember{Name: "remote", Addr: "192.0.2.2"}
		SetConfig(original)
		episode := RecordAlertCheckEpisode(original.Addr, rule, false)
		if episode == nil {
			t.Fatal("could not establish an active alert episode")
		}
		before := ConfigSnapshot()
		Root = filepath.Join(t.TempDir(), "missing")
		AuthIpLock.RLock()
		done := make(chan error, 1)
		finished := false
		defer func() {
			AuthIpLock.RUnlock()
			if !finished {
				<-done
			}
		}()
		go func() { done <- applyCloudConfigContext(context.Background(), original, endpoint) }()
		select {
		case err := <-done:
			finished = true
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("unchanged cloud sync waited for authorization readers")
		}
		after := ConfigSnapshot()
		if after.Mode["Status"] != "true" || after.Mode["LastSuccTime"] == "old" {
			t.Fatal("sync status not refreshed")
		}
		if !cloudConfigEqual(before, after) || !AuthUserIpMap["192.0.2.2"] || !AuthAgentIpMap["192.0.2.1"] {
			t.Fatal("sync changed persistent configuration or authorization")
		}
		AlertStatusLock.RLock()
		sameEpisode := alertEpisodes["192.0.2.2"] == episode && !AlertStatus["192.0.2.2"]
		AlertStatusLock.RUnlock()
		if !sameEpisode {
			t.Fatal("unchanged sync reset the active alert episode")
		}
	})
}

func TestCloudSuccessRejectsStaleOrCanceledUpdate(t *testing.T) {
	withGlobalConfigState(t, func() {
		for _, mode := range []map[string]string{
			{"Type": "cloud", "Endpoint": "new", "Status": "false"},
			{"Type": "local", "Endpoint": "old", "Status": "false"},
		} {
			SetConfig(Config{Mode: mode})
			before := ConfigSnapshot()
			if err := markCloudSyncSuccessContext(context.Background(), "old", "now"); err == nil {
				t.Fatal("stale update accepted")
			}
			if !reflect.DeepEqual(before, ConfigSnapshot()) {
				t.Fatal("stale update changed mode")
			}
		}
		SetConfig(Config{Mode: map[string]string{"Type": "cloud", "Endpoint": "old", "Status": "false"}})
		before := ConfigSnapshot()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := markCloudSyncSuccessContext(ctx, "old", "now"); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled update=%v", err)
		}
		if !reflect.DeepEqual(before, ConfigSnapshot()) {
			t.Fatal("canceled update changed mode")
		}
	})
}

func BenchmarkCloudSuccessPublication(b *testing.B) {
	previous := ConfigSnapshot()
	b.Cleanup(func() { SetConfig(previous) })
	for _, nodes := range []int{1, 128, 1024} {
		config := snapshotContextFixture(nodes, 4, nodes*8)
		config.Mode = map[string]string{"Type": "cloud", "Endpoint": "endpoint", "Status": "true", "LastSuccTime": "now"}
		SetConfig(config)
		b.Run(fmt.Sprintf("%dNodes/FullPublication", nodes), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				SetConfig(config)
			}
		})
		b.Run(fmt.Sprintf("%dNodes/StatusOnly", nodes), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if err := markCloudSyncSuccessContext(context.Background(), "endpoint", "now"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
