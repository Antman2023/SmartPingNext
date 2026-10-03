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
			config := Config{Addr: "192.0.0.0", Name: "local", Network: make(map[string]NetworkMember)}
			for i := 0; i < nodes; i++ {
				address := fmt.Sprintf("192.0.%d.%d", i/256, i%256)
				config.Network[address] = NetworkMember{Addr: address, Ping: []string{"192.0.2.1"}, Topology: []map[string]string{{"Addr": "192.0.2.1", "Thdloss": "30"}}}
			}
			SetConfig(config)
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
