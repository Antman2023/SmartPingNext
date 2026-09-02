package g

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

type managedLogHook struct {
	mu      sync.RWMutex
	writers map[logrus.Level]*os.File
}

var (
	applicationLogHook = &managedLogHook{}
	installLogHookOnce sync.Once
)

func (hook *managedLogHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (hook *managedLogHook) Fire(entry *logrus.Entry) error {
	hook.mu.RLock()
	defer hook.mu.RUnlock()
	writer := hook.writers[entry.Level]
	if writer == nil {
		return nil
	}
	line, err := entry.Bytes()
	if err != nil {
		return err
	}
	_, err = writer.Write(line)
	return err
}

func (hook *managedLogHook) replace(writers map[logrus.Level]*os.File) error {
	hook.mu.Lock()
	defer hook.mu.Unlock()
	oldWriters := hook.writers
	hook.writers = writers
	return closeLogWriters(oldWriters)
}

func (hook *managedLogHook) close() error {
	return hook.replace(nil)
}

func closeLogWriters(writers map[logrus.Level]*os.File) error {
	uniqueFiles := make(map[*os.File]struct{})
	for _, file := range writers {
		if file != nil {
			uniqueFiles[file] = struct{}{}
		}
	}
	var closeErr error
	for file := range uniqueFiles {
		closeErr = errors.Join(closeErr, file.Close())
	}
	return closeErr
}

func InitLogger(root string) error {
	logDir := filepath.Join(root, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("create logs directory: %w", err)
	}

	infoFile, err := openLogFile(filepath.Join(logDir, "info.log"))
	if err != nil {
		return err
	}
	debugFile, err := openLogFile(filepath.Join(logDir, "debug.log"))
	if err != nil {
		return errors.Join(err, infoFile.Close())
	}
	errorFile, err := openLogFile(filepath.Join(logDir, "error.log"))
	if err != nil {
		return errors.Join(err, infoFile.Close(), debugFile.Close())
	}

	writers := map[logrus.Level]*os.File{
		logrus.InfoLevel:  infoFile,
		logrus.DebugLevel: debugFile,
		logrus.TraceLevel: debugFile,
		logrus.WarnLevel:  errorFile,
		logrus.ErrorLevel: errorFile,
		logrus.FatalLevel: errorFile,
		logrus.PanicLevel: errorFile,
	}
	if err := applicationLogHook.replace(writers); err != nil {
		return err
	}
	installLogHookOnce.Do(func() {
		logrus.AddHook(applicationLogHook)
	})

	logrus.SetFormatter(&logrus.TextFormatter{
		DisableColors:   true,
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02/15:04:05",
		CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
			return "", filepath.Base(frame.File)
		},
	})
	logrus.SetOutput(os.Stdout)
	logrus.SetReportCaller(true)
	logrus.SetLevel(logrus.InfoLevel)

	if levelRaw := strings.TrimSpace(os.Getenv("SMARTPING_LOG_LEVEL")); levelRaw != "" {
		level, err := logrus.ParseLevel(strings.ToLower(levelRaw))
		if err != nil {
			log.Printf("[Warn]invalid SMARTPING_LOG_LEVEL: %s, fallback to info", levelRaw)
			return nil
		}
		logrus.SetLevel(level)
	}
	return nil
}

func CloseLogger() error {
	return applicationLogHook.close()
}

func openLogFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("open log file %s: %w", path, err)
	}
	return f, nil
}
