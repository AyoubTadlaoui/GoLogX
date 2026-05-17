// Example: a complete, realistic logger setup.
//
//	go run ./examples/basic
package main

import (
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/AyoubTadlaoui/GoLogX/logx"
)

func main() {
	// Rotate the JSON sink at 1 MiB, keep last 3 files.
	rotator := &logx.RotatingWriter{
		Path:       "app.log",
		MaxSize:    1 << 20,
		MaxBackups: 3,
	}
	defer rotator.Close()

	// Pretty to stderr for humans, JSON to the rotating file for machines.
	multi := logx.NewMultiHandler(
		logx.NewPrettyHandler(os.Stderr, &logx.PrettyHandlerOptions{
			Level: logx.EnvLevel(slog.LevelDebug),
		}),
		slog.NewJSONHandler(rotator, &slog.HandlerOptions{
			Level:     logx.EnvLevel(slog.LevelInfo),
			AddSource: true,
		}),
	)

	log := slog.New(multi).With(
		"service", "demo",
		"version", "0.1.0",
	)

	log.Info("starting", "port", 8080)
	log.Debug("warming caches", "items", 1024)

	start := time.Now()
	time.Sleep(80 * time.Millisecond)
	log.Info("request handled", "path", "/healthz", "dur", time.Since(start))

	log.Warn("slow downstream", "service", "db", "latency_ms", 230)
	log.Error("dropped task", "err", errors.New("queue full"), "task_id", 42)
}
