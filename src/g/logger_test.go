package g

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

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
