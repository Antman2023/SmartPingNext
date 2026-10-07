package g

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
)

func TestConfigMetadataSnapshot(t *testing.T) {
	withGlobalConfigState(t, func() {
		SetConfig(Config{Ver: "test", Port: 8899, Name: "local", Addr: "192.0.2.1", Toollimit: 15, Password: "test-password"})
		want := ConfigMetadata{Ver: "test", Port: 8899, Name: "local", Addr: "192.0.2.1", Toollimit: 15, Password: "test-password"}
		got := ConfigMetadataSnapshot()
		if got != want {
			t.Fatalf("metadata = %#v, want %#v", got, want)
		}
		got.Name = "changed"
		if ConfigMetadataSnapshot() != want {
			t.Fatal("changing snapshot changed active configuration")
		}
		if allocations := testing.AllocsPerRun(100, func() { _ = ConfigMetadataSnapshot() }); allocations != 0 {
			t.Fatalf("metadata allocated %v times, want zero", allocations)
		}
	})
}

func TestCloudModeSnapshot(t *testing.T) {
	withGlobalConfigState(t, func() {
		for _, mode := range []map[string]string{nil, {}, {"Type": "local"}, {"Type": "cloud", "Endpoint": ""}, {"Type": "cloud", "Endpoint": "https://example.test/config?version=1"}} {
			SetConfig(Config{Mode: mode})
			modeType, endpoint := CloudModeSnapshot()
			if modeType != mode["Type"] || endpoint != mode["Endpoint"] {
				t.Fatalf("cloud mode = (%q, %q), want (%q, %q)", modeType, endpoint, mode["Type"], mode["Endpoint"])
			}
			SetConfig(Config{Mode: map[string]string{"Type": "local", "Endpoint": "https://example.test/new"}})
			if modeType != mode["Type"] || endpoint != mode["Endpoint"] {
				t.Fatal("configuration replacement changed the previous snapshot")
			}
		}
	})
}

func TestLocalNetworkSnapshotIsIndependent(t *testing.T) {
	withGlobalConfigState(t, func() {
		const address = "192.0.2.1"
		SetConfig(Config{Addr: address, Network: map[string]NetworkMember{
			address: {Addr: address, Name: "local", Ping: []string{"192.0.2.2"}, Topology: []map[string]string{{"Addr": "192.0.2.2", "Thdloss": "30"}}},
		}})
		before := ConfigSnapshot()
		local, member := LocalNetworkSnapshot()
		if local != address || !reflect.DeepEqual(member, before.Network[address]) {
			t.Fatalf("local snapshot = %q, %#v", local, member)
		}
		member.Ping[0] = "changed"
		member.Topology[0]["Thdloss"] = "changed"
		if !reflect.DeepEqual(ConfigSnapshot(), before) {
			t.Fatal("local snapshot aliases active configuration")
		}
		SetConfig(Config{Addr: address})
		local, member = LocalNetworkSnapshot()
		if local != address || !reflect.DeepEqual(member, NetworkMember{}) {
			t.Fatalf("missing local member = %q, %#v", local, member)
		}
	})
}

func TestNarrowSnapshotsRemainConsistentDuringUpdates(t *testing.T) {
	withGlobalConfigState(t, func() {
		configs := []Config{
			{Addr: "192.0.2.1", Name: "one", Password: "one", Network: map[string]NetworkMember{"192.0.2.1": {Addr: "192.0.2.1", Name: "one"}}},
			{Addr: "192.0.2.2", Name: "two", Password: "two", Network: map[string]NetworkMember{"192.0.2.2": {Addr: "192.0.2.2", Name: "two"}}},
		}
		configs[0].Mode = map[string]string{"Type": "local", "Endpoint": "https://one.test/config"}
		configs[1].Mode = map[string]string{"Type": "cloud", "Endpoint": "https://two.test/config"}
		SetConfig(configs[0])
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				SetConfig(configs[i%2])
			}
		}()
		defer wg.Wait()
		for i := 0; i < 1000; i++ {
			modeType, endpoint := CloudModeSnapshot()
			if !(modeType == configs[0].Mode["Type"] && endpoint == configs[0].Mode["Endpoint"]) &&
				!(modeType == configs[1].Mode["Type"] && endpoint == configs[1].Mode["Endpoint"]) {
				t.Fatalf("mixed cloud mode versions: (%q, %q)", modeType, endpoint)
			}
			metadata := ConfigMetadataSnapshot()
			if metadata.Name != metadata.Password || (metadata.Addr == "192.0.2.1") != (metadata.Name == "one") {
				t.Fatalf("mixed metadata versions: %#v", metadata)
			}
			address, member := LocalNetworkSnapshot()
			if address != member.Addr {
				t.Fatalf("mixed local versions: %q, %#v", address, member)
			}
		}
	})
}

func BenchmarkNarrowConfigSnapshots(b *testing.B) {
	for _, nodes := range []int{1, 100, 1000} {
		b.Run(fmt.Sprintf("%dNodes", nodes), func(b *testing.B) {
			previous := ConfigSnapshot()
			b.Cleanup(func() { SetConfig(previous) })
			config := Config{Addr: "192.0.0.0", Name: "local", Mode: map[string]string{"Type": "cloud", "Endpoint": "https://example.test/config"}, Network: make(map[string]NetworkMember)}
			for i := 0; i < nodes; i++ {
				address := fmt.Sprintf("192.0.%d.%d", i/256, i%256)
				config.Network[address] = NetworkMember{Addr: address, Ping: []string{"192.0.2.1"}, Topology: []map[string]string{{"Addr": "192.0.2.1", "Thdloss": "30"}}}
			}
			SetConfig(config)
			b.Run("FullCloudMode", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					mode := ConfigSnapshot().Mode
					if mode["Type"] != config.Mode["Type"] || mode["Endpoint"] != config.Mode["Endpoint"] {
						b.Fatal("unexpected cloud mode")
					}
				}
			})
			b.Run("CloudMode", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					modeType, endpoint := CloudModeSnapshot()
					if modeType != config.Mode["Type"] || endpoint != config.Mode["Endpoint"] {
						b.Fatal("unexpected cloud mode")
					}
				}
			})
			b.Run("Full", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					_ = ConfigSnapshot()
				}
			})
			b.Run("Metadata", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					_ = ConfigMetadataSnapshot()
				}
			})
			b.Run("LocalNetwork", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					_, _ = LocalNetworkSnapshot()
				}
			})
		})
	}
}
