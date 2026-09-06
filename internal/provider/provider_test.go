package provider

import (
	"context"
	"strings"
	"testing"
	"time"
)

type fakeProvider struct {
	calls   int
	request QueryRequest
	result  QueryResult
	onQuery func(context.Context)
}

func (f *fakeProvider) SessionStatus(context.Context) SessionStatusResult {
	return SessionStatusResult{State: SessionConnected}
}

func (f *fakeProvider) Query(ctx context.Context, request QueryRequest) QueryResult {
	f.calls++
	f.request = request
	if f.onQuery != nil {
		f.onQuery(ctx)
	}
	return f.result
}

func TestCanonicalizeQuery(t *testing.T) {
	valid := []string{
		CanonicalProofQuery,
		"\tselect\r\ncurrent_user from sysibm.sysdummy1 \n",
		CanonicalProofQuery + strings.Repeat(" ", 87),
	}
	for _, input := range valid {
		t.Run("accepts", func(t *testing.T) {
			canonical, ok := CanonicalizeQuery(input)
			if !ok || canonical != CanonicalProofQuery {
				t.Fatalf("CanonicalizeQuery(%q) = %q, %v; want %q, true", input, canonical, ok, CanonicalProofQuery)
			}
			if len(canonical) != 41 {
				t.Fatalf("canonical query is %d bytes; want 41", len(canonical))
			}
		})
	}

	invalid := []string{
		"SELECT/*comment*/ CURRENT_USER FROM SYSIBM.SYSDUMMY1",
		CanonicalProofQuery + ";",
		"SELECT\u00a0CURRENT_USER FROM SYSIBM.SYSDUMMY1",
		"SELECT CURRENT_USER FROM SYSIBM .SYSDUMMY1",
		CanonicalProofQuery + " WHERE 1=1",
		"SELECT ? FROM SYSIBM.SYSDUMMY1",
		"SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1[]",
		"SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1\u00e9",
		CanonicalProofQuery + strings.Repeat(" ", 88),
	}
	for _, input := range invalid {
		t.Run("rejects", func(t *testing.T) {
			canonical, ok := CanonicalizeQuery(input)
			if ok || canonical != "" {
				t.Fatalf("CanonicalizeQuery(%q) = %q, %v; want empty, false", input, canonical, ok)
			}
		})
	}
}

func TestRunProofQueryValidatesBeforeCallingProvider(t *testing.T) {
	fake := &fakeProvider{result: QueryResult{
		State: QueryOK,
		Rows:  []NormalizedRow{{Value: "DAVID"}},
	}}

	result := RunProofQuery(context.Background(), fake, QueryRequest{SQL: " select CURRENT_USER\tFROM sysibm.sysdummy1"})
	if result.State != QueryOK || len(result.Rows) != 1 || result.Rows[0].Value != "DAVID" {
		t.Fatalf("RunProofQuery() = %#v; want one successful DAVID row", result)
	}
	if fake.calls != 1 || fake.request.SQL != CanonicalProofQuery {
		t.Fatalf("fake received %d calls with %q; want one canonical query", fake.calls, fake.request.SQL)
	}

	result = RunProofQuery(context.Background(), fake, QueryRequest{SQL: CanonicalProofQuery + ";"})
	if result.State != QueryInvalidQuery || len(result.Rows) != 0 {
		t.Fatalf("invalid query result = %#v; want invalid_query with no rows", result)
	}
	if fake.calls != 1 {
		t.Fatalf("invalid query called fake %d times; want no additional call", fake.calls)
	}

	fake.result = QueryResult{State: QueryOK, Rows: []NormalizedRow{{Value: "one"}, {Value: "two"}}}
	result = RunProofQuery(context.Background(), fake, QueryRequest{SQL: CanonicalProofQuery})
	if result.State != QueryFailed || len(result.Rows) != 0 {
		t.Fatalf("malformed provider result = %#v; want failed with no rows", result)
	}
}

func TestRunProofQueryMapsContextAndProviderAvailability(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*fakeProvider) (context.Context, context.CancelFunc)
		want  QueryState
		calls int
	}{
		{
			name: "deadline before provider",
			setup: func(_ *fakeProvider) (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 0)
			},
			want: QueryTimeout,
		},
		{
			name: "deadline during provider",
			setup: func(fake *fakeProvider) (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
				fake.onQuery = func(context.Context) { <-ctx.Done() }
				return ctx, cancel
			},
			want:  QueryTimeout,
			calls: 1,
		},
		{
			name: "explicit cancellation after provider call",
			setup: func(fake *fakeProvider) (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				fake.onQuery = func(context.Context) { cancel() }
				return ctx, cancel
			},
			want:  QueryCancelled,
			calls: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeProvider{result: QueryResult{State: QueryOK, Rows: []NormalizedRow{{Value: "DAVID"}}}}
			ctx, cancel := test.setup(fake)
			defer cancel()
			result := RunProofQuery(ctx, fake, QueryRequest{SQL: CanonicalProofQuery})
			if result.State != test.want || len(result.Rows) != 0 || fake.calls != test.calls {
				t.Fatalf("RunProofQuery() = %#v after %d calls; want %q with no rows after %d calls", result, fake.calls, test.want, test.calls)
			}
		})
	}

	result := RunProofQuery(context.Background(), nil, QueryRequest{SQL: CanonicalProofQuery})
	if result.State != QueryUnavailable || len(result.Rows) != 0 {
		t.Fatalf("nil provider result = %#v; want unavailable with no rows", result)
	}
}

func TestValidateQueryResult(t *testing.T) {
	if !ValidateQueryResult(QueryResult{State: QueryOK, Rows: []NormalizedRow{{Value: strings.Repeat("é", 128)}}}) {
		t.Fatal("valid 256-byte UTF-8 successful result was rejected")
	}
	for _, state := range []QueryState{QueryUnavailable, QueryInvalidQuery, QueryLimitExceeded, QueryTimeout, QueryCancelled, QueryFailed} {
		if !ValidateQueryResult(QueryResult{State: state}) {
			t.Fatalf("valid non-success state %q with no rows was rejected", state)
		}
	}

	invalid := []QueryResult{
		{State: QueryState("unknown")},
		{State: QueryOK},
		{State: QueryOK, Rows: []NormalizedRow{{Value: "one"}, {Value: "two"}}},
		{State: QueryOK, Rows: []NormalizedRow{{Value: strings.Repeat("x", 257)}}},
		{State: QueryOK, Rows: []NormalizedRow{{Value: string([]byte{0xff})}}},
		{State: QueryUnavailable, Rows: []NormalizedRow{{Value: "leak"}}},
	}
	for _, result := range invalid {
		if ValidateQueryResult(result) {
			t.Fatalf("invalid result %#v was accepted", result)
		}
	}
}

func TestStatesAreClosed(t *testing.T) {
	for _, state := range []SessionState{SessionConnected, SessionCompanionUnavailable, SessionCodeForIExtensionUnavailable, SessionConnectionUnavailable} {
		if !state.Valid() {
			t.Fatalf("session state %q was rejected", state)
		}
	}
	if SessionState("unknown").Valid() {
		t.Fatal("unknown session state was accepted")
	}
	if QueryState("unknown").Valid() {
		t.Fatal("unknown query state was accepted")
	}
}
