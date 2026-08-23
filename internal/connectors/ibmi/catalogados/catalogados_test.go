package catalogados

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"bac-nexus/internal/catalog"
)

type fakeExecutor struct {
	statement  string
	rowLimit   int
	parameters []string
	rows       []map[string]json.RawMessage
	err        error
}

func (f *fakeExecutor) Execute(_ context.Context, statement string, rowLimit int, parameters []string) ([]map[string]json.RawMessage, error) {
	f.statement, f.rowLimit, f.parameters = statement, rowLimit, append([]string(nil), parameters...)
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
	if executor.rowLimit != catalog.MaxCandidates+1 || !strings.Contains(executor.statement, "FROM pde400.SAHDR") || strings.Contains(executor.statement, "PISA061") || strings.Join(executor.parameters, "|") != "%PISA061%|PRODLIB" {
		t.Fatalf("query = %q, %d, %v", executor.statement, executor.rowLimit, executor.parameters)
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
