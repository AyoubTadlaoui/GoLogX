package logx

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// RotatingWriter is an io.WriteCloser that rotates a file when it would
// exceed MaxSize bytes after the next write.
//
// Rotation renames Path to Path.1, Path.1 to Path.2, ..., trimming anything
// beyond MaxBackups. It is zero-dependency by design — for production-grade
// rotation with compression and time-based windows, reach for
// gopkg.in/natefinch/lumberjack.v2.
//
// RotatingWriter is safe for concurrent use.
type RotatingWriter struct {
	// Path is the active log file. Required.
	Path string
	// MaxSize is the size in bytes that triggers a rotation. 0 disables
	// rotation entirely (behaves like a plain os.File append).
	MaxSize int64
	// MaxBackups is the number of historical files to keep (Path.1 .. Path.N).
	// 0 keeps all backups; negative is treated as 0.
	MaxBackups int
	// FileMode is applied to newly created files. 0 means 0o644.
	FileMode os.FileMode

	mu   sync.Mutex
	f    *os.File
	size int64
}

// Write implements io.Writer. It rotates before writing if the new size would
// cross MaxSize. A single Write call is never split across two files.
func (w *RotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.f == nil {
		if err := w.openLocked(); err != nil {
			return 0, err
		}
	}
	if w.MaxSize > 0 && w.size+int64(len(p)) > w.MaxSize {
		if err := w.rotateLocked(); err != nil {
			return 0, err
		}
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	return n, err
}

// Close releases the underlying file. Safe to call multiple times.
func (w *RotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}

// openLocked opens (or creates) the active file. Caller holds the lock.
func (w *RotatingWriter) openLocked() error {
	mode := w.FileMode
	if mode == 0 {
		mode = 0o644
	}
	if err := os.MkdirAll(filepath.Dir(w.Path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(w.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	w.f = f
	w.size = info.Size()
	return nil
}

// rotateLocked closes the active file, shifts backups, and opens a fresh file.
// Caller holds the lock.
func (w *RotatingWriter) rotateLocked() error {
	if w.f != nil {
		if err := w.f.Close(); err != nil {
			return err
		}
		w.f = nil
	}
	if err := w.shiftBackupsLocked(); err != nil {
		return err
	}
	// move Path -> Path.1
	if _, err := os.Stat(w.Path); err == nil {
		if err := os.Rename(w.Path, w.Path+".1"); err != nil {
			return err
		}
	}
	return w.openLocked()
}

// shiftBackupsLocked shifts Path.N up by one and trims excess. Caller holds the lock.
func (w *RotatingWriter) shiftBackupsLocked() error {
	dir := filepath.Dir(w.Path)
	base := filepath.Base(w.Path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	type backup struct {
		index int
		name  string
	}
	var backups []backup
	prefix := base + "."
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		idxStr := name[len(prefix):]
		var idx int
		if _, err := fmt.Sscanf(idxStr, "%d", &idx); err != nil || idx < 1 {
			continue
		}
		backups = append(backups, backup{index: idx, name: name})
	}
	// rename from largest down so we don't overwrite
	sort.Slice(backups, func(i, j int) bool { return backups[i].index > backups[j].index })
	for _, b := range backups {
		newIdx := b.index + 1
		// trim if we'd exceed MaxBackups (which counts active backups only)
		if w.MaxBackups > 0 && newIdx > w.MaxBackups {
			if err := os.Remove(filepath.Join(dir, b.name)); err != nil && !os.IsNotExist(err) {
				return err
			}
			continue
		}
		oldPath := filepath.Join(dir, b.name)
		newPath := filepath.Join(dir, fmt.Sprintf("%s.%d", base, newIdx))
		if err := os.Rename(oldPath, newPath); err != nil {
			return err
		}
	}
	return nil
}
