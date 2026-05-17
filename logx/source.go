package logx

import (
	"log/slog"
	"runtime"
	"strconv"
	"strings"
)

// recordSource returns "file:line" for the call site recorded in r, or "" if
// it can't be resolved. The file path is shortened to the last two path
// components so the output stays readable.
func recordSource(r slog.Record) string {
	fs := runtime.CallersFrames([]uintptr{r.PC})
	f, _ := fs.Next()
	if f.File == "" {
		return ""
	}
	file := f.File
	if i := strings.LastIndexByte(file, '/'); i > 0 {
		if j := strings.LastIndexByte(file[:i], '/'); j >= 0 {
			file = file[j+1:]
		} else {
			file = file[i+1:]
		}
	}
	return file + ":" + strconv.Itoa(f.Line)
}
