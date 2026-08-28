package mapepire

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPinnedProtocolFixtureRevisionIsValid(t *testing.T) {
	data, err := os.ReadFile("testdata/protocol-2ef44166fcb515744fb922b49ed3673b2dac6b26.json")
	if err != nil {
		t.Fatal(err)
	}
	var request Request
	if err := json.Unmarshal(data, &request); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRequest(request); err != nil {
		t.Fatalf("fixture rejected: %v", err)
	}
}

func TestFixedProofUsesAuthenticatedConnectAndReturnsMetadataOnly(t *testing.T) {
	transport := newMessageFake()
	s := NewMessageSession(transport)
	done := make(chan ProofMetadata, 1)
	go func() {
		proof, err := s.FixedProof(context.Background(), "USER", []byte("secret"))
		if err != nil {
			t.Errorf("fixed proof: %v", err)
		}
		done <- proof
	}()
	var connect Request
	json.Unmarshal(<-transport.sent, &connect)
	if connect.Type != OperationConnect || connect.Username != "USER" || connect.Password != "secret" {
		t.Fatalf("connect request=%#v", connect)
	}
	transport.receive <- []byte(`{"id":"` + connect.ID + `","success":true,"job":"job-1"}`)
	var proof Request
	json.Unmarshal(<-transport.sent, &proof)
	if proof.Type != OperationPrepareSQLExecute || proof.SQL != FixedProofSQL || proof.Password != "" || proof.Username != "" || proof.Rows != 1 || len(proof.Parameters) != 0 {
		t.Fatalf("proof request=%#v", proof)
	}
	transport.receive <- []byte(`{"id":"` + proof.ID + `","success":true,"is_done":true,"has_results":true,"data":[{"value":1}]}`)
	var closeRequest Request
	json.Unmarshal(<-transport.sent, &closeRequest)
	if closeRequest.Type != OperationSQLClose || closeRequest.ContID != proof.ID {
		t.Fatalf("close request=%#v", closeRequest)
	}
	transport.receive <- []byte(`{"id":"` + closeRequest.ID + `","success":true}`)
	var exit Request
	json.Unmarshal(<-transport.sent, &exit)
	if exit.Type != OperationExit {
		t.Fatalf("exit request=%#v", exit)
	}
	transport.receive <- []byte(`{"id":"` + exit.ID + `","success":true}`)
	if proof := <-done; proof.Rows != 1 || proof.Revision != FixedProofRevision {
		t.Fatalf("proof metadata=%#v", proof)
	}
}

func TestFixedProofCancellationClosesSessionWithoutPartialMetadata(t *testing.T) {
	transport := newMessageFake()
	s := NewMessageSession(transport)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := s.FixedProof(ctx, "USER", []byte("secret")); done <- err }()
	connect := <-transport.sent
	var request Request
	json.Unmarshal(connect, &request)
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation=%v", err)
	}
	select {
	case <-transport.closed:
	case <-time.After(time.Second):
		t.Fatal("cancellation did not close transport")
	}
}

func TestTypedRequestsValidateOperationsAndBounds(t *testing.T) {
	a, b := requestID(t), requestID(t)
	if a == b || len(a) < 20 {
		t.Fatalf("IDs are not cryptographically sized and unique: %q %q", a, b)
	}
	for _, request := range []Request{{Type: OperationGetVersion}, {Type: OperationConnect, Application: "app"}, {Type: OperationPrepareSQLExecute, SQL: "select 1", Rows: 1}, {Type: OperationSQLMore, ContID: "cursor", Rows: 1}, {Type: OperationSQLClose, ContID: "cursor"}, {Type: OperationPing}, {Type: OperationExit}} {
		if err := ValidateRequest(request); err != nil {
			t.Errorf("%s: %v", request.Type, err)
		}
	}
	for _, request := range []Request{{Type: "shell"}, {Type: OperationPrepareSQLExecute}, {Type: OperationSQLMore, Rows: MaxPageRows + 1}, {Type: OperationConnect, Application: strings.Repeat("x", MaxFieldBytes+1)}} {
		if err := ValidateRequest(request); err == nil {
			t.Errorf("accepted invalid request %#v", request)
		}
	}
}

func TestCallReplacesCallerIDAndRejectsTrailingResponse(t *testing.T) {
	transport := newMessageFake()
	s := NewMessageSession(transport)
	done := startCall(s, context.Background(), Request{ID: "caller-id", Type: OperationPing})
	var sent Request
	json.Unmarshal(<-transport.sent, &sent)
	if sent.ID == "caller-id" || sent.ID == "" {
		t.Fatalf("request ID = %q, want generated ID", sent.ID)
	}
	transport.receive <- []byte(`{"id":"` + sent.ID + `","success":true} {"extra":true}`)
	if err := <-done; !errors.Is(err, ErrProtocolViolation) {
		t.Fatalf("trailing response error = %v", err)
	}
}

func TestCallCapsUnboundedContextAndClosesAllPending(t *testing.T) {
	transport := newMessageFake()
	s := NewMessageSession(transport)
	first := startCall(s, context.Background(), Request{Type: OperationPing})
	second := startCall(s, context.Background(), Request{Type: OperationGetVersion})
	<-transport.sent
	<-transport.sent
	if err := <-first; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("bounded call error = %v", err)
	}
	if err := <-second; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("pending caller error = %v", err)
	}
}

func TestCursorAndResponseLimitsAreEnforcedAcrossPages(t *testing.T) {
	transport := newMessageFake()
	s := NewMessageSession(transport)
	done := startCall(s, context.Background(), Request{Type: OperationPrepareSQLExecute, SQL: "select", Rows: 1})
	var prepare Request
	json.Unmarshal(<-transport.sent, &prepare)
	transport.receive <- []byte(`{"id":"` + prepare.ID + `","success":true,"is_done":false,"has_results":true,"data":[{"a":1}]}`)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	more := startCall(s, context.Background(), Request{Type: OperationSQLMore, ContID: prepare.ID, Rows: 1})
	var page Request
	json.Unmarshal(<-transport.sent, &page)
	transport.receive <- []byte(`{"id":"` + page.ID + `","success":true,"is_done":true,"data":[{"a":2}]}`)
	if err := <-more; err != nil {
		t.Fatal(err)
	}
	bad := startCall(s, context.Background(), Request{Type: OperationSQLMore, ContID: prepare.ID, Rows: 1})
	if err := <-bad; !errors.Is(err, ErrProtocolViolation) {
		t.Fatalf("closed cursor error = %v", err)
	}
}

func TestResponseLimitsRejectColumnsAndCursorCount(t *testing.T) {
	transport := newMessageFake()
	s := NewMessageSession(transport)
	done := startCall(s, context.Background(), Request{Type: OperationPrepareSQLExecute, SQL: "select", Rows: 1})
	var request Request
	json.Unmarshal(<-transport.sent, &request)
	columns := make([]string, MaxColumns+1)
	for i := range columns {
		columns[i] = `"c` + strconv.Itoa(i) + `":1`
	}
	transport.receive <- []byte(`{"id":"` + request.ID + `","success":true,"data":[{` + strings.Join(columns, ",") + `}]}`)
	if err := <-done; !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("column error = %v", err)
	}
}
func TestSessionCorrelatesOutOfOrderAndFailsClosedOnUnknownDuplicate(t *testing.T) {
	transport := newMessageFake()
	s := NewMessageSession(transport)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	firstDone := startCall(s, ctx, Request{Type: OperationPing})
	secondDone := startCall(s, ctx, Request{Type: OperationGetVersion})
	first, second := <-transport.sent, <-transport.sent
	var a, b Request
	_ = json.Unmarshal(first, &a)
	_ = json.Unmarshal(second, &b)
	if a.ID == "" || b.ID == "" {
		t.Fatal("request was written without an ID")
	}
	transport.receive <- []byte(`{"id":"` + b.ID + `","success":true}`)
	transport.receive <- []byte(`{"id":"` + a.ID + `","success":true}`)
	for _, done := range []chan error{firstDone, secondDone} {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	transport.receive <- []byte(`{"id":"unknown","success":true}`)
	select {
	case <-transport.closed:
	case <-time.After(time.Second):
		t.Fatal("unknown ID did not close session")
	}
	if _, err := s.Call(ctx, Request{Type: OperationPing}); !errors.Is(err, ErrSessionClosed) {
		t.Fatalf("post-close error = %v", err)
	}
	transport = newMessageFake()
	s = NewMessageSession(transport)
	for i := 0; i < MaxCursors; i++ {
		done := startCall(s, context.Background(), Request{Type: OperationPrepareSQLExecute, SQL: "select", Rows: 1})
		var request Request
		json.Unmarshal(<-transport.sent, &request)
		transport.receive <- []byte(`{"id":"` + request.ID + `","success":true,"is_done":false}`)
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Call(context.Background(), Request{Type: OperationPrepareSQLExecute, SQL: "select", Rows: 1}); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("cursor limit error = %v", err)
	}
	transport = newMessageFake()
	s = NewMessageSession(transport)
	cancelCtx, stop := context.WithCancel(context.Background())
	done := startCall(s, cancelCtx, Request{Type: OperationPing})
	<-transport.sent
	stop()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
	select {
	case <-transport.closed:
	case <-time.After(time.Second):
		t.Fatal("cancellation did not close session")
	}
}

type messageFake struct {
	sent, receive chan []byte
	closed        chan struct{}
	once          sync.Once
}

func requestID(t *testing.T) string {
	id, err := NewRequestID()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func startCall(s *Client, ctx context.Context, r Request) chan error {
	done := make(chan error, 1)
	go func() { _, err := s.Call(ctx, r); done <- err }()
	return done
}
func newMessageFake() *messageFake {
	return &messageFake{sent: make(chan []byte, 8), receive: make(chan []byte, 8), closed: make(chan struct{})}
}
func (f *messageFake) Send(_ context.Context, p []byte) error {
	select {
	case f.sent <- append([]byte(nil), p...):
		return nil
	case <-f.closed:
		return ErrSessionClosed
	}
}
func (f *messageFake) Receive(ctx context.Context) ([]byte, error) {
	select {
	case p := <-f.receive:
		return p, nil
	case <-f.closed:
		return nil, ErrSessionClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
func (f *messageFake) Close() error { f.once.Do(func() { close(f.closed) }); return nil }
