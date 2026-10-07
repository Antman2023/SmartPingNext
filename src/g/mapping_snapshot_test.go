package g

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestMappingRoundSnapshotIsolationAndEmptyShapes(t *testing.T) {
	withGlobalConfigState(t, func() {
		for _, targets := range []map[string]map[string][]string{nil, {}, {"nil": nil, "empty": {}, "province": {"nil": nil, "empty": {}, "ctcc": {"192.0.2.1", "192.0.2.1"}}}} {
			SetConfig(Config{Base: map[string]int{"MappingConcurrency": 0, "PingCount": 20}, Chinamap: targets})
			before := ConfigSnapshot()
			for _, active := range []bool{false, true} {
				ctx := context.Background()
				if active {
					var cancel context.CancelFunc
					ctx, cancel = context.WithCancel(ctx)
					defer cancel()
				}
				got, err := MappingRoundSnapshotContext(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got.Chinamap, before.Chinamap) || !reflect.DeepEqual(got.Base, map[string]int{"MappingConcurrency": 0}) {
					t.Fatalf("snapshot differs from legacy map or settings: %#v", got)
				}
				got.Base["MappingConcurrency"] = 64
				if got.Chinamap["province"] != nil {
					got.Chinamap["province"]["ctcc"][0] = "changed"
					got.Chinamap["province"]["new"] = []string{"changed"}
					delete(got.Chinamap, "empty")
				}
				if !reflect.DeepEqual(before, ConfigSnapshot()) {
					t.Fatal("snapshot aliases source")
				}
			}
		}
	})
}

func TestMappingRoundSnapshotCancellation(t *testing.T) {
	withGlobalConfigState(t, func() {
		SetConfig(snapshotContextFixture(1, 0, 8192))
		base, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctx := &snapshotCancelOnCheckContext{Context: base, cancel: cancel}
		got, err := MappingRoundSnapshotContext(ctx)
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, MappingRoundConfig{}) {
			t.Fatalf("canceled copy = %#v, %v", got, err)
		}
		if !CfgLock.TryLock() {
			t.Fatal("cancellation retained read lock")
		}
		CfgLock.Unlock()
		if _, err := MappingRoundSnapshotContext(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
}

func TestMappingRoundSnapshotOneVersion(t *testing.T) {
	withGlobalConfigState(t, func() {
		versions := []Config{
			{Base: map[string]int{"MappingProbeCount": 1}, Chinamap: map[string]map[string][]string{"one": {"ctcc": {"192.0.2.1"}}}},
			{Base: map[string]int{"MappingProbeCount": 2}, Chinamap: map[string]map[string][]string{"two": {"cucc": {"192.0.2.2"}}}},
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
			got, err := MappingRoundSnapshotContext(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			version := got.Base["MappingProbeCount"] - 1
			if version < 0 || version >= len(versions) || !reflect.DeepEqual(got.Chinamap, versions[version].Chinamap) {
				t.Fatalf("mixed version: %#v", got)
			}
		}
	})
}

func BenchmarkMappingRoundConfigSnapshot(b *testing.B) {
	previous := ConfigSnapshot()
	b.Cleanup(func() { SetConfig(previous) })
	for _, nodes := range []int{16, 128, 1024} {
		config := snapshotContextFixture(nodes, 4, 8192)
		config.Base = map[string]int{"MappingConcurrency": 8, "MappingProbeCount": 3}
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
		b.Run(fmt.Sprintf("%dNodes/MappingOnly", nodes), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := MappingRoundSnapshotContext(ctx); err != nil {
					b.Fatal(err)
				}
			}
		})
		cancel()
	}
}
