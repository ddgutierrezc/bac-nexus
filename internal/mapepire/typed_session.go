package mapepire

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"
)

type MessageTransport interface {
	Send(context.Context, []byte) error
	Receive(context.Context) ([]byte, error)
	Close() error
}
type Client struct {
	transport   MessageTransport
	pending     map[string]pendingCall
	cursors     map[string]int
	mu, write   sync.Mutex
	closed      chan struct{}
	once        sync.Once
	reader      sync.Once
	created     time.Time
	application string
}
type callResult struct {
	response Response
	err      error
}

const FixedProofSQL = "VALUES 1"
const FixedProofRevision = "values-1-v1"

type ProofMetadata struct {
	Rows     int
	Revision string
}
type pendingCall struct {
	result  chan callResult
	request Request
}

func NewMessageSession(t MessageTransport, application ...string) *Client {
	app := "BAC Nexus"
	if len(application) > 0 && application[0] != "" {
		app = application[0]
	}
	return &Client{transport: t, pending: map[string]pendingCall{}, cursors: map[string]int{}, closed: make(chan struct{}), created: time.Now(), application: app}
}
func (c *Client) start() { c.reader.Do(func() { go c.read() }) }
func (c *Client) read() {
	for {
		p, e := c.transport.Receive(context.Background())
		if e != nil {
			c.fail(e)
			return
		}
		if len(p) > MaxFrameBytes {
			c.fail(ErrLimitExceeded)
			return
		}
		var r Response
		d := json.NewDecoder(bytes.NewReader(p))
		d.DisallowUnknownFields()
		if e = d.Decode(&r); e != nil || r.ID == "" {
			c.fail(ErrProtocolViolation)
			return
		}
		var extra any
		if e = d.Decode(&extra); e != io.EOF {
			c.fail(ErrProtocolViolation)
			return
		}
		if len(r.Data) > MaxPageRows {
			c.fail(ErrLimitExceeded)
			return
		}
		c.mu.Lock()
		call, ok := c.pending[r.ID]
		c.mu.Unlock()
		if !ok {
			c.fail(ErrProtocolViolation)
			return
		}
		if err := c.acceptResponse(call.request, r, len(p)); err != nil {
			c.fail(err)
			return
		}
		if !r.Success {
			c.fail(&SQLError{State: r.SQLState, Code: r.SQLCode})
			return
		}
		c.mu.Lock()
		delete(c.pending, r.ID)
		c.mu.Unlock()
		call.result <- callResult{response: r}
	}
}
func (c *Client) Call(ctx context.Context, r Request) (Response, error) {
	if e := ValidateRequest(r); e != nil {
		return Response{}, e
	}
	deadline := c.created.Add(MaxSessionLifetime)
	if !time.Now().Before(deadline) {
		c.fail(ErrLimitExceeded)
		return Response{}, ErrLimitExceeded
	}
	var e error
	r.ID, e = NewRequestID()
	if e != nil {
		return Response{}, e
	}
	callCtx, cancel := boundedContext(ctx, deadline)
	defer cancel()
	if err := callCtx.Err(); err != nil {
		c.fail(err)
		return Response{}, err
	}
	c.start()
	w := make(chan callResult, 1)
	c.mu.Lock()
	if len(c.pending) >= MaxPendingRequests {
		c.mu.Unlock()
		return Response{}, ErrLimitExceeded
	}
	select {
	case <-c.closed:
		c.mu.Unlock()
		return Response{}, ErrSessionClosed
	default:
	}
	if _, ok := c.pending[r.ID]; ok {
		c.mu.Unlock()
		c.fail(ErrProtocolViolation)
		return Response{}, ErrProtocolViolation
	}
	if r.Type == OperationSQLMore {
		if _, ok := c.cursors[r.ContID]; !ok {
			c.mu.Unlock()
			return Response{}, ErrProtocolViolation
		}
	} else if r.Type == OperationSQLClose {
		if _, ok := c.cursors[r.ContID]; !ok {
			c.mu.Unlock()
			return Response{}, ErrProtocolViolation
		}
	} else if r.Type == OperationPrepareSQLExecute {
		if len(c.cursors) >= MaxCursors {
			c.mu.Unlock()
			return Response{}, ErrLimitExceeded
		}
		c.cursors[r.ID] = 0
	}
	c.pending[r.ID] = pendingCall{result: w, request: r}
	c.mu.Unlock()
	p, _ := json.Marshal(r)
	c.write.Lock()
	e = c.transport.Send(callCtx, p)
	c.write.Unlock()
	if e != nil {
		c.fail(e)
		return Response{}, contextError(callCtx, e)
	}
	select {
	case x := <-w:
		return x.response, x.err
	case <-callCtx.Done():
		c.fail(callCtx.Err())
		return Response{}, callCtx.Err()
	case <-c.closed:
		select {
		case x := <-w:
			return x.response, x.err
		default:
			return Response{}, ErrSessionClosed
		}
	}
}

func (c *Client) Connect(ctx context.Context, username string, password []byte) error {
	if username == "" || len(password) == 0 {
		return ErrProtocolViolation
	}
	r, err := c.Call(ctx, AuthenticatedConnectRequest("", c.application, username, password))
	if err != nil {
		return err
	}
	if r.Job == "" {
		c.Close()
		return ErrProtocolViolation
	}
	return nil
}

func (c *Client) FixedProof(ctx context.Context, username string, password []byte) (ProofMetadata, error) {
	defer c.Close()
	if err := c.Connect(ctx, username, password); err != nil {
		return ProofMetadata{}, err
	}
	r, err := c.Call(ctx, Request{Type: OperationPrepareSQLExecute, SQL: FixedProofSQL, Rows: 1})
	if err != nil {
		return ProofMetadata{}, err
	}
	if !r.HasResults || !r.IsDone || len(r.Data) != 1 {
		return ProofMetadata{}, ErrProtocolViolation
	}
	// A completed one-page response may already have removed its cursor from
	// the general paging state; the fixed proof still closes it explicitly.
	c.mu.Lock()
	if _, ok := c.cursors[r.ID]; !ok {
		c.cursors[r.ID] = 0
	}
	c.mu.Unlock()
	if _, err := c.Call(ctx, CloseCursorRequest("", r.ID)); err != nil {
		return ProofMetadata{}, err
	}
	if _, err := c.Call(ctx, ExitRequest("")); err != nil {
		return ProofMetadata{}, err
	}
	return ProofMetadata{Rows: 1, Revision: FixedProofRevision}, nil
}
func boundedContext(parent context.Context, sessionDeadline time.Time) (context.Context, context.CancelFunc) {
	deadline := time.Now().Add(MaxRequestTimeout)
	if sessionDeadline.Before(deadline) {
		deadline = sessionDeadline
	}
	if parentDeadline, ok := parent.Deadline(); ok && parentDeadline.Before(deadline) {
		deadline = parentDeadline
	}
	return context.WithDeadline(parent, deadline)
}
func (c *Client) acceptResponse(req Request, r Response, size int) error {
	if len(r.Data) > 0 {
		for _, row := range r.Data {
			if len(row) > MaxColumns {
				return ErrLimitExceeded
			}
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if req.Type == OperationPrepareSQLExecute || req.Type == OperationSQLMore {
		id := req.ID
		if req.Type == OperationSQLMore {
			id = req.ContID
		}
		c.cursors[id] += size
		if c.cursors[id] > MaxAggregateBytes {
			return ErrLimitExceeded
		}
		if !r.Success || r.IsDone {
			delete(c.cursors, id)
		}
	} else if req.Type == OperationSQLClose && r.Success {
		delete(c.cursors, req.ContID)
	}
	return nil
}
func (c *Client) fail(e error) {
	c.once.Do(func() {
		c.mu.Lock()
		for id, call := range c.pending {
			delete(c.pending, id)
			if errors.Is(e, context.Canceled) || errors.Is(e, context.DeadlineExceeded) || errors.Is(e, ErrLimitExceeded) {
				call.result <- callResult{err: e}
			} else {
				call.result <- callResult{err: ErrProtocolViolation}
			}
		}
		c.mu.Unlock()
		close(c.closed)
		_ = c.transport.Close()
	})
}
func (c *Client) Close() error { c.fail(ErrSessionClosed); return nil }
