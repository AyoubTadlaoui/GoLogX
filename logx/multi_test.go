package logx

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

// errHandler always errors when asked to handle a record.
type errHandler struct {
	enabled bool
	err     error
}

func (e *errHandler) Enabled(context.Context, slog.Level) bool  { return e.enabled }
func (e *errHandler) Handle(context.Context, slog.Record) error { return e.err }
func (e *errHandler) WithAttrs(_ []slog.Attr) slog.Handler      { return e }
func (e *errHandler) WithGroup(_ string) slog.Handler           { return e }

func TestMultiHandler_DeliversToAll(t *testing.T) {
	var a, b bytes.Buffer
	hA := slog.NewJSONHandler(&a, &slog.HandlerOptions{Level: slog.LevelDebug})
	hB := NewPrettyHandler(&b, &PrettyHandlerOptions{Level: slog.LevelDebug, NoColor: true})

	log := slog.New(NewMultiHandler(hA, hB))
	log.Info("hello", "k", "v")

	if !strings.Contains(a.String(), `"msg":"hello"`) {
		t.Errorf("json sink missed record: %q", a.String())
	}
	if !strings.Contains(b.String(), "hello") || !strings.Contains(b.String(), "k=v") {
		t.Errorf("pretty sink missed record: %q", b.String())
	}
}

func TestMultiHandler_EnabledIsUnion(t *testing.T) {
	debugSink := &errHandler{enabled: true}
	infoOnlySink := &errHandler{enabled: false}
	m := NewMultiHandler(debugSink, infoOnlySink)
	if !m.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("Enabled should be true if any sink is enabled")
	}
}

func TestMultiHandler_JoinErrors(t *testing.T) {
	errA := errors.New("sinkA")
	errB := errors.New("sinkB")
	m := NewMultiHandler(
		&errHandler{enabled: true, err: errA},
		&errHandler{enabled: true, err: errB},
	)
	err := m.Handle(context.Background(), slog.Record{Level: slog.LevelInfo})
	if !errors.Is(err, errA) || !errors.Is(err, errB) {
		t.Fatalf("expected joined errA & errB, got %v", err)
	}
}

func TestMultiHandler_WithAttrsPropagates(t *testing.T) {
	var a, b bytes.Buffer
	hA := slog.NewJSONHandler(&a, nil)
	hB := NewPrettyHandler(&b, &PrettyHandlerOptions{NoColor: true})

	log := slog.New(NewMultiHandler(hA, hB)).With("svc", "api")
	log.Info("m")

	if !strings.Contains(a.String(), `"svc":"api"`) {
		t.Errorf("attrs missing from json sink: %q", a.String())
	}
	if !strings.Contains(b.String(), "svc=api") {
		t.Errorf("attrs missing from pretty sink: %q", b.String())
	}
}

func TestMultiHandler_NilHandlerIgnored(t *testing.T) {
	var buf bytes.Buffer
	h := NewPrettyHandler(&buf, &PrettyHandlerOptions{NoColor: true})
	log := slog.New(NewMultiHandler(nil, h, nil))
	log.Info("ok")
	if !strings.Contains(buf.String(), "ok") {
		t.Fatalf("expected output despite nil sinks, got %q", buf.String())
	}
}
