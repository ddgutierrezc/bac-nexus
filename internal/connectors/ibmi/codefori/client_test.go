package codefori

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"bac-nexus/internal/provider"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
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

func TestClientPostsUnauthenticatedFixedLoopbackRequest(t *testing.T) {
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
		if decoded.Method != methodSQLQuery || decoded.Params.SQL != provider.CanonicalProofQuery {
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
		if decoded.Method != methodStatus || decoded.Params.SQL != "" {
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
