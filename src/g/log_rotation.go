package g

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
)

const (
	logFileMaxBytes = 10 << 20
	logFileBackups  = 3
)

// rotatingLogFile serializes writes and rotation, including shared log levels.
type rotatingLogFile struct {
	mu       sync.Mutex
	path     string
	file     *os.File
	size     int64
	maxBytes int64
	backups  int
	closed   bool
}

func openRotatingLogFile(path string, maxBytes int64, backups int) (*rotatingLogFile, error) {
	if maxBytes <= 0 || backups < 1 {
		return nil, errors.New("invalid log rotation limits")
	}
	writer := &rotatingLogFile{path: path, maxBytes: maxBytes, backups: backups}
	if err := writer.open(); err != nil {
		return nil, err
	}
	return writer, nil
}

func (writer *rotatingLogFile) open() error {
	file, err := os.OpenFile(writer.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file %s: %w", writer.path, err)
	}
	info, err := file.Stat()
	if err != nil {
		return errors.Join(err, file.Close())
	}
	writer.file, writer.size = file, info.Size()
	return nil
}

func (writer *rotatingLogFile) Write(data []byte) (int, error) {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	if writer.closed {
		return 0, os.ErrClosed
	}
	if len(data) == 0 {
		return 0, nil
	}
	if writer.file == nil {
		if err := writer.open(); err != nil {
			return 0, err
		}
	}
	// Keep a single oversized entry intact, then rotate before the next entry.
	if writer.size > 0 && writer.size+int64(len(data)) > writer.maxBytes {
		if err := writer.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := writer.file.Write(data)
	writer.size += int64(n)
	return n, err
}

func (writer *rotatingLogFile) rotate() (err error) {
	// Windows requires closing the active file before renaming it.
	err = writer.file.Close()
	writer.file = nil
	// Reopen even after a failed rename so the next write can retry rotation.
	defer func() { err = errors.Join(err, writer.open()) }()
	if err != nil {
		return err
	}
	oldest := writer.path + "." + strconv.Itoa(writer.backups)
	if err = os.Remove(oldest); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for index := writer.backups - 1; index >= 1; index-- {
		source := writer.path + "." + strconv.Itoa(index)
		target := writer.path + "." + strconv.Itoa(index+1)
		if err = os.Rename(source, target); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	err = os.Rename(writer.path, writer.path+".1")
	return err
}

func (writer *rotatingLogFile) Close() error {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	if writer.closed {
		return nil
	}
	writer.closed = true
	if writer.file == nil {
		return nil
	}
	err := writer.file.Close()
	writer.file = nil
	return err
}
