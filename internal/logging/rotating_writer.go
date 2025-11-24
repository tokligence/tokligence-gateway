package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// RotatingWriter writes to files that rotate daily and when exceeding max size.
//
// File naming:
//
//	If basePath ends with .log (or any extension), the prefix is base name without extension.
//	Output files are named: <prefix>-YYYY-MM-DD[-N].log where N is a 1-based index when size rolls over.
//	Example: logs/gateway.log -> logs/gateway-2025-10-26.log, logs/gateway-2025-10-26-2.log
//
// Rotation rules:
//   - New file each UTC day
//   - If current file size would exceed MaxBytes on write, increment index within the same day
type RotatingWriter struct {
	BasePath string
	MaxBytes int64

	mu       sync.Mutex
	curDate  string // YYYY-MM-DD
	curIndex int    // 1-based index for same-day rollover; 1 means first file of the day
	file     *os.File
	size     int64
}

// NewRotatingWriter creates a new rotating writer using basePath as the logical log file.
// If basePath is "-", writes to io.Discard to effectively disable file output.
func NewRotatingWriter(basePath string, maxBytes int64) (io.WriteCloser, error) {
	if strings.TrimSpace(basePath) == "-" {
		return nopWriteCloser{w: io.Discard}, nil
	}
	rw := &RotatingWriter{BasePath: basePath, MaxBytes: maxBytes}
	if err := rw.rotateIfNeeded(int64(0)); err != nil {
		return nil, err
	}
	return rw, nil
}

func (w *RotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.rotateIfNeeded(int64(len(p))); err != nil {
		return 0, err
	}
	n, err := w.file.Write(p)
	if err == nil {
		w.size += int64(n)
	}
	return n, err
}

func (w *RotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

func (w *RotatingWriter) rotateIfNeeded(incoming int64) error {
	// Rotate based on UTC day to avoid timezone surprises
	today := time.Now().UTC().Format("2006-01-02")
	// Compute target path for current state
	if w.file == nil || w.curDate != today {
		// New day: reset to index 1
		w.curDate = today
		w.curIndex = 1
		return w.openCurrent()
	}
	// Same day: check size threshold
	if w.size+incoming > w.MaxBytes {
		w.curIndex++
		return w.openCurrent()
	}
	return nil
}

func (w *RotatingWriter) openCurrent() error {
	// Close existing
	if w.file != nil {
		_ = w.file.Close()
	}
	dir, name := filepath.Split(w.BasePath)
	if dir == "" {
		dir = "."
	}
	// Ensure directory exists
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	if ext == "" {
		ext = ".log"
	}
	filename := fmt.Sprintf("%s-%s%s", base, w.curDate, ext)
	if w.curIndex > 1 {
		filename = fmt.Sprintf("%s-%s-%d%s", base, w.curDate, w.curIndex, ext)
	}
	path := filepath.Join(dir, filename)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	// Determine current size
	st, err := f.Stat()
	var size int64
	if err == nil {
		size = st.Size()
	}
	w.file = f
	w.size = size
	w.updatePointer(path)
	return nil
}

func (w *RotatingWriter) updatePointer(target string) {
	base := strings.TrimSpace(w.BasePath)
	if base == "" || base == "-" {
		return
	}
	// If base already points to target, skip.
	if info, err := os.Lstat(base); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			if dest, derr := os.Readlink(base); derr == nil && dest == target {
				return
			}
		}
		_ = os.Remove(base)
	}
	// Prefer symbolic link; fall back to hard link; finally write pointer text.
	if err := os.Symlink(target, base); err == nil {
		return
	}
	if err := os.Link(target, base); err == nil {
		return
	}
	if f, err := os.OpenFile(base, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644); err == nil {
		defer f.Close()
		_, _ = fmt.Fprintf(f, "current log file: %s\n", target)
	}
}

type nopWriteCloser struct{ w io.Writer }

func (n nopWriteCloser) Write(p []byte) (int, error) { return n.w.Write(p) }
func (n nopWriteCloser) Close() error                { return nil }
