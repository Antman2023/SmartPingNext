package g

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestLoggerFailedReinitializationPreservesActiveFiles(t *testing.T) {
	logger := logrus.StandardLogger()
	oldOutput, oldFormatter := logger.Out, logger.Formatter
	oldLevel, oldReportCaller := logger.Level, logger.ReportCaller
	defer func() {
		_ = CloseLogger()
		logrus.SetOutput(oldOutput)
		logrus.SetFormatter(oldFormatter)
		logrus.SetLevel(oldLevel)
		logrus.SetReportCaller(oldReportCaller)
	}()
	t.Setenv("SMARTPING_LOG_LEVEL", "info")
	root := t.TempDir()
	if err := InitLogger(root); err != nil {
		t.Fatal(err)
	}
	logrus.SetOutput(io.Discard)
	for _, blockedFile := range []string{"debug.log", "error.log"} {
		t.Run(blockedFile, func(t *testing.T) {
			failedRoot := t.TempDir()
			if err := os.MkdirAll(filepath.Join(failedRoot, "logs", blockedFile), 0755); err != nil {
				t.Fatal(err)
			}
			if err := InitLogger(failedRoot); err == nil {
				t.Fatal("expected initialization failure")
			}
			logrus.Info("retained-after-" + blockedFile)
			assertLogFileContains(t, filepath.Join(root, "logs", "info.log"), "retained-after-"+blockedFile, "unexpected-record")
			// On Windows, leaked open handles prevent this directory rename.
			if err := os.Rename(filepath.Join(failedRoot, "logs"), filepath.Join(failedRoot, "closed-logs")); err != nil {
				t.Fatalf("partially opened log files were not released: %v", err)
			}
		})
	}
}

func TestManagedLogHookConcurrentReplacementRetainsEveryRecord(t *testing.T) {
	root := t.TempDir()
	hook := &managedLogHook{}
	t.Cleanup(func() { _ = hook.close() })
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{DisableTimestamp: true})
	var paths []string
	install := func(index int) {
		t.Helper()
		path := filepath.Join(root, fmt.Sprintf("%d.log", index))
		file, err := openLogFile(path)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
		if err := hook.replace(map[logrus.Level]*rotatingLogFile{logrus.InfoLevel: file}); err != nil {
			t.Fatal(err)
		}
	}
	install(0)
	const workers, records = 8, 100
	var wg sync.WaitGroup
	start := make(chan struct{})
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			<-start
			for record := 0; record < records; record++ {
				entry := logrus.NewEntry(logger)
				entry.Level = logrus.InfoLevel
				entry.Message = fmt.Sprintf("worker-%d-record-%d", worker, record)
				if err := hook.Fire(entry); err != nil {
					t.Errorf("write during replacement: %v", err)
				}
			}
		}(worker)
	}
	close(start)
	for index := 1; index <= 16; index++ {
		install(index)
	}
	wg.Wait()
	if err := hook.close(); err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(content)), "\n") {
			if line == "" {
				continue
			}
			if seen[line] {
				t.Fatalf("duplicate log record: %s", line)
			}
			seen[line] = true
		}
	}
	if len(seen) != workers*records {
		t.Fatalf("got %d records, want %d", len(seen), workers*records)
	}
}

func TestLoggerRoutesLevelsAndClosesFilesAcrossReinitialization(t *testing.T) {
	logger := logrus.StandardLogger()
	oldOutput := logger.Out
	oldFormatter := logger.Formatter
	oldLevel := logger.Level
	oldReportCaller := logger.ReportCaller
	defer func() {
		_ = CloseLogger()
		logrus.SetOutput(oldOutput)
		logrus.SetFormatter(oldFormatter)
		logrus.SetLevel(oldLevel)
		logrus.SetReportCaller(oldReportCaller)
	}()
	_ = CloseLogger()
	t.Setenv("SMARTPING_LOG_LEVEL", "debug")

	firstRoot := t.TempDir()
	if err := InitLogger(firstRoot); err != nil {
		t.Fatalf("first InitLogger returned error: %v", err)
	}
	logrus.SetOutput(io.Discard)
	logrus.Info("first-info-message")
	logrus.Debug("first-debug-message")
	logrus.Warn("first-warning-message")

	secondRoot := t.TempDir()
	if err := InitLogger(secondRoot); err != nil {
		t.Fatalf("second InitLogger returned error: %v", err)
	}
	logrus.SetOutput(io.Discard)
	logrus.Info("second-info-message")
	logrus.Debug("second-debug-message")
	logrus.Error("second-error-message")
	if err := CloseLogger(); err != nil {
		t.Fatalf("CloseLogger returned error: %v", err)
	}
	if err := CloseLogger(); err != nil {
		t.Fatalf("second CloseLogger should be idempotent, got: %v", err)
	}

	assertLogFileContains(t, filepath.Join(firstRoot, "logs", "info.log"), "first-info-message", "second-info-message")
	assertLogFileContains(t, filepath.Join(firstRoot, "logs", "debug.log"), "first-debug-message", "second-debug-message")
	assertLogFileContains(t, filepath.Join(firstRoot, "logs", "error.log"), "first-warning-message", "second-error-message")
	assertLogFileContains(t, filepath.Join(secondRoot, "logs", "info.log"), "second-info-message", "first-info-message")
	assertLogFileContains(t, filepath.Join(secondRoot, "logs", "debug.log"), "second-debug-message", "first-debug-message")
	assertLogFileContains(t, filepath.Join(secondRoot, "logs", "error.log"), "second-error-message", "first-warning-message")

	if err := os.Rename(filepath.Join(firstRoot, "logs"), filepath.Join(firstRoot, "logs-closed")); err != nil {
		t.Fatalf("rename first log directory after reinitialization: %v", err)
	}
	if err := os.Rename(filepath.Join(secondRoot, "logs"), filepath.Join(secondRoot, "logs-closed")); err != nil {
		t.Fatalf("rename second log directory after CloseLogger: %v", err)
	}
}

func TestInitLoggerReturnsDirectoryCreationError(t *testing.T) {
	_ = CloseLogger()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "logs"), []byte("not a directory"), 0600); err != nil {
		t.Fatalf("create blocking file: %v", err)
	}
	if err := InitLogger(root); err == nil {
		t.Fatal("InitLogger should fail when the log path is a file")
	}
}

func TestInitLoggerInvalidLevelFallsBackToInfo(t *testing.T) {
	logger := logrus.StandardLogger()
	oldOutput := logger.Out
	oldFormatter := logger.Formatter
	oldLevel := logger.Level
	oldReportCaller := logger.ReportCaller
	defer func() {
		_ = CloseLogger()
		logrus.SetOutput(oldOutput)
		logrus.SetFormatter(oldFormatter)
		logrus.SetLevel(oldLevel)
		logrus.SetReportCaller(oldReportCaller)
	}()
	_ = CloseLogger()
	t.Setenv("SMARTPING_LOG_LEVEL", "not-a-level")

	if err := InitLogger(t.TempDir()); err != nil {
		t.Fatalf("InitLogger returned error for invalid level: %v", err)
	}
	logrus.SetOutput(io.Discard)
	if got := logrus.GetLevel(); got != logrus.InfoLevel {
		t.Fatalf("log level = %s, want info fallback", got)
	}
}

func assertLogFileContains(t *testing.T, path, included, excluded string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	text := string(content)
	if strings.Count(text, included) != 1 {
		t.Fatalf("%s should contain %q exactly once: %q", path, included, text)
	}
	if strings.Contains(text, excluded) {
		t.Fatalf("%s unexpectedly contains %q: %q", path, excluded, text)
	}
}
