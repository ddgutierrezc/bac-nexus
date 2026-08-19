package source

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"bac-nexus/internal/catalog"
)

// fakeClock is the deterministic clock seam injected into LeaseStore so tests
// advance time without sleeping.
type fakeClock struct{ t time.Time }

func (f *fakeClock) Now() time.Time          { return f.t }
func (f *fakeClock) Advance(d time.Duration) { f.t = f.t.Add(d) }

func deterministicRandom() io.Reader {
	buf := make([]byte, 4096)
	for i := range buf {
		buf[i] = byte(i % 251)
	}
	return bytes.NewReader(buf)
}

func newTestSnapshot(t *testing.T, content string) *Snapshot {
	t.Helper()
	snap, err := NewSnapshot([]byte(content))
	if err != nil {
		t.Fatalf("NewSnapshot: %v", err)
	}
	return snap
}

func canonicalCandidate(item, library string) catalog.Candidate {
	return catalog.Candidate{
		Item: item, SourceLibrary: library, SourceFileBase: "Q",
		ObjectType: "RPGLE", SourceType: "RPGLE", Application: "TEST",
		Version: "V1", ProductionLibrary: "PRODLIB", Description: "deterministic test fixture",
	}
}

func newTestStoreAt(clock func() time.Time, capacity int64, epoch []byte) *LeaseStore {
	if clock == nil {
		clock = (&fakeClock{t: time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)}).Now
	}
	if capacity <= 0 {
		capacity = DefaultResidentCapacity
	}
	if epoch == nil {
		epoch = bytes.Repeat([]byte{0xA5}, EpochBytes)
	}
	return newLeaseStoreWithEpoch(epoch, clock, deterministicRandom(), capacity, DefaultIdleTTL)
}

func acquireLease(t *testing.T, store *LeaseStore, content string) (Cursor, catalog.Candidate) {
	t.Helper()
	snap := newTestSnapshot(t, content)
	sel := canonicalCandidate("PISA061", "SRCLIB")
	cursor, err := store.Acquire(snap, sel, "test-policy")
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	return cursor, sel
}

// --- cursor encoding & binding -------------------------------------------------

func TestStoreCursorEncodingAndBinding(t *testing.T) {
	store := newTestStoreAt(nil, 0, nil)
	cur, _ := acquireLease(t, store, "alpha\nbeta\n")

	raw, err := base64.RawURLEncoding.DecodeString(string(cur))
	if err != nil || len(raw) != CapabilityBytes+EpochBytes {
		t.Fatalf("decode err=%v len=%d", err, len(raw))
	}
	if !bytes.Equal(raw[CapabilityBytes:], store.ProcessEpoch()) {
		t.Fatalf("cursor epoch %x != store epoch %x", raw[CapabilityBytes:], store.ProcessEpoch())
	}

	bogus := []Cursor{
		"", "not-base64!!!",
		Cursor(base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{1}, CapabilityBytes+EpochBytes-1))),
		Cursor(base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{1}, CapabilityBytes+EpochBytes+1))),
	}
	for _, c := range bogus {
		if _, err := store.OpenReader(c, canonicalCandidate("PISA061", "SRCLIB"), "test-policy"); !errors.Is(err, ErrInvalidCursor) {
			t.Fatalf("err = %v for %q", err, c)
		}
	}
	for _, tt := range []struct {
		name   string
		sel    catalog.Candidate
		policy ClientPolicy
	}{
		{"different item", canonicalCandidate("OTHER", "SRCLIB"), "test-policy"},
		{"different library", canonicalCandidate("PISA061", "OTHSRC"), "test-policy"},
		{"different policy", canonicalCandidate("PISA061", "SRCLIB"), "other-policy"},
	} {
		if _, err := store.OpenReader(cur, tt.sel, tt.policy); !errors.Is(err, ErrInvalidCursor) {
			t.Fatalf("%s: err = %v, want ErrInvalidCursor", tt.name, err)
		}
	}
}

// --- replay / out-of-order / concurrency ---------------------------------------

func TestStoreReplayOutOfOrderAndConcurrency(t *testing.T) {
	store := newTestStoreAt(nil, 0, nil)
	cur, sel := acquireLease(t, store, "one\ntwo\nthree\nfour\n")

	r1, err := store.OpenReader(cur, sel, "test-policy")
	if err != nil {
		t.Fatalf("OpenReader 1: %v", err)
	}
	pageA, err := r1.Page(1, 4)
	if err != nil {
		t.Fatalf("Page 1: %v", err)
	}
	r1.Close()
	r2, err := store.OpenReader(cur, sel, "test-policy")
	if err != nil {
		t.Fatalf("OpenReader 2: %v", err)
	}
	pageB, err := r2.Page(1, 4)
	if err != nil {
		t.Fatalf("Page 2: %v", err)
	}
	r2.Close()
	if !equalLines(pageA.Lines, pageB.Lines) {
		t.Fatalf("replay differs: %#v vs %#v", pageA.Lines, pageB.Lines)
	}

	cur3, sel3 := acquireLease(t, store, "l1\nl2\nl3\nl4\nl5\nl6\nl7\n")
	reader, err := store.OpenReader(cur3, sel3, "test-policy")
	if err != nil {
		t.Fatalf("OpenReader 3: %v", err)
	}
	defer reader.Close()
	for _, p := range []struct {
		start, max int
		want       []string
	}{
		{5, 2, []string{"l5", "l6"}},
		{1, 1, []string{"l1"}},
		{7, 1, []string{"l7"}},
		{3, 3, []string{"l3", "l4", "l5"}},
	} {
		got, err := reader.Page(p.start, p.max)
		if err != nil || !equalLines(got.Lines, p.want) {
			t.Fatalf("Page(start=%d,max=%d) err=%v lines=%#v want=%#v", p.start, p.max, err, got.Lines, p.want)
		}
	}

	curC, selC := acquireLease(t, store, "alpha\nbeta\ngamma\ndelta\nepsilon\nzeta\n")
	const n = 8
	readers := make([]*LeaseReader, n)
	for i := range readers {
		r, err := store.OpenReader(curC, selC, "test-policy")
		if err != nil {
			t.Fatalf("OpenReader[%d]: %v", i, err)
		}
		readers[i] = r
	}
	type pageCase struct {
		start, max int
		want       []string
	}
	cases := []pageCase{
		{1, 6, []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta"}},
		{2, 4, []string{"beta", "gamma", "delta", "epsilon"}},
		{3, 3, []string{"gamma", "delta", "epsilon"}},
		{4, 2, []string{"delta", "epsilon"}},
		{5, 1, []string{"epsilon"}},
		{6, 1, []string{"zeta"}},
		{1, 2, []string{"alpha", "beta"}},
		{2, 5, []string{"beta", "gamma", "delta", "epsilon", "zeta"}},
	}
	var wg sync.WaitGroup
	var failures int32
	for i, r := range readers {
		wg.Add(1)
		go func(idx int, reader *LeaseReader, p pageCase) {
			defer wg.Done()
			page, err := reader.Page(p.start, p.max)
			if err != nil || !equalLines(page.Lines, p.want) {
				atomic.AddInt32(&failures, 1)
			}
		}(i, r, cases[i])
	}
	wg.Wait()
	for _, r := range readers {
		r.Close()
	}
	if failures != 0 {
		t.Fatalf("%d concurrent Page calls failed", failures)
	}
}

// --- TTL ----------------------------------------------------------------------

func TestStoreTTLRefreshAndIdleExpiry(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)}
	store := newTestStoreAt(clock.Now, 0, nil)
	cur, sel := acquireLease(t, store, "x\ny\nz\n")

	reader, err := store.OpenReader(cur, sel, "test-policy")
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	clock.Advance(DefaultIdleTTL - time.Minute)
	if _, err := reader.Page(1, 3); err != nil {
		t.Fatalf("Page near TTL: %v", err)
	}
	clock.Advance(DefaultIdleTTL - time.Minute)
	if _, err := reader.Page(1, 3); err != nil {
		t.Fatalf("Page after each refresh: %v", err)
	}
	reader.Close()

	clock.Advance(DefaultIdleTTL + time.Minute)
	if _, err := store.OpenReader(cur, sel, "test-policy"); !errors.Is(err, ErrExpiredLease) {
		t.Fatalf("err = %v, want ErrExpiredLease", err)
	}
}

// --- quota --------------------------------------------------------------------

func TestStoreQuotaAdmitsAndRecoversAfterEvict(t *testing.T) {
	store := newTestStoreAt(nil, 64, nil)

	const content = "012345678\n" // 10 bytes per Acquire
	var evictedCur Cursor
	for i := 0; i < 6; i++ {
		snap := newTestSnapshot(t, content)
		cur, err := store.Acquire(snap, canonicalCandidate(string(rune('A'+i)), "SRCLIB"), "test-policy")
		if err != nil {
			t.Fatalf("Acquire(%d): %v", i, err)
		}
		if i == 0 {
			evictedCur = cur
		}
	}
	if got := store.Resident(); got != 60 {
		t.Fatalf("resident = %d, want 60", got)
	}
	if _, err := store.Acquire(newTestSnapshot(t, "01234\n"), canonicalCandidate("OVR", "SRCLIB"), "test-policy"); !errors.Is(err, ErrCapacityExceeded) {
		t.Fatalf("err = %v, want ErrCapacityExceeded", err)
	}
	if err := store.Evict(evictedCur); err != nil {
		t.Fatalf("Evict: %v", err)
	}
	if got := store.Resident(); got != 50 {
		t.Fatalf("resident after Evict (unopened) = %d, want 50", got)
	}
	if _, err := store.Acquire(newTestSnapshot(t, "01234\n"), canonicalCandidate("NEW", "SRCLIB"), "test-policy"); err != nil {
		t.Fatalf("re-Acquire after retirement: %v", err)
	}
}

// --- eviction & zeroization ----------------------------------------------------

func TestStoreEvictionAndZeroization(t *testing.T) {
	store := newTestStoreAt(nil, 0, nil)
	cur, sel := acquireLease(t, store, "aa\nbb\ncc\n")

	first, err := store.OpenReader(cur, sel, "test-policy")
	if err != nil {
		t.Fatalf("OpenReader first: %v", err)
	}
	second, err := store.OpenReader(cur, sel, "test-policy")
	if err != nil {
		t.Fatalf("OpenReader second: %v", err)
	}
	if err := store.Evict(cur); err != nil {
		t.Fatalf("Evict: %v", err)
	}
	if _, err := store.OpenReader(cur, sel, "test-policy"); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("post-evict lookup err = %v, want ErrInvalidCursor", err)
	}
	page, err := first.Page(1, 3)
	if err != nil || !equalLines(page.Lines, []string{"aa", "bb", "cc"}) || !page.EOF {
		t.Fatalf("Page after Evict err=%v lines=%#v eof=%v", err, page.Lines, page.EOF)
	}
	first.store.mu.Lock()
	buf := first.lease.data
	first.store.mu.Unlock()
	if bytes.IndexByte(buf, 0) >= 0 {
		t.Fatalf("resident buffer is zero before retirement")
	}
	first.Close()
	if got := store.Resident(); got == 0 {
		t.Fatalf("resident dropped while second reader is still open")
	}
	second.Close()
	if got := store.Resident(); got != 0 {
		t.Fatalf("resident = %d after release, want 0", got)
	}
	for i, v := range buf {
		if v != 0 {
			t.Fatalf("retired byte at %d = 0x%02x, want 0", i, v)
		}
	}
}

// --- restart / process epoch --------------------------------------------------

func TestStoreEpochAndRestartInvalidateCursors(t *testing.T) {
	first := newTestStoreAt(nil, 0, bytes.Repeat([]byte{0x11}, EpochBytes))
	cur, sel := acquireLease(t, first, "one\ntwo\n")
	second := newTestStoreAt(nil, 0, bytes.Repeat([]byte{0x22}, EpochBytes))
	if bytes.Equal(first.ProcessEpoch(), second.ProcessEpoch()) {
		t.Fatalf("process epochs should differ between fresh stores")
	}
	if _, err := second.OpenReader(cur, sel, "test-policy"); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("post-epoch err = %v, want ErrInvalidCursor", err)
	}

	rStore := newTestStoreAt(nil, 0, nil)
	cursor, rsel := acquireLease(t, rStore, "alpha\nbeta\n")
	replacement, _ := acquireLease(t, rStore, "alpha\nbeta\n")
	reader, _ := rStore.OpenReader(cursor, rsel, "test-policy")
	reader.Close()

	rStore.Restart()

	for _, name := range []struct {
		label string
		cur   Cursor
		sel   catalog.Candidate
	}{
		{"prior cursor", cursor, rsel},
		{"unopened post-acquire cursor", replacement, canonicalCandidate("PISA061", "SRCLIB")},
	} {
		if _, err := rStore.OpenReader(name.cur, name.sel, "test-policy"); !errors.Is(err, ErrInvalidCursor) {
			t.Fatalf("%s err = %v, want ErrInvalidCursor after Restart", name.label, err)
		}
	}
	if got := rStore.Resident(); got != 0 {
		t.Fatalf("resident after Restart = %d, want 0", got)
	}
}
