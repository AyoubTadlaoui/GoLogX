package logx

import (
	"io"
	"log/slog"
	"testing"
)

// Benchmarks compare PrettyHandler against the stdlib handlers writing to
// io.Discard. They report ns/op and allocs/op — run with:
//
//	go test ./logx -bench=. -benchmem
func benchLog(b *testing.B, log *slog.Logger) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Info("event", "user", "ayoub", "id", 42, "ok", true)
	}
}

func BenchmarkPretty(b *testing.B) {
	log := slog.New(NewPrettyHandler(io.Discard, &PrettyHandlerOptions{NoColor: true}))
	benchLog(b, log)
}

func BenchmarkPrettyColor(b *testing.B) {
	log := slog.New(NewPrettyHandler(io.Discard, &PrettyHandlerOptions{NoColor: false}))
	benchLog(b, log)
}

func BenchmarkStdlibText(b *testing.B) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	benchLog(b, log)
}

func BenchmarkStdlibJSON(b *testing.B) {
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))
	benchLog(b, log)
}

func BenchmarkMulti(b *testing.B) {
	pretty := NewPrettyHandler(io.Discard, &PrettyHandlerOptions{NoColor: true})
	json := slog.NewJSONHandler(io.Discard, nil)
	log := slog.New(NewMultiHandler(pretty, json))
	benchLog(b, log)
}
