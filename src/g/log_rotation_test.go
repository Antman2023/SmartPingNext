package g

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestLogRotationRetainsNewestFilesAndResumesAfterRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "info.log")
	writer, err := openRotatingLogFile(path, 8, 2)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { writer.Close() })
	for _, line := range []string{"one\n", "two\n", "tri\n", "for\n", "fiv\n", "six\n", "sev\n"} {
		if _, err := writer.Write([]byte(line)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	writer, err = openRotatingLogFile(path, 8, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"egt\n", "nin\n"} {
		if _, err := writer.Write([]byte(line)); err != nil {
			t.Fatal(err)
		}
	}
	for suffix, want := range map[string]string{"": "nin\n", ".1": "sev\negt\n", ".2": "fiv\nsix\n"} {
		got, err := os.ReadFile(path + suffix)
		if err != nil || string(got) != want {
			t.Fatalf("file %s: got %q, error %v, want %q", suffix, got, err, want)
		}
	}
	files, err := filepath.Glob(path + "*")
	if err != nil || len(files) != 3 {
		t.Fatalf("rotation retained unexpected files: %v, %v", files, err)
	}
}

func TestLogRotationPreservesOversizedEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "debug.log")
	large := strings.Repeat("x", 30) + "\n"
	if err := os.WriteFile(path, []byte(large), 0600); err != nil {
		t.Fatal(err)
	}
	writer, err := openRotatingLogFile(path, 8, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	if _, err := writer.Write([]byte(large)); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("next\n")); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{".1", ".2"} {
		got, err := os.ReadFile(path + suffix)
		if err != nil || string(got) != large {
			t.Fatalf("oversized entry lost: %q, %v", got, err)
		}
	}
}

func TestLogRotationRecoversAfterFilesystemFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "error.log")
	writer, err := openRotatingLogFile(path, 4, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	if _, err := writer.Write([]byte("old\n")); err != nil {
		t.Fatal(err)
	}
	// A non-empty directory at the oldest backup prevents removing that path.
	if err := os.Mkdir(path+".2", 0700); err != nil {
		t.Fatal(err)
	}
	blocker := filepath.Join(path+".2", "blocker")
	if err := os.WriteFile(blocker, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if n, err := writer.Write([]byte("new\n")); err == nil || n != 0 {
		t.Fatalf("failed rotation should report failure: n=%d, err=%v", n, err)
	}
	if err := os.Remove(blocker); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path + ".2"); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("new\n")); err != nil {
		t.Fatalf("rotation did not recover: %v", err)
	}
	got, err := os.ReadFile(path + ".1")
	if err != nil || string(got) != "old\n" {
		t.Fatalf("original log was lost: %q, %v", got, err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("late\n")); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("write after close: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLogRotationConcurrentWritesPreserveCompleteEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "info.log")
	writer, err := openRotatingLogFile(path, 64, 32)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for entry := 0; entry < 20; entry++ {
				if _, err := fmt.Fprintf(writer, "%d:%02d\n", worker, entry); err != nil {
					t.Error(err)
				}
			}
		}(worker)
	}
	wg.Wait()
	files, err := filepath.Glob(path + "*")
	if err != nil {
		t.Fatal(err)
	}
	entries := make(map[string]int)
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if len(data) > 64 {
			t.Fatalf("file exceeded size limit: %s, %d bytes", file, len(data))
		}
		for _, entry := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
			entries[entry]++
		}
	}
	if len(entries) != 160 {
		t.Fatalf("got %d unique entries, want 160", len(entries))
	}
	for worker := 0; worker < 8; worker++ {
		for entry := 0; entry < 20; entry++ {
			key := fmt.Sprintf("%d:%02d", worker, entry)
			if entries[key] != 1 {
				t.Fatalf("entry %s count = %d, want 1", key, entries[key])
			}
		}
	}
}
