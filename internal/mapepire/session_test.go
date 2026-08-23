package mapepire

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

type scriptedChannel struct {
	reader *bytes.Buffer
	writes bytes.Buffer
	closed bool
}

func (c *scriptedChannel) Read(p []byte) (int, error)  { return c.reader.Read(p) }
func (c *scriptedChannel) Write(p []byte) (int, error) { return c.writes.Write(p) }
func (c *scriptedChannel) Close() error                { c.closed = true; return nil }

func successFrames(rows string) string {
	return `{"id":"connect-1","success":true,"job":"123456/USER/JOB"}` + "\n" +
		`{"id":"query-1","success":true,"has_results":true,"is_done":true,"data":` + rows + `}` + "\n" +
		`{"id":"close-1","success":true}` + "\n" +
		`{"id":"exit-1","success":true}` + "\n"
}

func TestSessionReturnsGenericRows(t *testing.T) {
	rows := `[{"any_column":"value","number":42}]`
	channel := &scriptedChannel{reader: bytes.NewBufferString(successFrames(rows))}
	result, err := NewSession(channel).Execute(context.Background(), "select ?", 51, []string{"value"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || string(result[0]["number"]) != "42" {
		t.Fatalf("rows = %#v", result)
	}
	if !channel.closed || !strings.Contains(channel.writes.String(), `"type":"sqlclose"`) || !strings.Contains(channel.writes.String(), `"type":"exit"`) {
		t.Fatalf("session lifecycle incomplete: closed=%v writes=%s", channel.closed, channel.writes.String())
	}
}

func TestSessionRejectsOutOfRangeRowLimitsBeforeWrite(t *testing.T) {
	for _, rowLimit := range []int{0, -1, MaxQueryRows + 1} {
		t.Run(fmt.Sprintf("row limit %d", rowLimit), func(t *testing.T) {
			channel := &scriptedChannel{reader: bytes.NewBuffer(nil)}
			_, err := NewSession(channel).Execute(context.Background(), "select ?", rowLimit, []string{"value"})
			if !errors.Is(err, ErrInvalidQueryRowLimit) || err.Error() != "Mapepire query row limit must be between 1 and 1000" {
				t.Fatalf("error = %v", err)
			}
			if channel.writes.Len() != 0 {
				t.Fatalf("protocol bytes = %q, want none", channel.writes.String())
			}
		})
	}
}

func TestSessionIncludesValidBoundaryRowLimits(t *testing.T) {
	for _, rowLimit := range []int{1, MaxQueryRows} {
		t.Run(fmt.Sprintf("row limit %d", rowLimit), func(t *testing.T) {
			channel := &scriptedChannel{reader: bytes.NewBufferString(successFrames("[]"))}
			if _, err := NewSession(channel).Execute(context.Background(), "select ?", rowLimit, nil); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(channel.writes.String(), fmt.Sprintf(`"rows":%d`, rowLimit)) {
				t.Fatalf("query request omitted row limit: %s", channel.writes.String())
			}
		})
	}
}

func TestSessionMapsSQLStateWithoutRawServerError(t *testing.T) {
	frames := `{"id":"connect-1","success":true,"job":"job"}` + "\n" +
		`{"id":"query-1","success":false,"error":"sensitive SQL text","sql_state":"42704","sql_rc":-204}` + "\n"
	channel := &scriptedChannel{reader: bytes.NewBufferString(frames)}
	_, err := NewSession(channel).Execute(context.Background(), "fixed", 51, nil)
	var sqlErr *SQLError
	if !errors.As(err, &sqlErr) || sqlErr.State != "42704" || sqlErr.Code != -204 {
		t.Fatalf("error = %#v", err)
	}
	if strings.Contains(err.Error(), "sensitive") {
		t.Fatal("raw server error leaked")
	}
}

func TestSessionRequiresExplicitQueryCompletion(t *testing.T) {
	tests := []struct {
		name      string
		queryJSON string
		wantError bool
	}{
		{name: "missing is_done", queryJSON: `{"id":"query-1","success":true,"has_results":true,"data":[]}`, wantError: true},
		{name: "false is_done", queryJSON: `{"id":"query-1","success":true,"has_results":true,"is_done":false,"data":[]}`, wantError: true},
		{name: "true is_done", queryJSON: `{"id":"query-1","success":true,"has_results":true,"is_done":true,"data":[]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frames := `{"id":"connect-1","success":true,"job":"job"}` + "\n" + tt.queryJSON + "\n" +
				`{"id":"close-1","success":true}` + "\n" + `{"id":"exit-1","success":true}` + "\n"
			channel := &scriptedChannel{reader: bytes.NewBufferString(frames)}
			_, err := NewSession(channel).Execute(context.Background(), "fixed", 51, nil)
			if tt.wantError {
				if err == nil || !strings.Contains(err.Error(), "did not prove result-set completion") {
					t.Fatalf("error = %v, want completion rejection", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("error = %v, want completed response accepted", err)
			}
		})
	}
}

type blockingChannel struct {
	closed chan struct{}
	once   sync.Once
}

func (c *blockingChannel) Read([]byte) (int, error)    { <-c.closed; return 0, io.EOF }
func (c *blockingChannel) Write(p []byte) (int, error) { return len(p), nil }
func (c *blockingChannel) Close() error                { c.once.Do(func() { close(c.closed) }); return nil }

func TestSessionDeadlineClosesChannel(t *testing.T) {
	channel := &blockingChannel{closed: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := NewSession(channel).Execute(ctx, "fixed", 51, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want deadline exceeded", err)
	}
	select {
	case <-channel.closed:
	default:
		t.Fatal("channel remained open after deadline")
	}
}

func TestSessionRejectsMalformedOrPartialResponses(t *testing.T) {
	tests := []struct {
		name   string
		frames string
		want   string
	}{
		{name: "partial frame", frames: `{"id":"connect-1","success":true,"job":"job"}`, want: "before newline terminator"},
		{name: "malformed frame", frames: "not-json\n", want: "invalid character"},
		{name: "connect missing job", frames: `{"id":"connect-1","success":true}` + "\n", want: "omitted job identifier"},
		{name: "query missing result marker", frames: `{"id":"connect-1","success":true,"job":"job"}` + "\n" + `{"id":"query-1","success":true}` + "\n", want: "returned no result set"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := &scriptedChannel{reader: bytes.NewBufferString(tt.frames)}
			_, err := NewSession(channel).Execute(context.Background(), "fixed", 51, nil)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}
