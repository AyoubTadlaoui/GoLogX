package audit

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"
)

// rec builds a minimal info record with the given message and attrs.
func rec(msg string, attrs ...slog.Attr) slog.Record {
	r := slog.NewRecord(time.Now(), slog.LevelInfo, msg, 0)
	r.AddAttrs(attrs...)
	return r
}

func TestRoundTripSignedAndUnsigned(t *testing.T) {
	for _, signed := range []bool{false, true} {
		var buf bytes.Buffer
		opts := &Options{}
		var pub []byte
		if signed {
			p, priv, err := GenerateKey()
			if err != nil {
				t.Fatal(err)
			}
			opts.Signer = priv
			pub = p
		}
		h, err := NewHandler(&buf, opts)
		if err != nil {
			t.Fatal(err)
		}
		log := slog.New(h)
		log.Info("first", "n", 1)
		log.Warn("second", "name", "left-pad")
		log.Error("third")

		rep, err := Verify(bytes.NewReader(buf.Bytes()), pub)
		if err != nil {
			t.Fatalf("signed=%v verify error: %v", signed, err)
		}
		if !rep.OK {
			t.Fatalf("signed=%v expected OK, got bad seq %d: %s", signed, rep.BadSeq, rep.Reason)
		}
		if rep.Entries != 3 {
			t.Fatalf("signed=%v expected 3 entries, got %d", signed, rep.Entries)
		}
		if rep.Signed != signed {
			t.Fatalf("signed=%v report.Signed=%v", signed, rep.Signed)
		}
	}
}

func TestEditedDataBreaksChain(t *testing.T) {
	var buf bytes.Buffer
	h, _ := NewHandler(&buf, nil)
	if err := h.Handle(context.Background(), rec("clean", slog.String("k", "v"))); err != nil {
		t.Fatal(err)
	}
	if err := h.Handle(context.Background(), rec("also clean")); err != nil {
		t.Fatal(err)
	}
	tampered := bytes.Replace(buf.Bytes(), []byte(`"v"`), []byte(`"x"`), 1)
	rep, err := Verify(bytes.NewReader(tampered), nil)
	if err != nil {
		t.Fatal(err)
	}
	if rep.OK {
		t.Fatal("expected tamper to be detected, got OK")
	}
	if rep.BadSeq != 0 {
		t.Fatalf("expected break at seq 0, got %d (%s)", rep.BadSeq, rep.Reason)
	}
}
