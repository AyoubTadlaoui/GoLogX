// Command logx pretty-prints JSON slog logs from stdin or a file.
//
//	myapp 2>&1 | logx
//	logx -f /var/log/app.json
//	logx -level=warn -grep=timeout app.log
//
// Lines that don't parse as JSON are passed through verbatim so it works
// safely on mixed streams (e.g. a Go binary's stderr that includes panics).
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/AyoubTadlaoui/GoLogX/logx"
)

// version is overridden at build time via -ldflags "-X main.version=...".
// When unset (e.g. plain `go install` without ldflags) we fall back to the
// version recorded in the module build info so users still see something
// meaningful from `logx -version`.
var version = "dev"

func resolvedVersion() string {
	if version != "dev" && version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

type config struct {
	level      slog.Level
	grep       string
	follow     bool
	noColor    bool
	addSource  bool
	timeFormat string
	files      []string
}

func parseFlags(argv []string, stdout, stderr io.Writer) (*config, error) {
	fs := flag.NewFlagSet("logx", flag.ContinueOnError)
	// Errors and -h usage go to stderr — that's where flag package puts them
	// and what `tool 2>/dev/null` users expect when they're suppressing chatter.
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(stderr, `logx %s — pretty-print JSON slog logs

Usage:
  logx [flags] [file ...]
  myapp 2>&1 | logx [flags]

Flags:
`, resolvedVersion())
		fs.PrintDefaults()
	}

	levelStr := fs.String("level", "debug", "minimum level to show (debug, info, warn, error)")
	grep := fs.String("grep", "", "only show lines containing this substring (raw line, before JSON parse)")
	follow := fs.Bool("f", false, "follow the file like 'tail -f' (single file only)")
	noColor := fs.Bool("no-color", false, "disable ANSI colors")
	addSource := fs.Bool("source", false, "show source file:line if present in record")
	timeFmt := fs.String("time", "15:04:05.000", "Go time format for timestamps")
	showVer := fs.Bool("version", false, "print version and exit")

	if err := fs.Parse(argv); err != nil {
		return nil, err
	}
	if *showVer {
		// -version output goes to STDOUT, matching git/go/node/etc. and
		// letting `$(logx -version)` capture it. The previous version wrote
		// to stderr, which broke `brew test`'s shell_output() assertion.
		fmt.Fprintln(stdout, resolvedVersion())
		return nil, errVersionPrinted
	}

	lvl, err := logx.LevelFromString(*levelStr)
	if err != nil {
		fs.Usage()
		return nil, err
	}

	return &config{
		level:      lvl,
		grep:       *grep,
		follow:     *follow,
		noColor:    *noColor,
		addSource:  *addSource,
		timeFormat: *timeFmt,
		files:      fs.Args(),
	}, nil
}

// errVersionPrinted is a sentinel that lets main exit 0 after -version.
var errVersionPrinted = errors.New("version printed")

// run is the testable entry point. It returns the process exit code.
func run(stdin io.Reader, stdout, stderr io.Writer, argv []string) int {
	cfg, err := parseFlags(argv, stdout, stderr)
	if err != nil {
		if errors.Is(err, errVersionPrinted) {
			return 0
		}
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(stderr, "logx:", err)
		return 2
	}

	handler := logx.NewPrettyHandler(stdout, &logx.PrettyHandlerOptions{
		Level:      cfg.level,
		TimeFormat: cfg.timeFormat,
		NoColor:    cfg.noColor,
		AddSource:  cfg.addSource,
	})

	switch {
	case len(cfg.files) == 0:
		return streamReader(stdin, stdout, handler, cfg, stderr)
	case cfg.follow:
		if len(cfg.files) != 1 {
			fmt.Fprintln(stderr, "logx: -f requires exactly one file")
			return 2
		}
		return followFile(cfg.files[0], stdout, handler, cfg, stderr)
	default:
		exit := 0
		for _, path := range cfg.files {
			f, err := os.Open(path)
			if err != nil {
				fmt.Fprintln(stderr, "logx:", err)
				exit = 1
				continue
			}
			if code := streamReader(f, stdout, handler, cfg, stderr); code != 0 {
				exit = code
			}
			_ = f.Close()
		}
		return exit
	}
}

// streamReader consumes r line by line, parses each line as a JSON slog
// record, and writes a pretty representation through handler. Non-JSON lines
// are passed through to passthrough (stdout) so panics aren't lost.
func streamReader(r io.Reader, passthrough io.Writer, handler slog.Handler, cfg *config, stderr io.Writer) int {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		processLine(scanner.Bytes(), passthrough, handler, cfg)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(stderr, "logx: read:", err)
		return 1
	}
	return 0
}

// followFile is a minimal `tail -f`: open the file, stream existing bytes,
// then poll for additions until SIGINT (or the file is removed/truncated).
func followFile(path string, passthrough io.Writer, handler slog.Handler, cfg *config, stderr io.Writer) int {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintln(stderr, "logx:", err)
		return 1
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			processLine(stripTrailingNewline(line), passthrough, handler, cfg)
		}
		if errors.Is(err, io.EOF) {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		if err != nil {
			fmt.Fprintln(stderr, "logx:", err)
			return 1
		}
	}
}

func stripTrailingNewline(b []byte) []byte {
	if n := len(b); n > 0 && b[n-1] == '\n' {
		return b[:n-1]
	}
	return b
}

// processLine handles one input line: if it's a JSON slog record, it re-emits
// it via handler; otherwise it's passed through to passthrough (typically stdout)
// untouched so panics and plain text don't disappear.
func processLine(line []byte, passthrough io.Writer, handler slog.Handler, cfg *config) {
	if len(line) == 0 {
		return
	}
	if cfg.grep != "" && !strings.Contains(string(line), cfg.grep) {
		return
	}
	rec, ok := decodeRecord(line)
	if !ok {
		fmt.Fprintln(passthrough, string(line))
		return
	}
	if rec.Level < cfg.level {
		return
	}
	_ = handler.Handle(context.Background(), rec)
}

// decodeRecord parses a single JSON-slog line into a slog.Record.
// Returns ok=false when the line isn't a JSON object — the caller can then
// fall back to pass-through.
func decodeRecord(line []byte) (slog.Record, bool) {
	var raw map[string]any
	if err := json.Unmarshal(line, &raw); err != nil {
		return slog.Record{}, false
	}

	r := slog.Record{}
	// time: slog's JSONHandler uses "time" (RFC3339). Be lenient.
	if v, ok := raw["time"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			r.Time = t
		} else if t, err := time.Parse(time.RFC3339, v); err == nil {
			r.Time = t
		}
	}
	delete(raw, "time")

	// level
	switch v := raw["level"].(type) {
	case string:
		if lvl, err := logx.LevelFromString(v); err == nil {
			r.Level = lvl
		}
	case float64:
		r.Level = slog.Level(int(v))
	}
	delete(raw, "level")

	// msg
	if v, ok := raw["msg"].(string); ok {
		r.Message = v
	}
	delete(raw, "msg")

	// source — JSONHandler emits a nested object {function, file, line}.
	if v, ok := raw["source"].(map[string]any); ok {
		if file, _ := v["file"].(string); file != "" {
			if lineN, _ := v["line"].(float64); lineN > 0 {
				r.AddAttrs(slog.String("source", file+":"+strconv.Itoa(int(lineN))))
			}
		}
		delete(raw, "source")
	}

	// remaining keys become attrs (stable-ordered by key for deterministic output)
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sortStrings(keys)
	for _, k := range keys {
		r.AddAttrs(slog.Any(k, raw[k]))
	}
	return r, true
}

// sortStrings is a tiny insertion sort to avoid pulling in sort just for this hot path.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

func main() {
	os.Exit(run(os.Stdin, os.Stdout, os.Stderr, os.Args[1:]))
}
