package catalogados

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"bac-nexus/internal/catalog"
	"bac-nexus/internal/mapepire"
)

type fakeExecutor struct {
	search catalog.Search
	rows   []map[string]json.RawMessage
	err    error
}

func (f *fakeExecutor) Resolve(_ context.Context, search catalog.Search) ([]map[string]json.RawMessage, error) {
	f.search = search
	return f.rows, f.err
}

func TestResolverOwnsParameterizedQueryAndMapping(t *testing.T) {
	executor := &fakeExecutor{rows: []map[string]json.RawMessage{{"ITEM": json.RawMessage(`" PISA061 "`), "TIPO_DE_FUENTE": json.RawMessage(`" RPGLE "`), "TIPO_OBJETO": json.RawMessage(`" RPGLE "`), "BIBLIOTECA_FUENTES": json.RawMessage(`" SRCLIB "`), "ARCHIVO_FUENTES": json.RawMessage(`" Q "`)}}}
	search, err := catalog.NewSearch("pisa061", "prodlib")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := (Resolver{Executor: executor}).Resolve(context.Background(), search)
	if err != nil {
		t.Fatal(err)
	}
	if executor.search != search {
		t.Fatalf("search = %#v", executor.search)
	}
	if len(rows) != 1 || rows[0].Item != "PISA061" || rows[0].SourceLibrary != "SRCLIB" {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestResolverRejectsMalformedRowsAndSentinel(t *testing.T) {
	search, _ := catalog.NewSearch("PISA061", "")
	for _, tt := range []struct {
		name string
		rows []map[string]json.RawMessage
		want error
	}{
		{"missing coordinates", []map[string]json.RawMessage{{"ITEM": json.RawMessage(`"PISA061"`)}}, nil},
		{"sentinel", completeRows(catalog.MaxCandidates + 1), catalog.ErrCandidateLimit},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := (Resolver{Executor: &fakeExecutor{rows: tt.rows}}).Resolve(context.Background(), search)
			if err == nil || (tt.want != nil && !errors.Is(err, tt.want)) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func completeRows(count int) []map[string]json.RawMessage {
	rows := make([]map[string]json.RawMessage, count)
	for i := range rows {
		rows[i] = map[string]json.RawMessage{"ITEM": json.RawMessage(`"ITEM"`), "TIPO_DE_FUENTE": json.RawMessage(`"RPG"`), "TIPO_OBJETO": json.RawMessage(`"RPG"`), "BIBLIOTECA_FUENTES": json.RawMessage(`"LIB"`), "ARCHIVO_FUENTES": json.RawMessage(`"Q"`)}
	}
	return rows
}

type authenticatedSessionFake struct {
	connectUser string
	connectPass []byte
	requests    []mapepire.Request
	responses   []mapepire.Response
	err         error
	closed      int
}

func (f *authenticatedSessionFake) Connect(_ context.Context, username string, password []byte) error {
	f.connectUser, f.connectPass = username, append([]byte(nil), password...)
	return f.err
}

func (f *authenticatedSessionFake) Call(_ context.Context, request mapepire.Request) (mapepire.Response, error) {
	f.requests = append(f.requests, request)
	if f.err != nil {
		return mapepire.Response{}, f.err
	}
	response := f.responses[0]
	f.responses = f.responses[1:]
	return response, nil
}

func (f *authenticatedSessionFake) Close() error { f.closed++; return nil }

func TestAuthenticatedExecutorConnectsExecutesFixedQueryAndCloses(t *testing.T) {
	session := &authenticatedSessionFake{responses: []mapepire.Response{
		{Success: true, HasResults: true, IsDone: true, ContID: "catalog-cursor", Data: completeRows(1)},
		{Success: true}, {Success: true},
	}}
	executor := NewAuthenticatedExecutor(func(context.Context) (AuthenticatedSession, string, []byte, error) {
		return session, "NEXUSUSR", []byte("secret"), nil
	})
	search, err := catalog.NewSearch("PISA061", "PRODLIB")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := executor.Resolve(context.Background(), search)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || session.connectUser != "NEXUSUSR" || string(session.connectPass) != "secret" || session.closed != 1 {
		t.Fatalf("rows=%d connect=%q closed=%d", len(rows), session.connectUser, session.closed)
	}
	statement, parameters, limit := PreparedSearch(search)
	if len(session.requests) != 3 || session.requests[0].Type != mapepire.OperationPrepareSQLExecute || session.requests[0].SQL != statement || session.requests[0].Rows != limit || strings.Join(session.requests[0].Parameters, "|") != strings.Join(parameters, "|") || session.requests[1].Type != mapepire.OperationSQLClose || session.requests[2].Type != mapepire.OperationExit {
		t.Fatalf("requests=%#v", session.requests)
	}
}

func TestAuthenticatedExecutorRejectsForgedSearchBeforeOpeningSession(t *testing.T) {
	for _, forged := range []catalog.Search{
		{Item: "PISA061'; DROP", ProductionLibrary: "PRODLIB"},
		{Item: "pisa061", ProductionLibrary: "PRODLIB"},
	} {
		t.Run(forged.Item, func(t *testing.T) {
			opened := 0
			executor := NewAuthenticatedExecutor(func(context.Context) (AuthenticatedSession, string, []byte, error) {
				opened++
				return nil, "", nil, errors.New("remote secret")
			})
			_, err := executor.Resolve(context.Background(), forged)
			if !errors.Is(err, ErrCatalogUnavailable) || opened != 0 || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), forged.Item) {
				t.Fatalf("err=%v opened=%d", err, opened)
			}
		})
	}
}

type fixedSessionFake struct {
	statement  string
	limit      int
	parameters []string
	rows       []map[string]json.RawMessage
}

func (f *fixedSessionFake) Execute(_ context.Context, statement string, limit int, parameters []string) ([]map[string]json.RawMessage, error) {
	f.statement, f.limit, f.parameters = statement, limit, append([]string(nil), parameters...)
	return f.rows, nil
}

func TestFixedSessionExecutorOwnsCatalogadosQuery(t *testing.T) {
	search, err := catalog.NewSearch("PISA061", "PRODLIB")
	if err != nil {
		t.Fatal(err)
	}
	session := &fixedSessionFake{rows: completeRows(1)}
	rows, err := NewFixedSessionExecutor(session).Resolve(context.Background(), search)
	if err != nil {
		t.Fatal(err)
	}
	statement, parameters, limit := PreparedSearch(search)
	if len(rows) != 1 || session.statement != statement || session.limit != limit || strings.Join(session.parameters, "|") != strings.Join(parameters, "|") {
		t.Fatalf("rows=%d statement=%q limit=%d parameters=%v", len(rows), session.statement, session.limit, session.parameters)
	}
}

func TestAuthenticatedExecutorSanitizesFailuresAndClosesOwnedSession(t *testing.T) {
	for _, tt := range []struct {
		name string
		ctx  context.Context
		err  error
		want error
	}{
		{"connect failure", context.Background(), errors.New("ssh ibmi.example.test USER secret"), ErrCatalogUnavailable},
		{"cancelled", context.Background(), context.Canceled, ErrCatalogCancelled},
	} {
		t.Run(tt.name, func(t *testing.T) {
			session := &authenticatedSessionFake{err: tt.err}
			executor := NewAuthenticatedExecutor(func(context.Context) (AuthenticatedSession, string, []byte, error) {
				return session, "USER", []byte("secret"), nil
			})
			search, err := catalog.NewSearch("PISA061", "")
			if err != nil {
				t.Fatal(err)
			}
			_, err = executor.Resolve(tt.ctx, search)
			if !errors.Is(err, tt.want) || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "ibmi.example.test") || session.closed != 1 {
				t.Fatalf("err=%v closed=%d", err, session.closed)
			}
		})
	}
}
