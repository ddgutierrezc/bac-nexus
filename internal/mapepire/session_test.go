package mapepire

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"bac-nexus/internal/catalog"
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

func TestSessionMapsPinnedResponseShape(t *testing.T) {
	rows := `[{"ITEM":"PISA061","TIPO_DE_FUENTE":"RPGLE","TIPO_OBJETO":"RPGLE","APLICACION":"APP","VERSION":"1","BIBLIOTECA_PRODUCCION":"PRODLIB","BIBLIOTECA_FUENTES":"SRCLIB","ARCHIVO_FUENTES":"Q","DESCRIPCION":"Program"}]`
	channel := &scriptedChannel{reader: bytes.NewBufferString(successFrames(rows))}
	query, err := catalog.BuildQuery("PISA061", "")
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := NewSession(channel).Catalog(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].SourceLibrary != "SRCLIB" {
		t.Fatalf("candidates = %#v", candidates)
	}
	if !channel.closed || !strings.Contains(channel.writes.String(), `"type":"sqlclose"`) || !strings.Contains(channel.writes.String(), `"type":"exit"`) {
		t.Fatalf("session lifecycle incomplete: closed=%v writes=%s", channel.closed, channel.writes.String())
	}
}

func TestSessionMapsSQLStateWithoutRawServerError(t *testing.T) {
	frames := `{"id":"connect-1","success":true,"job":"job"}` + "\n" +
		`{"id":"query-1","success":false,"error":"sensitive SQL text","sql_state":"42704","sql_rc":-204}` + "\n"
	channel := &scriptedChannel{reader: bytes.NewBufferString(frames)}
	_, err := NewSession(channel).Catalog(context.Background(), catalog.Query{Statement: "fixed", RowLimit: 51})
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
			_, err := NewSession(channel).Catalog(context.Background(), catalog.Query{Statement: "fixed", RowLimit: 51})
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

func TestSessionRejectsCandidateSentinel(t *testing.T) {
	row := `{"ITEM":"ITEM","TIPO_DE_FUENTE":"RPG","TIPO_OBJETO":"RPG","BIBLIOTECA_FUENTES":"LIB","ARCHIVO_FUENTES":"Q"}`
	channel := &scriptedChannel{reader: bytes.NewBufferString(successFrames("[" + strings.TrimSuffix(strings.Repeat(row+",", 51), ",") + "]"))}
	_, err := NewSession(channel).Catalog(context.Background(), catalog.Query{Statement: "fixed", RowLimit: 51})
	if !errors.Is(err, catalog.ErrCandidateLimit) {
		t.Fatalf("error = %v, want candidate limit", err)
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
	_, err := NewSession(channel).Catalog(ctx, catalog.Query{Statement: "fixed", RowLimit: 51})
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
		{name: "partial candidate", frames: successFrames(`[{"ITEM":"ITEM"}]`), want: "omitted required Catalogados coordinates"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := &scriptedChannel{reader: bytes.NewBufferString(tt.frames)}
			_, err := NewSession(channel).Catalog(context.Background(), catalog.Query{Statement: "fixed", RowLimit: 51})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}
