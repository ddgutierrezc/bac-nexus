package source

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"sync"
	"time"

	"bac-nexus/internal/catalog"
)

// Cursor and epoch byte widths plus the default resident quota and idle TTL.
const (
	CapabilityBytes         = 32
	EpochBytes              = 32
	DefaultResidentCapacity = 16 << 20
	DefaultIdleTTL          = 10 * time.Minute
)

var (
	// ErrInvalidCursor signals an opaque cursor whose payload is malformed, whose
	// embedded epoch does not match the store's, or whose binding (selection,
	// policy) does not match the call site's canonical selection.
	ErrInvalidCursor = errors.New("invalid snapshot cursor")
	// ErrExpiredLease signals a cursor whose idle deadline elapsed before any
	// valid access refreshed it.
	ErrExpiredLease = errors.New("snapshot lease expired")
	// ErrCapacityExceeded signals Acquire would push resident usage above the
	// configured aggregate ceiling.
	ErrCapacityExceeded = errors.New("snapshot lease capacity exceeded")
)

// ClientPolicy names an allowed client; OpenReader requires the same string the
// lease was Acquired with.
type ClientPolicy string

// Cursor is the opaque base64url encoding of a 32-byte capability followed by
// the store's 32-byte process epoch. The server-side binding lives in the lease.
type Cursor string

// LeaseStore is the bounded, TTL-managed snapshot ledger.
type LeaseStore struct {
	mu       sync.Mutex
	epoch    []byte
	capacity int64
	resident int64
	ttl      time.Duration
	now      func() time.Time
	random   io.Reader
	leases   map[string]*lease
}

type lease struct {
	capability   []byte
	selection    catalog.Candidate
	clientPolicy ClientPolicy
	size         int64
	expiresAt    time.Time
	refCount     int
	hidden       bool
	data         []byte // owned copy, best-effort zeroizable on retirement
}

// LeaseReader holds an open refcount against a lease. Close is required to
// match each successful OpenReader.
type LeaseReader struct {
	store  *LeaseStore
	lease  *lease
	closed bool
}

// NewLeaseStore generates a fresh 32-byte process epoch from random and
// prepares a ledger with the default 16 MiB resident quota and 10 minute TTL.
func NewLeaseStore(clock func() time.Time, random io.Reader) *LeaseStore {
	epoch := make([]byte, EpochBytes)
	if random != nil {
		if _, err := io.ReadFull(random, epoch); err != nil {
			copy(epoch, []byte{0x77, 0x88, 0xAA, 0xCC})
		}
	}
	return newLeaseStoreWithEpoch(epoch, clock, random, DefaultResidentCapacity, DefaultIdleTTL)
}

// newLeaseStoreWithEpoch builds the store with pre-supplied epoch, clock, and
// capacity; used by NewLeaseStore and the test/internal restart helper.
func newLeaseStoreWithEpoch(epoch []byte, clock func() time.Time, random io.Reader, capacity int64, ttl time.Duration) *LeaseStore {
	if clock == nil {
		clock = time.Now
	}
	if random == nil {
		random = bytes.NewReader(nil)
	}
	return &LeaseStore{
		epoch:    append([]byte(nil), epoch...),
		capacity: capacity,
		ttl:      ttl,
		now:      clock,
		random:   random,
		leases:   make(map[string]*lease),
	}
}

// ProcessEpoch returns a defensive copy of the 32-byte process epoch so MCP
// diagnostics and tests can verify a cursor's embedded epoch matches.
func (s *LeaseStore) ProcessEpoch() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.epoch...)
}

// Capacity returns the configured aggregate resident quota in bytes.
func (s *LeaseStore) Capacity() int64 { return s.capacity }

// Resident returns the current resident byte usage (sum of un-retired leases).
func (s *LeaseStore) Resident() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.resident
}

// Acquire copies snapshot bytes into resident memory, binds the cursor to
// selection + policy + epoch, mints a fresh 32-byte capability, and reserves
// aggregate quota. ErrCapacityExceeded is returned before any state changes
// when admission fails.
func (s *LeaseStore) Acquire(snap *Snapshot, selection catalog.Candidate, policy ClientPolicy) (Cursor, error) {
	if snap == nil {
		return "", ErrInvalidCursor
	}
	size := int64(len(snap.content))

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.resident+size > s.capacity {
		return "", ErrCapacityExceeded
	}
	capability := make([]byte, CapabilityBytes)
	if _, err := io.ReadFull(s.random, capability); err != nil {
		return "", errors.New("snapshot lease entropy unavailable")
	}
	ls := &lease{
		capability:   capability,
		selection:    selection,
		clientPolicy: policy,
		size:         size,
		expiresAt:    s.now().Add(s.ttl),
		data:         append([]byte(nil), snap.content...),
	}
	s.leases[string(capability)] = ls
	s.resident += size
	return encodeCursor(capability, s.epoch), nil
}

func encodeCursor(capability, epoch []byte) Cursor {
	buf := make([]byte, 0, len(capability)+len(epoch))
	buf = append(buf, capability...)
	buf = append(buf, epoch...)
	return Cursor(base64.RawURLEncoding.EncodeToString(buf))
}

func decodeCursor(c Cursor) (capability, epoch []byte, err error) {
	raw, err := base64.RawURLEncoding.DecodeString(string(c))
	if err != nil || len(raw) != CapabilityBytes+EpochBytes {
		return nil, nil, ErrInvalidCursor
	}
	return raw[:CapabilityBytes], raw[CapabilityBytes:], nil
}

// OpenReader validates the cursor, validates the embedding epoch and binding
// (selection + policy), refreshes TTL, and increments the refcount.
func (s *LeaseStore) OpenReader(cursor Cursor, selection catalog.Candidate, policy ClientPolicy) (*LeaseReader, error) {
	capability, cursorEpoch, err := decodeCursor(cursor)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !bytes.Equal(cursorEpoch, s.epoch) {
		return nil, ErrInvalidCursor
	}
	ls, ok := s.leases[string(capability)]
	if !ok || ls.hidden {
		return nil, ErrInvalidCursor
	}
	if ls.selection != selection || ls.clientPolicy != policy {
		return nil, ErrInvalidCursor
	}
	if !s.now().Before(ls.expiresAt) {
		return nil, ErrExpiredLease
	}
	ls.expiresAt = s.now().Add(s.ttl)
	ls.refCount++
	return &LeaseReader{store: s, lease: ls}, nil
}

// Page serves the requested range from the immutable resident bytes and
// refreshes TTL. The transient snapshot copy keeps lease bytes untouched.
func (r *LeaseReader) Page(startLine, maxLines int) (Page, error) {
	if r.closed {
		return Page{}, ErrInvalidCursor
	}
	r.store.mu.Lock()
	r.lease.expiresAt = r.store.now().Add(r.store.ttl)
	data := append([]byte(nil), r.lease.data...)
	r.store.mu.Unlock()
	snap, err := NewSnapshot(data)
	if err != nil {
		return Page{}, err
	}
	return snap.Page(startLine, maxLines)
}

// Close releases the refcount. The lease retires only when it has no active
// readers and has been evicted or its idle deadline has elapsed.
func (r *LeaseReader) Close() {
	if r.closed {
		return
	}
	r.closed = true
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	r.lease.refCount--
	if r.lease.refCount > 0 {
		return
	}
	if r.lease.hidden || !r.store.now().Before(r.lease.expiresAt) {
		r.store.retireLocked(r.lease)
	}
}

// retireLocked removes the lease from the lookup map, frees the resident
// quota, and best-effort zeros its data buffer.
func (s *LeaseStore) retireLocked(ls *lease) {
	delete(s.leases, string(ls.capability))
	s.resident -= ls.size
	for i := range ls.data {
		ls.data[i] = 0
	}
}

// Evict marks the lease hidden; existing readers keep serving through their
// Close, and subsequent OpenReader calls return ErrInvalidCursor.
func (s *LeaseStore) Evict(cursor Cursor) error {
	capability, _, err := decodeCursor(cursor)
	if err != nil {
		return ErrInvalidCursor
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ls, ok := s.leases[string(capability)]
	if !ok || ls.hidden {
		return ErrInvalidCursor
	}
	ls.hidden = true
	if ls.refCount == 0 {
		s.retireLocked(ls)
	}
	return nil
}

// Restart simulates a process restart: it rotates the process epoch and retires
// every lease with no active readers.
func (s *LeaseStore) Restart() {
	s.mu.Lock()
	defer s.mu.Unlock()
	newEpoch := make([]byte, EpochBytes)
	if _, err := io.ReadFull(s.random, newEpoch); err == nil {
		s.epoch = newEpoch
	} else {
		for i := range s.epoch {
			s.epoch[i] ^= 0xFF
		}
	}
	for _, ls := range s.leases {
		if ls.refCount == 0 {
			s.retireLocked(ls)
		}
	}
}
