package g

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

type saveCancelOnCheckContext struct {
	context.Context
	cancel   context.CancelFunc
	checks   int
	cancelAt int
}

func (ctx *saveCancelOnCheckContext) Err() error {
	ctx.checks++
	if ctx.checks == ctx.cancelAt {
		ctx.cancel()
	}
	return ctx.Context.Err()
}

func TestSaveConfigFileCancellationPreservesFile(t *testing.T) {
	withGlobalConfigState(t, func() {
		Root = t.TempDir()
		conf := filepath.Join(Root, "conf")
		if err := os.Mkdir(conf, 0755); err != nil {
			t.Fatal(err)
		}
		original := snapshotContextFixture(1, 1, 1)
		if err := ApplyConfig(original); err != nil {
			t.Fatal(err)
		}
		before := ConfigSnapshot()
		filename := filepath.Join(conf, "config.json")
		diskBefore, err := os.ReadFile(filename)
		if err != nil {
			t.Fatal(err)
		}
		candidate := snapshotContextFixture(1024, 4, 8192)
		candidate.Name = "replacement"
		for _, stage := range []struct {
			name  string
			check int
		}{{"before serialization", 1}, {"after serialization", 2}} {
			t.Run(stage.name, func(t *testing.T) {
				base, cancel := context.WithCancel(context.Background())
				defer cancel()
				ctx := &saveCancelOnCheckContext{Context: base, cancel: cancel, cancelAt: stage.check}
				if err := saveConfigFileContext(ctx, candidate); !errors.Is(err, context.Canceled) {
					t.Fatalf("save error = %v, want canceled", err)
				}
				diskAfter, err := os.ReadFile(filename)
				if err != nil || !bytes.Equal(diskBefore, diskAfter) {
					t.Fatalf("canceled save changed file: %v", err)
				}
				if !reflect.DeepEqual(before, ConfigSnapshot()) {
					t.Fatal("canceled save changed runtime config")
				}
				entries, err := os.ReadDir(conf)
				if err != nil || len(entries) != 1 {
					t.Fatalf("canceled save left temporary files: %v", err)
				}
			})
		}
		if err := saveConfigFileContext(context.Background(), candidate); err != nil {
			t.Fatalf("retry failed: %v", err)
		}
		data, err := os.ReadFile(filename)
		if err != nil {
			t.Fatal(err)
		}
		var saved Config
		if err := json.Unmarshal(data, &saved); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(candidate, saved) {
			t.Fatal("retry did not persist candidate")
		}
	})
}

func TestConfigPublicationCompletesAfterPersistedCancellation(t *testing.T) {
	withGlobalConfigState(t, func() {
		Root = t.TempDir()
		if err := os.Mkdir(filepath.Join(Root, "conf"), 0755); err != nil {
			t.Fatal(err)
		}
		original := snapshotContextFixture(1, 1, 1)
		if err := ApplyConfig(original); err != nil {
			t.Fatal(err)
		}
		candidate := snapshotContextFixture(2, 1, 2)
		candidate.Name = "persisted"
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// Publication acquires CfgLock before AuthIpLock, after saving the file.
		// Hold authorization readers to pause precisely after the disk commit.
		AuthIpLock.RLock()
		locked := true
		finished := make(chan error, 1)
		go func() {
			configSaveLock.Lock()
			defer configSaveLock.Unlock()
			finished <- applyConfigLockedContext(ctx, candidate)
		}()
		completed := false
		defer func() {
			if locked {
				AuthIpLock.RUnlock()
			}
			if !completed {
				<-finished
			}
		}()
		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for CfgLock.TryRLock() {
			CfgLock.RUnlock()
			select {
			case err := <-finished:
				completed = true
				t.Fatalf("publication finished before commit barrier: %v", err)
			case <-timer.C:
				t.Fatal("publication did not reach commit barrier")
			case <-ticker.C:
			}
		}
		data, err := os.ReadFile(filepath.Join(Root, "conf", "config.json"))
		if err != nil {
			t.Fatal(err)
		}
		var saved Config
		if err := json.Unmarshal(data, &saved); err != nil {
			t.Fatal(err)
		}
		if saved.Name != candidate.Name {
			t.Fatal("publication started before persistence")
		}
		cancel()
		AuthIpLock.RUnlock()
		locked = false
		err = <-finished
		completed = true
		if err != nil {
			t.Fatalf("committed update returned error: %v", err)
		}
		if !reflect.DeepEqual(saved, ConfigSnapshot()) {
			t.Fatal("late cancellation left disk and runtime inconsistent")
		}
	})
}
