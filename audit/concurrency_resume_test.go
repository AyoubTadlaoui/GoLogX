package audit

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
)

// ---- Attack class (k): concurrent writes through one handler. ----

// TestConcurrentWrites_ChainStaysIntact fires many goroutines at one handler
// (and at WithAttrs-derived handlers sharing the same core) and then verifies
// the file. Run under -race, this proves the lock keeps chain links from
// interleaving: the result must be OK with contiguous seqs and the exact count.
func TestConcurrentWrites_ChainStaysIntact(t *testing.T) {
	for _, signed := range []bool{false, true} {
		pub, priv := keypairIf(t, signed)
		var buf safeBuffer
		h, err := NewHandler(&buf, &Options{Signer: priv})
		if err != nil {
			t.Fatal(err)
		}
		log := slog.New(h)

		const goroutines = 16
		const perGoroutine = 64
		var wg sync.WaitGroup
		for g := 0; g < goroutines; g++ {
			wg.Add(1)
			go func(g int) {
				defer wg.Done()
				// Mix plain and WithAttrs-derived loggers (same core).
				l := log.With("worker", g)
				for i := 0; i < perGoroutine; i++ {
					l.Info("event", slog.Int("i", i))
				}
			}(g)
		}
		wg.Wait()

		want := goroutines * perGoroutine
		rep := mustVerify(t, buf.Bytes(), pub)
		requireOK(t, rep)
		if rep.Entries != want {
			t.Fatalf("signed=%v expected %d entries, got %d", signed, want, rep.Entries)
		}
		// Confirm seqs are exactly 0..want-1, contiguous, no dupes.
		assertContiguousSeqs(t, buf.Bytes(), want)
	}
}

// safeBuffer is a tiny mutex-guarded bytes.Buffer. The handler already
// serializes writes under its own lock, but the buffer guard removes any doubt
// for the race detector about the test's own access pattern.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *safeBuffer) Bytes() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.buf.Bytes()...)
}

// assertContiguousSeqs parses every entry and asserts the seqs form the exact
// set 0..n-1 with no gaps or duplicates.
func assertContiguousSeqs(t *testing.T, file []byte, n int) {
	t.Helper()
	var seqs []uint64
	for _, l := range lines(file) {
		if len(bytes.TrimSpace(l)) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(l, &e); err != nil {
			t.Fatalf("parse: %v", err)
		}
		seqs = append(seqs, e.Seq)
	}
	if len(seqs) != n {
		t.Fatalf("expected %d seqs, got %d", n, len(seqs))
	}
	sort.Slice(seqs, func(i, j int) bool { return seqs[i] < seqs[j] })
	for i, s := range seqs {
		if s != uint64(i) {
			t.Fatalf("seq set not contiguous: position %d has seq %d", i, s)
		}
	}
}

// ---- Attack class (l): resume across restart. ----
//
// OpenFile resumes a non-empty log by reading its last entry (lastLine -> ReadAt)
// to recover the head seq and prev hash, then continuing the chain. The tests
// below exercise that real path across a close/reopen, plus the supported manual
// path (NewHandler with StartSeq/PrevHash) the resume logic is built on.

// TestResume_OpenFileContinuousChain proves OpenFile resumes a NON-EMPTY file:
// write a batch, close, reopen the same path, write more, and verify the whole
// file as one continuous, tamper-evident chain (signed and unsigned).
func TestResume_OpenFileContinuousChain(t *testing.T) {
	for _, signed := range []bool{false, true} {
		pub, priv := keypairIf(t, signed)
		dir := t.TempDir()
		path := filepath.Join(dir, "audit.log")

		// Session 1.
		h1, f1, err := OpenFile(path, &Options{Signer: priv})
		if err != nil {
			t.Fatalf("session 1 OpenFile: %v", err)
		}
		l1 := slog.New(h1)
		l1.Info("session1-a", "k", 1)
		l1.Info("session1-b", "k", 2)
		if err := f1.Close(); err != nil {
			t.Fatal(err)
		}

		// Session 2 reopens the existing, non-empty file and must continue it.
		h2, f2, err := OpenFile(path, &Options{Signer: priv})
		if err != nil {
			t.Fatalf("session 2 OpenFile (resume) must succeed: %v", err)
		}
		l2 := slog.New(h2)
		l2.Info("session2-a", "k", 3)
		l2.Info("session2-b", "k", 4)
		if err := f2.Close(); err != nil {
			t.Fatal(err)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		rep := mustVerify(t, data, pub)
		requireOK(t, rep)
		if rep.Entries != 4 {
			t.Fatalf("signed=%v expected 4 entries across OpenFile resume, got %d", signed, rep.Entries)
		}
		assertContiguousSeqs(t, data, 4)
	}
}

// TestResume_OpenFileTamperAcrossBoundary_Detected resumes via OpenFile, then
// tampers entries on BOTH sides of the restart seam; each edit must be caught,
// proving the seam is not a blind spot through the real resume path.
func TestResume_OpenFileTamperAcrossBoundary_Detected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	h1, f1, err := OpenFile(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	slog.New(h1).Info("before-restart", "k", 1)
	if err := f1.Close(); err != nil {
		t.Fatal(err)
	}

	h2, f2, err := OpenFile(path, nil)
	if err != nil {
		t.Fatalf("resume OpenFile must succeed: %v", err)
	}
	slog.New(h2).Info("after-restart", "k", 2)
	if err := f2.Close(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	requireOK(t, mustVerify(t, data, nil)) // clean baseline

	after := bytesReplaceOnce(t, data, `"after-restart"`, `"forged-after"`)
	requireBad(t, mustVerify(t, after, nil), 1)

	before := bytesReplaceOnce(t, data, `"before-restart"`, `"forged-befor"`)
	requireBad(t, mustVerify(t, before, nil), 0)
}

// TestResume_OpenFileFirstWriteFromEmpty confirms OpenFile DOES work for the
// genesis session (empty file) and produces a verifiable single entry.
func TestResume_OpenFileFirstWriteFromEmpty(t *testing.T) {
	for _, signed := range []bool{false, true} {
		pub, priv := keypairIf(t, signed)
		dir := t.TempDir()
		path := filepath.Join(dir, "audit.log")
		h, f, err := OpenFile(path, &Options{Signer: priv})
		if err != nil {
			t.Fatal(err)
		}
		l := slog.New(h)
		l.Info("a", "k", 1)
		l.Info("b", "k", 2)
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		rep := mustVerify(t, data, pub)
		requireOK(t, rep)
		if rep.Entries != 2 {
			t.Fatalf("signed=%v expected 2 entries, got %d", signed, rep.Entries)
		}
		assertContiguousSeqs(t, data, 2)
	}
}

// resumeManually mirrors what a corrected OpenFile would do: read the last entry
// of an existing chain and continue it via NewHandler with StartSeq/PrevHash.
// This is the supported resume contract (Options documents StartSeq+PrevHash),
// so it is exercised here to prove the chain logic itself resumes cleanly.
func resumeManually(t *testing.T, existing []byte, w *bytes.Buffer, signer ed25519.PrivateKey) *Handler {
	t.Helper()
	w.Write(existing)
	ls := lines(existing)
	var last Entry
	if err := json.Unmarshal(ls[len(ls)-1], &last); err != nil {
		t.Fatal(err)
	}
	h, err := NewHandler(w, &Options{
		Signer:   signer,
		StartSeq: last.Seq + 1,
		PrevHash: last.Hash,
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// TestResume_ManualContinuousChain writes a first batch, "restarts" by resuming
// from the last entry's seq/hash, writes more, and verifies the whole file as
// one continuous, OK chain (signed and unsigned).
func TestResume_ManualContinuousChain(t *testing.T) {
	for _, signed := range []bool{false, true} {
		pub, priv := keypairIf(t, signed)

		// Session 1.
		var s1 bytes.Buffer
		h1, err := NewHandler(&s1, &Options{Signer: priv})
		if err != nil {
			t.Fatal(err)
		}
		slog.New(h1).Info("session1-a", "k", 1)
		slog.New(h1).Info("session1-b", "k", 2)

		// Session 2 resumes from session 1's bytes.
		var whole bytes.Buffer
		h2 := resumeManually(t, s1.Bytes(), &whole, priv)
		slog.New(h2).Info("session2-a", "k", 3)
		slog.New(h2).Info("session2-b", "k", 4)

		rep := mustVerify(t, whole.Bytes(), pub)
		requireOK(t, rep)
		if rep.Entries != 4 {
			t.Fatalf("signed=%v expected 4 entries across resume, got %d", signed, rep.Entries)
		}
		assertContiguousSeqs(t, whole.Bytes(), 4)
	}
}

// TestResume_ManualTamperAcrossBoundary_Detected resumes a chain then tampers
// entries on BOTH sides of the resume seam; each edit must be caught, proving
// the seam is not a blind spot.
func TestResume_ManualTamperAcrossBoundary_Detected(t *testing.T) {
	var s1 bytes.Buffer
	h1, err := NewHandler(&s1, nil)
	if err != nil {
		t.Fatal(err)
	}
	slog.New(h1).Info("before-restart", "k", 1)

	var whole bytes.Buffer
	h2 := resumeManually(t, s1.Bytes(), &whole, nil)
	slog.New(h2).Info("after-restart", "k", 2)

	data := whole.Bytes()
	requireOK(t, mustVerify(t, data, nil)) // clean baseline

	// Tamper the post-restart entry (seq 1).
	after := bytesReplaceOnce(t, data, `"after-restart"`, `"forged-after"`)
	requireBad(t, mustVerify(t, after, nil), 1)

	// Tamper the pre-restart entry (seq 0); the resumed entry's prev chains off
	// seq 0's real hash, so editing seq 0 breaks at seq 0.
	before := bytesReplaceOnce(t, data, `"before-restart"`, `"forged-befor"`)
	requireBad(t, mustVerify(t, before, nil), 0)
}

// TestResume_ManualRecoversHeadHash confirms the resumed entry's prev equals the
// prior entry's hash and its seq increments, i.e. the resume actually links.
func TestResume_ManualRecoversHeadHash(t *testing.T) {
	var s1 bytes.Buffer
	h1, err := NewHandler(&s1, nil)
	if err != nil {
		t.Fatal(err)
	}
	slog.New(h1).Info("one")

	var whole bytes.Buffer
	h2 := resumeManually(t, s1.Bytes(), &whole, nil)
	slog.New(h2).Info("two")

	ls := lines(whole.Bytes())
	var e0, e1 Entry
	if err := json.Unmarshal(ls[0], &e0); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(ls[1], &e1); err != nil {
		t.Fatal(err)
	}
	if e1.Seq != 1 {
		t.Fatalf("resumed seq = %d, want 1", e1.Seq)
	}
	if e1.Prev != e0.Hash {
		t.Fatalf("resumed prev %q does not link to prior hash %q", e1.Prev, e0.Hash)
	}
}
