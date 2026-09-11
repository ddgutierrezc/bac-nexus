package codefori

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"bac-nexus/internal/catalog"
	"bac-nexus/internal/inspection"
	"bac-nexus/internal/provider"
)

func TestClientResolveCatalogAuthenticationRetries(t *testing.T) {
	first, second := testToken(t), testToken(t)
	search, _ := catalog.NewSearch("PISA061", "PRODLIB")
	for _, tt := range []struct {
		name      string
		status    int
		tokens    []string
		wantCalls int
		want      error
	}{
		{"rotated unauthorized", http.StatusUnauthorized, []string{first, second}, 2, nil},
		{"rotated forbidden", http.StatusForbidden, []string{first, second}, 2, nil},
		{"unchanged token", http.StatusUnauthorized, []string{first, first}, 1, ErrCatalogUnavailable},
		{"unchanged forbidden", http.StatusForbidden, []string{first, first}, 1, ErrCatalogUnavailable},
		{"missing token unauthorized", http.StatusUnauthorized, nil, 1, ErrCatalogUnavailable},
		{"missing token forbidden", http.StatusForbidden, nil, 1, ErrCatalogUnavailable},
		{"non-auth status", http.StatusInternalServerError, []string{first, second}, 1, ErrCatalogUnavailable},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls, reads := 0, 0
			var bodies, ids, tokens []string
			client := NewClient()
			client.tokens = tokenSourceFunc(func(context.Context) (string, bool) {
				if reads == len(tt.tokens) {
					return "", false
				}
				value := tt.tokens[reads]
				reads++
				return value, true
			})
			client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				body := mustReadAll(t, request.Body)
				decoded, err := decodeRequest(body)
				if err != nil || decoded.Method != methodResolveCatalog || decoded.Params["item"] != "PISA061" {
					t.Fatalf("catalog request = %#v, %v", decoded, err)
				}
				bodies, ids, tokens = append(bodies, string(body)), append(ids, decoded.RequestID), append(tokens, request.Header.Get(companionTokenHeader))
				if strings.Contains(request.URL.String(), first) || strings.Contains(string(body), first) || strings.Contains(request.URL.String(), second) || strings.Contains(string(body), second) {
					t.Fatal("token escaped dedicated header")
				}
				if calls == 1 {
					return &http.Response{StatusCode: tt.status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(""))}, nil
				}
				return catalogOKResponse(decoded.RequestID), nil
			})}
			result, err := client.ResolveCatalog(context.Background(), search)
			if !errors.Is(err, tt.want) || calls != tt.wantCalls || (tt.wantCalls == 2 && len(result) != 1) {
				t.Fatalf("ResolveCatalog() = %#v, %v; calls=%d", result, err, calls)
			}
			if tt.wantCalls == 2 && (tokens[0] != first || tokens[1] != second || bodies[0] != bodies[1] || ids[0] != ids[1]) {
				t.Fatalf("retry did not preserve request identity: tokens=%q ids=%q", tokens, ids)
			}
		})
	}
}

func TestClientResolveCatalogFailsClosed(t *testing.T) {
	search, _ := catalog.NewSearch("PISA061", "")
	for _, tt := range []struct {
		name, result string
		want         error
	}{
		{"not found", `{"state":"ok","candidates":[]}`, catalog.ErrCandidateNotFound}, {"limit", `{"state":"candidate_limit_exceeded"}`, catalog.ErrCandidateLimit}, {"unavailable", `{"state":"unavailable"}`, ErrCatalogUnavailable}, {"malformed", `{"state":"ok","candidates":null}`, ErrCatalogFailed},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient()
			client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				decoded, _ := decodeRequest(mustReadAll(t, request.Body))
				body := `{"version":1,"request_id":"` + decoded.RequestID + `","result":` + tt.result + `}`
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			if _, err := client.ResolveCatalog(context.Background(), search); !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	client := NewClient()
	client.httpClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { calls++; return nil, errors.New("unexpected HTTP call") })}
	if _, err := client.ResolveCatalog(ctx, search); !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatalf("cancelled ResolveCatalog error = %v", err)
	}
}

func TestClientResolveCatalogRejectsForgedSearchBeforeHTTP(t *testing.T) {
	client := NewClient()
	client.httpClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { t.Fatal("forged search reached HTTP"); return nil, nil })}
	for _, search := range []catalog.Search{{Item: "bad name"}, {Item: "PISA061", ProductionLibrary: "bad name"}} {
		if _, err := client.ResolveCatalog(context.Background(), search); !errors.Is(err, ErrCatalogFailed) {
			t.Fatalf("ResolveCatalog(%#v) error = %v", search, err)
		}
	}
}

func TestClientResolveCatalogHTTPFailuresFailClosed(t *testing.T) {
	search, _ := catalog.NewSearch("PISA061", "")
	for _, tt := range []struct {
		name string
		body func(string) string
		want error
	}{
		{"mismatched ID", func(string) string { return `{"version":1,"request_id":"other","result":{"state":"unavailable"}}` }, ErrCatalogUnavailable},
		{"mismatched version", func(id string) string {
			return `{"version":2,"request_id":"` + id + `","result":{"state":"unavailable"}}`
		}, ErrCatalogUnavailable},
		{"malformed envelope", func(string) string { return `{"version":1,"result":{"state":"unavailable"}}` }, ErrCatalogUnavailable},
		{"catalog overflow", func(string) string { return strings.Repeat("x", maxCatalogResponseBytes+1) }, ErrCatalogFailed},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			client := NewClient()
			client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				decoded, err := decodeRequest(mustReadAll(t, request.Body))
				if err != nil {
					t.Fatal(err)
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(tt.body(decoded.RequestID)))}, nil
			})}
			if _, err := client.ResolveCatalog(context.Background(), search); !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
			if calls != 1 {
				t.Fatalf("HTTP calls = %d, want 1", calls)
			}
		})
	}
}

func TestClientResolveCatalogCancellationBoundaries(t *testing.T) {
	search, _ := catalog.NewSearch("PISA061", "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	calls := 0
	client := NewClient()
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		close(started)
		<-request.Context().Done()
		return nil, request.Context().Err()
	})}
	completed := make(chan error, 1)
	go func() { _, err := client.ResolveCatalog(ctx, search); completed <- err }()
	<-started
	cancel()
	if err := <-completed; !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("in-flight cancellation error = %v; calls=%d", err, calls)
	}
	deadline, stop := context.WithTimeout(context.Background(), time.Millisecond)
	defer stop()
	client = NewClient()
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, request.Context().Err()
	})}
	if _, err := client.ResolveCatalog(deadline, search); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline error = %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type tokenSourceFunc func(context.Context) (string, bool)

func (f tokenSourceFunc) Token(ctx context.Context) (string, bool) { return f(ctx) }

type targetSourceFunc func(context.Context) (companionTarget, bool)

func (f targetSourceFunc) Token(ctx context.Context) (string, bool) {
	target, ok := f(ctx)
	return target.token, ok
}
func (f targetSourceFunc) Target(ctx context.Context) (companionTarget, bool) { return f(ctx) }

func TestClientDoesNotRefreshAcrossEndpointChange(t *testing.T) {
	calls, reads := 0, 0
	client := NewClient()
	client.tokens = targetSourceFunc(func(context.Context) (companionTarget, bool) {
		reads++
		if reads == 1 {
			return companionTarget{endpoint: "http://127.0.0.1:41001", token: testToken(t), instance: "instance", generation: 1}, true
		}
		return companionTarget{endpoint: "http://127.0.0.1:41002", token: testToken(t), instance: "instance", generation: 1}, true
	})
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusUnauthorized, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(""))}, nil
	})}
	result := client.Query(context.Background(), provider.QueryRequest{SQL: provider.CanonicalProofQuery})
	if result.State != provider.QueryUnavailable || calls != 1 {
		t.Fatalf("Query() = %#v; calls=%d", result, calls)
	}
}

func TestClientRejectsInvalidQueryBeforeHTTP(t *testing.T) {
	requests := 0
	client := NewClient()
	client.httpClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return nil, nil
	})}

	result := client.Query(context.Background(), provider.QueryRequest{SQL: "SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1;"})
	if result.State != provider.QueryInvalidQuery || len(result.Rows) != 0 {
		t.Fatalf("Query() = %#v, want invalid_query", result)
	}
	if requests != 0 {
		t.Fatalf("HTTP requests = %d, want 0", requests)
	}
}

func TestClientPreservesUnauthenticatedCompatibilityWhenTokenStateIsAbsent(t *testing.T) {
	client := NewClient()
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", request.Method)
		}
		if request.URL.String() != fixedRPCURL {
			t.Fatalf("URL = %q, want %q", request.URL.String(), fixedRPCURL)
		}
		if got := request.Header.Get("Authorization"); got != "" {
			t.Fatalf("Authorization = %q, want empty", got)
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := decodeRequest(body)
		if err != nil {
			t.Fatal(err)
		}
		if decoded.Method != methodSQLQuery || decoded.Params["sql"] != provider.CanonicalProofQuery {
			t.Fatalf("request = %#v, want canonical sql.query", decoded)
		}
		if strings.Contains(string(body), "bearer") || strings.Contains(string(body), "token") || strings.Contains(string(body), "generation") {
			t.Fatalf("request body contains removed authentication field: %s", body)
		}
		return jsonResponse(t, rpcResponse{
			Version:   protocolVersion,
			RequestID: decoded.RequestID,
			Result:    provider.QueryResult{State: provider.QueryOK, Rows: []provider.NormalizedRow{{Value: "BACUSER"}}},
		}), nil
	})}

	result := client.Query(context.Background(), provider.QueryRequest{SQL: " select\tcurrent_user\r\nfrom sysibm.sysdummy1 "})
	want := provider.QueryResult{State: provider.QueryOK, Rows: []provider.NormalizedRow{{Value: "BACUSER"}}}
	if !sameQueryResult(result, want) {
		t.Fatalf("Query() = %#v, want %#v", result, want)
	}
}

func TestClientInjectsOnlyValidPrivateTokenStateIntoDedicatedHeader(t *testing.T) {
	token := testToken(t)
	client := NewClient()
	client.tokens = tokenSourceFunc(func(context.Context) (string, bool) { return token, true })
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get(companionTokenHeader) != token {
			t.Fatal("request omitted valid Companion token header")
		}
		body := mustReadAll(t, request.Body)
		if strings.Contains(request.URL.String(), token) || strings.Contains(string(body), token) {
			t.Fatal("request placed Companion token outside its dedicated header")
		}
		request.Body = io.NopCloser(bytes.NewReader(body))
		return successfulQueryResponse(t, request), nil
	})}
	result := client.Query(context.Background(), provider.QueryRequest{SQL: provider.CanonicalProofQuery})
	if result.State != provider.QueryOK || strings.Contains(fmt.Sprintf("%#v", result), token) {
		t.Fatal("client result did not preserve token redaction")
	}
}

func TestClientRefreshesTokenOnlyForExplicitAuthenticationRejection(t *testing.T) {
	first, second := testToken(t), testToken(t)
	for _, tt := range []struct {
		name       string
		tokens     []string
		statusCode int
		wantCalls  int
	}{
		{"rotated token retries once", []string{first, second}, http.StatusUnauthorized, 2},
		{"unchanged token does not retry", []string{first, first}, http.StatusForbidden, 1},
		{"non authentication failure does not retry", []string{first, second}, http.StatusInternalServerError, 1},
		{"missing token does not retry", nil, http.StatusUnauthorized, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			reads, calls := 0, 0
			client := NewClient()
			client.tokens = tokenSourceFunc(func(context.Context) (string, bool) {
				if reads >= len(tt.tokens) {
					return "", false
				}
				value := tt.tokens[reads]
				reads++
				return value, true
			})
			client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return &http.Response{StatusCode: tt.statusCode, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(""))}, nil
				}
				if request.Header.Get(companionTokenHeader) != second {
					t.Fatal("retry did not use the refreshed token")
				}
				return successfulQueryResponse(t, request), nil
			})}
			result := client.Query(context.Background(), provider.QueryRequest{SQL: provider.CanonicalProofQuery})
			if result.State != provider.QueryUnavailable && result.State != provider.QueryOK {
				t.Fatalf("Query() state = %q, want bounded result", result.State)
			}
			if calls != tt.wantCalls {
				t.Fatalf("HTTP calls = %d, want %d", calls, tt.wantCalls)
			}
		})
	}
}

func TestClientCancellationPreventsTokenReadAndHTTP(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reads, calls := 0, 0
	client := NewClient()
	client.tokens = tokenSourceFunc(func(context.Context) (string, bool) { reads++; return testToken(t), true })
	client.httpClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { calls++; return nil, nil })}
	if got := client.SessionStatus(ctx); got.State != provider.SessionCompanionUnavailable {
		t.Fatalf("SessionStatus() = %#v, want companion_unavailable", got)
	}
	if reads != 0 || calls != 0 {
		t.Fatalf("token reads=%d HTTP calls=%d, want zero", reads, calls)
	}
}

func TestClientRejectsMismatchedAndOversizedResponsesWithoutRetry(t *testing.T) {
	tests := []struct {
		name     string
		response func(*http.Request) *http.Response
		want     provider.QueryState
	}{
		{
			name: "mismatched version",
			response: func(request *http.Request) *http.Response {
				decoded, err := decodeRequest(mustReadAll(t, request.Body))
				if err != nil {
					t.Fatal(err)
				}
				return jsonResponse(t, rpcResponse{Version: protocolVersion + 1, RequestID: decoded.RequestID, Result: provider.QueryResult{State: provider.QueryOK, Rows: []provider.NormalizedRow{{Value: "BACUSER"}}}})
			},
			want: provider.QueryUnavailable,
		},
		{
			name: "mismatched request ID",
			response: func(request *http.Request) *http.Response {
				_, err := decodeRequest(mustReadAll(t, request.Body))
				if err != nil {
					t.Fatal(err)
				}
				return jsonResponse(t, rpcResponse{Version: protocolVersion, RequestID: "another-request", Result: provider.QueryResult{State: provider.QueryOK, Rows: []provider.NormalizedRow{{Value: "BACUSER"}}}})
			},
			want: provider.QueryUnavailable,
		},
		{
			name: "oversized body",
			response: func(*http.Request) *http.Response {
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(strings.Repeat("x", maxResponseBytes+1)))}
			},
			want: provider.QueryFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			client := NewClient()
			client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				return tt.response(request), nil
			})}

			result := client.Query(context.Background(), provider.QueryRequest{SQL: provider.CanonicalProofQuery})
			if result.State != tt.want || len(result.Rows) != 0 {
				t.Fatalf("Query() = %#v, want state %q without rows", result, tt.want)
			}
			if calls != 1 {
				t.Fatalf("HTTP calls = %d, want exactly one", calls)
			}
		})
	}
}

func TestClientAcceptsCorrelatedCompanionProgramResponseAndRejectsUncorrelatedResponse(t *testing.T) {
	tests := []struct {
		name          string
		omitRequestID bool
		want          inspection.State
	}{
		{
			name: "correlated Companion response",
			want: inspection.StateNotFound,
		},
		{
			name:          "Companion response missing request ID",
			omitRequestID: true,
			want:          inspection.StateUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient()
			client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				decoded, err := decodeRequest(mustReadAll(t, request.Body))
				if err != nil {
					t.Fatal(err)
				}
				if decoded.Method != methodResolveProgram {
					t.Fatalf("method = %q, want %q", decoded.Method, methodResolveProgram)
				}
				result := `{"state":"not_found","searchStrategy":"code_for_i_configured_context","librariesSearched":[],"matches":[],"completeness":"complete","truncated":false,"runtimeLiblVerified":false}`
				body := `{"version":1,"request_id":"` + decoded.RequestID + `","result":` + result + `}`
				if tt.omitRequestID {
					body = `{"version":1,"result":` + result + `}`
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			})}

			result := client.ResolveProgram(context.Background(), inspection.ResolveRequest{Name: "PISA061"})
			if result.State != tt.want {
				t.Fatalf("ResolveProgram() = %#v, want state %q", result, tt.want)
			}
			if tt.want == inspection.StateUnavailable && result.Reason != "companion_unavailable" {
				t.Fatalf("ResolveProgram() = %#v, want companion_unavailable", result)
			}
		})
	}
}

func TestClientMapsTransportAbsenceToBoundedStates(t *testing.T) {
	client := NewClient()
	client.httpClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("unreachable")
	})}
	if got := client.Query(context.Background(), provider.QueryRequest{SQL: provider.CanonicalProofQuery}); got.State != provider.QueryUnavailable || len(got.Rows) != 0 {
		t.Fatalf("Query() = %#v, want unavailable without rows", got)
	}
	if got := client.SessionStatus(context.Background()); got.State != provider.SessionCompanionUnavailable {
		t.Fatalf("SessionStatus() = %#v, want companion_unavailable", got)
	}
}

func TestClientUsesBoundedDeadlinesAndFixedResponseHeaderTimeout(t *testing.T) {
	client := NewClient()
	transport, ok := client.httpClient.Transport.(*http.Transport)
	if !ok || transport.ResponseHeaderTimeout < responseHeaderTimeout {
		t.Fatalf("response header timeout = %v, want at least %v", transport.ResponseHeaderTimeout, responseHeaderTimeout)
	}
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		deadline, ok := request.Context().Deadline()
		if !ok {
			t.Fatal("request context has no deadline")
		}
		remaining := time.Until(deadline)
		if remaining > queryTimeout || remaining < queryTimeout-time.Second {
			t.Fatalf("query deadline remaining = %v, want approximately %v", remaining, queryTimeout)
		}
		decoded, err := decodeRequest(mustReadAll(t, request.Body))
		if err != nil {
			t.Fatal(err)
		}
		return jsonResponse(t, rpcResponse{Version: protocolVersion, RequestID: decoded.RequestID, Result: provider.QueryResult{State: provider.QueryOK, Rows: []provider.NormalizedRow{{Value: "BACUSER"}}}}), nil
	})}
	if got := client.Query(context.Background(), provider.QueryRequest{SQL: provider.CanonicalProofQuery}); got.State != provider.QueryOK {
		t.Fatalf("Query() = %#v, want ok", got)
	}
}

func TestClientStatusUsesFixedMethodAndOneSecondDeadline(t *testing.T) {
	client := NewClient()
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		deadline, ok := request.Context().Deadline()
		if !ok {
			t.Fatal("request context has no deadline")
		}
		remaining := time.Until(deadline)
		if remaining > statusTimeout || remaining < statusTimeout-time.Second/2 {
			t.Fatalf("status deadline remaining = %v, want approximately %v", remaining, statusTimeout)
		}
		decoded, err := decodeRequest(mustReadAll(t, request.Body))
		if err != nil {
			t.Fatal(err)
		}
		if decoded.Method != methodStatus || decoded.Params["sql"] != "" {
			t.Fatalf("status request = %#v, want session.status with empty params", decoded)
		}
		body := `{"version":1,"request_id":"` + decoded.RequestID + `","result":{"state":"connected"}}`
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}

	if got := client.SessionStatus(context.Background()); got.State != provider.SessionConnected {
		t.Fatalf("SessionStatus() = %#v, want connected", got)
	}
}

func TestNilHTTPClientFailsClosedWithoutHTTP(t *testing.T) {
	client := NewClient()
	client.httpClient = nil
	if got := client.Query(context.Background(), provider.QueryRequest{SQL: provider.CanonicalProofQuery}); got.State != provider.QueryUnavailable || len(got.Rows) != 0 {
		t.Fatalf("Query() = %#v, want unavailable without rows", got)
	}
	if got := client.SessionStatus(context.Background()); got.State != provider.SessionCompanionUnavailable {
		t.Fatalf("SessionStatus() = %#v, want companion_unavailable", got)
	}
}

func jsonResponse(t *testing.T, response rpcResponse) *http.Response {
	t.Helper()
	body, err := encodeResponse(response)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
}

func successfulQueryResponse(t *testing.T, request *http.Request) *http.Response {
	t.Helper()
	decoded, err := decodeRequest(mustReadAll(t, request.Body))
	if err != nil {
		t.Fatal(err)
	}
	return jsonResponse(t, rpcResponse{Version: protocolVersion, RequestID: decoded.RequestID, Result: provider.QueryResult{State: provider.QueryOK, Rows: []provider.NormalizedRow{{Value: "BACUSER"}}}})
}

func catalogOKResponse(requestID string) *http.Response {
	body := `{"version":1,"request_id":"` + requestID + `","result":{"state":"ok","candidates":[{"item":"PISA061","sourceLibrary":"SRCLIB","sourceFileBase":"QRPG","objectType":"M","sourceType":"RPGLE","application":"APP","version":"V1","productionLibrary":"PRODLIB","description":"description"}]}}`
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func sameQueryResult(got, want provider.QueryResult) bool {
	if got.State != want.State || len(got.Rows) != len(want.Rows) {
		return false
	}
	for index := range got.Rows {
		if got.Rows[index] != want.Rows[index] {
			return false
		}
	}
	return true
}

func mustReadAll(t *testing.T, body io.Reader) []byte {
	t.Helper()
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
