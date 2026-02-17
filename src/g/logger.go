package g

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
	writerHook "github.com/sirupsen/logrus/hooks/writer"
)

func InitLogger(root string) {
	logDir := filepath.Join(root, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Fatalln("[Fault]create logs dir fail:", err)
	}

	infoFile := openLogFile(filepath.Join(logDir, "info.log"))
	debugFile := openLogFile(filepath.Join(logDir, "debug.log"))
	errorFile := openLogFile(filepath.Join(logDir, "error.log"))

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
			log.Println("[Warn]invalid SMARTPING_LOG_LEVEL:", levelRaw, "fallback to info")
		} else {
			logrus.SetLevel(level)
		}
	}

	logrus.AddHook(&writerHook.Hook{
		Writer:    infoFile,
		LogLevels: []logrus.Level{logrus.InfoLevel},
	})
	logrus.AddHook(&writerHook.Hook{
		Writer:    debugFile,
		LogLevels: []logrus.Level{logrus.DebugLevel, logrus.TraceLevel},
	})
	logrus.AddHook(&writerHook.Hook{
		Writer: errorFile,
		LogLevels: []logrus.Level{
			logrus.WarnLevel,
			logrus.ErrorLevel,
			logrus.FatalLevel,
			logrus.PanicLevel,
		},
	})
}

func openLogFile(path string) *os.File {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalln("[Fault]open log file fail:", path, err)
	}
	return f
}
