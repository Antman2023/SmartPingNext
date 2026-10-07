package g

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestPingRoundSnapshotTargetsAndIsolation(t *testing.T) {
	withGlobalConfigState(t, func() {
		config := Config{Addr: "local", Base: map[string]int{
			"PingCount": 3, "PingIntervalMs": 500, "PingStaggerMs": 0, "Archive": 30,
		}, Network: map[string]NetworkMember{
			"local": {Addr: "local", Ping: []string{"peer", "missing", "blank", "peer", "local"}},
			"peer":  {Addr: "192.0.2.1"}, "blank": {Addr: " "},
		}}
		SetConfig(config)
		before := ConfigSnapshot()
		got, err := PingRoundSnapshotContext(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		want := PingRoundConfig{
			Base:    map[string]int{"PingCount": 3, "PingIntervalMs": 500, "PingStaggerMs": 0},
			Targets: []PingTarget{{"peer", "192.0.2.1"}, {"missing", ""}, {"blank", " "}, {"peer", "192.0.2.1"}, {"local", "local"}},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("snapshot = %#v, want %#v", got, want)
		}
		got.Base["PingCount"] = 99
		got.Targets[0].Addr = "changed"
		if !reflect.DeepEqual(before, ConfigSnapshot()) {
			t.Fatal("snapshot aliases source configuration")
		}
		SetConfig(Config{})
		if !reflect.DeepEqual(got.Targets[1:], want.Targets[1:]) {
			t.Fatal("replacement mutated existing snapshot")
		}
		got, err = PingRoundSnapshotContext(context.Background())
		if err != nil || len(got.Targets) != 0 || len(got.Base) != 0 {
			t.Fatalf("empty config snapshot = %#v, %v", got, err)
		}
	})
}

func TestPingRoundSnapshotCancellationReturnsNoPartialData(t *testing.T) {
	withGlobalConfigState(t, func() {
		config := Config{Addr: "local", Network: map[string]NetworkMember{
			"local": {Ping: make([]string, 1024)},
		}}
		SetConfig(config)
		base, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctx := &snapshotCancelOnCheckContext{Context: base, cancel: cancel}
		got, err := PingRoundSnapshotContext(ctx)
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, PingRoundConfig{}) {
			t.Fatalf("canceled snapshot = %#v, %v", got, err)
		}
		if !CfgLock.TryLock() {
			t.Fatal("snapshot retained config lock")
		}
		CfgLock.Unlock()
	})
}

func TestPingRoundSnapshotUsesOneConfigurationVersion(t *testing.T) {
	withGlobalConfigState(t, func() {
		versions := []Config{
			{Addr: "a", Base: map[string]int{"PingCount": 1}, Network: map[string]NetworkMember{"a": {Ping: []string{"one"}}, "one": {Addr: "192.0.2.1"}}},
			{Addr: "b", Base: map[string]int{"PingCount": 2}, Network: map[string]NetworkMember{"b": {Ping: []string{"two"}}, "two": {Addr: "192.0.2.2"}}},
		}
		SetConfig(versions[0])
		done := make(chan struct{})
		go func() {
			defer close(done)
			for i := 0; i < 200; i++ {
				SetConfig(versions[i%2])
			}
		}()
		defer func() { <-done }()
		for i := 0; i < 200; i++ {
			got, err := PingRoundSnapshotContext(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			want := PingTarget{"one", "192.0.2.1"}
			if got.Base["PingCount"] == 2 {
				want = PingTarget{"two", "192.0.2.2"}
			}
			if !reflect.DeepEqual(got.Targets, []PingTarget{want}) {
				t.Fatalf("mixed snapshot: %#v", got)
			}
		}
	})
}

func BenchmarkPingRoundSnapshot(b *testing.B) {
	previous := ConfigSnapshot()
	b.Cleanup(func() { SetConfig(previous) })
	for _, nodes := range []int{16, 128, 1024} {
		config := snapshotContextFixture(nodes, 4, 8192)
		config.Base = map[string]int{"PingCount": 20, "PingIntervalMs": 3000, "PingTimeoutMs": 3000, "PingStaggerMs": 100}
		local := config.Network[config.Addr]
		local.Ping = nil
		for i := 0; i < 16; i++ {
			local.Ping = append(local.Ping, fmt.Sprintf("192.0.0.%d", i))
		}
		config.Network[config.Addr] = local
		SetConfig(config)
		ctx, cancel := context.WithCancel(context.Background())
		b.Run(fmt.Sprintf("%dNodes/Full", nodes), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := ConfigSnapshotContext(ctx); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(fmt.Sprintf("%dNodes/PingOnly", nodes), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := PingRoundSnapshotContext(ctx); err != nil {
					b.Fatal(err)
				}
			}
		})
		cancel()
	}
}
