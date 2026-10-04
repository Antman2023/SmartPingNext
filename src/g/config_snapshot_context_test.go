package g

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

func snapshotContextFixture(nodes, rules, addresses int) Config {
	config := Config{Ver: "test", Port: 8899, Name: "local", Addr: "192.0.0.0", Password: "private",
		Mode: map[string]string{"Type": "local"}, Base: map[string]int{"Timeout": 3},
		Topology: map[string]string{"Tline": "2"}, Network: make(map[string]NetworkMember),
		Chinamap: map[string]map[string][]string{"江苏": {"ctcc": {}, "cucc": {}, "cmcc": {}}}}
	for i := 0; i < nodes; i++ {
		addr := fmt.Sprintf("192.0.%d.%d", i/256, i%256)
		member := NetworkMember{Name: addr, Addr: addr, Smartping: true, Ping: []string{"192.0.2.1"}, Topology: make([]map[string]string, rules)}
		for j := range member.Topology {
			member.Topology[j] = map[string]string{"Addr": "192.0.2.1", "Thdloss": "30", "Thdchecksec": "900"}
		}
		config.Network[addr] = member
	}
	for i := 0; i < addresses; i++ {
		config.Chinamap["江苏"]["ctcc"] = append(config.Chinamap["江苏"]["ctcc"], "192.0.2.1")
	}
	return config
}

func TestConfigSnapshotContextRejectsFinishedRequests(t *testing.T) {
	withGlobalConfigState(t, func() {
		SetConfig(snapshotContextFixture(1024, 1, 8192))
		for _, expired := range []bool{false, true} {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if expired {
				ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			}
			got, err := ConfigSnapshotContext(ctx)
			cancel()
			if !errors.Is(err, ctx.Err()) || !reflect.DeepEqual(got, Config{}) {
				t.Fatalf("returned canceled configuration: error=%v, nodes=%d", err, len(got.Network))
			}
		}
	})
}

func TestConfigSnapshotContextSkipsLockWhenAlreadyCanceled(t *testing.T) {
	withGlobalConfigState(t, func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		CfgLock.Lock()
		finished := make(chan error, 1)
		go func() { _, err := ConfigSnapshotContext(ctx); finished <- err }()
		completed := false
		defer func() {
			CfgLock.Unlock()
			if !completed {
				<-finished
			}
		}()
		select {
		case err := <-finished:
			completed = true
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("locked canceled snapshot error=%v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("already canceled snapshot waited for the configuration lock")
		}
	})
}

type snapshotCancelOnCheckContext struct {
	context.Context
	cancel context.CancelFunc
	checks int
}

func (ctx *snapshotCancelOnCheckContext) Err() error {
	ctx.checks++
	if ctx.checks == 40 {
		ctx.cancel()
	}
	return ctx.Context.Err()
}

func TestConfigSnapshotContextDiscardsCopiesAndReleasesLock(t *testing.T) {
	withGlobalConfigState(t, func() {
		for _, fixture := range []Config{snapshotContextFixture(1024, 1, 0), snapshotContextFixture(1, 1024, 0), snapshotContextFixture(0, 0, 8192)} {
			SetConfig(fixture)
			before := ConfigSnapshot()
			base, cancel := context.WithCancel(context.Background())
			ctx := &snapshotCancelOnCheckContext{Context: base, cancel: cancel}
			got, err := ConfigSnapshotContext(ctx)
			cancel()
			if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, Config{}) {
				t.Fatalf("partial snapshot returned: checks=%d, nodes=%d, error=%v", ctx.checks, len(got.Network), err)
			}
			if !CfgLock.TryLock() {
				t.Fatal("canceled copy retained its read lock")
			}
			CfgLock.Unlock()
			active, err := ConfigSnapshotContext(context.Background())
			if err != nil || !reflect.DeepEqual(active, before) {
				t.Fatal("canceled copy changed configuration or prevented recovery")
			}
		}
	})
}

func TestConfigSnapshotContextCopiesStayIndependentDuringUpdates(t *testing.T) {
	withGlobalConfigState(t, func() {
		one, two := snapshotContextFixture(3, 2, 4), snapshotContextFixture(7, 3, 8)
		one.Name, one.Password = "one", "one"
		two.Name, two.Password = "two", "two"
		SetConfig(one)
		wantOne := ConfigSnapshot()
		SetConfig(two)
		wantTwo := ConfigSnapshot()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var updates sync.WaitGroup
		updates.Add(1)
		go func() {
			defer updates.Done()
			for i := 0; i < 200; i++ {
				if i%2 == 0 {
					SetConfig(one)
				} else {
					SetConfig(two)
				}
			}
		}()
		defer updates.Wait()
		for i := 0; i < 200; i++ {
			got, err := ConfigSnapshotContext(ctx)
			if err != nil || (!reflect.DeepEqual(got, wantOne) && !reflect.DeepEqual(got, wantTwo)) {
				t.Fatal("mixed configuration versions in snapshot")
			}
			got.Mode["Type"] = "changed"
			got.Base["Timeout"] = 60
			got.Topology["Tline"] = "changed"
			member := got.Network["192.0.0.0"]
			member.Ping[0] = "changed"
			member.Topology[0]["Addr"] = "changed"
			got.Chinamap["江苏"]["ctcc"][0] = "changed"
		}
	})
}

func TestConfigSnapshotContextPreservesLegacyCollectionShapes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, config := range []Config{
		{},
		{Mode: map[string]string{}, Base: map[string]int{}, Topology: map[string]string{}, Network: map[string]NetworkMember{}, Chinamap: map[string]map[string][]string{}},
		{Network: map[string]NetworkMember{"nil": {}, "empty": {Ping: []string{}, Topology: []map[string]string{}}, "rules": {Topology: []map[string]string{nil, {}, {"custom": "value"}}}},
			Chinamap: map[string]map[string][]string{"nil": nil, "empty": {}, "addresses": {"nil": nil, "empty": {}}}},
		snapshotContextFixture(3, 2, 4),
	} {
		got, err := cloneConfigContext(ctx, config)
		if err != nil || !reflect.DeepEqual(got, cloneConfig(config)) {
			t.Fatalf("collection shapes differ from the legacy snapshot: got=%#v, error=%v", got, err)
		}
	}
}

func BenchmarkConfigSnapshotContext(b *testing.B) {
	config := snapshotContextFixture(1024, 1, 8192)
	active, activeCancel := context.WithCancel(context.Background())
	defer activeCancel()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, mode := range []string{"Legacy", "Background", "Active", "Canceled"} {
		b.Run(mode, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var got Config
				var err error
				switch mode {
				case "Legacy":
					got = cloneConfig(config)
				case "Background":
					got, err = cloneConfigContext(context.Background(), config)
				case "Active":
					got, err = cloneConfigContext(active, config)
				case "Canceled":
					got, err = cloneConfigContext(canceled, config)
				}
				if mode == "Canceled" {
					if !errors.Is(err, context.Canceled) || got.Network != nil {
						b.Fatal("canceled copy returned configuration")
					}
				} else if err != nil || len(got.Network) != 1024 || len(got.Chinamap["江苏"]["ctcc"]) != 8192 {
					b.Fatal("snapshot copy failed")
				}
			}
		})
	}
}
