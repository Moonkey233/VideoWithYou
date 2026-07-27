package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

const (
	defaultMaxBytes = 5 * 1024 * 1024
	defaultBackups  = 5
)

type RotatingWriter struct {
	mu       sync.Mutex
	path     string
	file     *os.File
	size     int64
	maxBytes int64
	backups  int
}

func New(logDir string) (*log.Logger, io.Closer, error) {
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return nil, nil, err
	}
	writer := &RotatingWriter{
		path:     filepath.Join(logDir, "videowithyou.log"),
		maxBytes: defaultMaxBytes,
		backups:  defaultBackups,
	}
	if err := writer.open(); err != nil {
		return nil, nil, err
	}
	logger := log.New(io.MultiWriter(os.Stdout, writer), "", log.Ldate|log.Ltime|log.Lmicroseconds)
	return logger, writer, nil
}

func (w *RotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return 0, os.ErrClosed
	}
	if w.size+int64(len(p)) > w.maxBytes {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *RotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func (w *RotatingWriter) open() error {
	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return err
	}
	w.file = file
	w.size = info.Size()
	return nil
}

func (w *RotatingWriter) rotate() error {
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			return err
		}
		w.file = nil
	}
	oldest := fmt.Sprintf("%s.%d", w.path, w.backups)
	_ = os.Remove(oldest)
	for index := w.backups - 1; index >= 1; index-- {
		source := fmt.Sprintf("%s.%d", w.path, index)
		target := fmt.Sprintf("%s.%d", w.path, index+1)
		if _, err := os.Stat(source); err == nil {
			if err := os.Rename(source, target); err != nil {
				return err
			}
		}
	}
	if _, err := os.Stat(w.path); err == nil {
		if err := os.Rename(w.path, w.path+".1"); err != nil {
			return err
		}
	}
	return w.open()
}
