package codefori

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"bac-nexus/internal/catalog"
)

func TestClientPageSourceDoesNotRefreshAcrossEndpointChange(t *testing.T) {
	candidate := sourceCandidate()
	calls, reads := 0, 0
	client := NewClient()
	client.tokens = targetSourceFunc(func(context.Context) (companionTarget, bool) {
		reads++
		if reads == 1 {
			return companionTarget{endpoint: "http://127.0.0.1:41001", token: testToken(t), instance: "instance", generation: 1}, true
		}
		return companionTarget{endpoint: "http://127.0.0.1:41002", token: testToken(t), instance: "instance", generation: 1}, true
	})
	client.httpClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusUnauthorized, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(""))}, nil
	})}
	if _, err := client.PageSource(context.Background(), SourcePageRequest{Candidate: &candidate, StartLine: 1, MaxLines: 1}); !errors.Is(err, ErrSourceUnavailable) || calls != 1 {
		t.Fatalf("PageSource() error=%v calls=%d", err, calls)
	}
}

func TestClientPageSourceCarriesCandidateOnlyOnFirstPage(t *testing.T) {
	candidate, cursor := sourceCandidate(), strings.Repeat("A", 43)
	requests := make([]map[string]any, 0, 2)
	client := sourceTestClient(t, func(request *http.Request, body map[string]any) *http.Response {
		requests = append(requests, body)
		if len(requests) == 2 {
			return sourceResponse(body["request_id"].(string), `{"state":"ok","cursor":"`+cursor+`","page":{"content":"line","start_line":2,"line_count":1,"eof":false}}`)
		}
		return sourceResponse(body["request_id"].(string), `{"state":"ok","cursor":"`+cursor+`","page":{"content":"line","start_line":1,"line_count":1,"eof":false}}`)
	})
	first, err := client.PageSource(context.Background(), SourcePageRequest{Candidate: &candidate, StartLine: 1, MaxLines: 200})
	if err != nil || first.State != SourceOK || first.Page.Content != "line" || first.Cursor != cursor {
		t.Fatalf("first page = %#v, %v", first, err)
	}
	_, err = client.PageSource(context.Background(), SourcePageRequest{Cursor: cursor, StartLine: 2, MaxLines: 1})
	if err != nil || len(requests) != 2 || requests[0]["method"] != methodSourcePage || requests[1]["method"] != methodSourcePage {
		t.Fatalf("page requests = %#v, %v", requests, err)
	}
	if params := requests[0]["params"].(map[string]any); len(params) != 3 || params["candidate"] == nil || params["cursor"] != nil {
		t.Fatalf("first params = %#v", params)
	} else if candidate := params["candidate"].(map[string]any); len(candidate) != 9 || candidate["item"] != "PISA061" {
		t.Fatalf("first candidate = %#v", candidate)
	}
	if params := requests[1]["params"].(map[string]any); len(params) != 3 || params["cursor"] != cursor || params["candidate"] != nil {
		t.Fatalf("later params = %#v", params)
	}
}

func TestClientSourceCursorStaysWithIssuingCompanion(t *testing.T) {
	candidate, cursor := sourceCandidate(), strings.Repeat("A", 43)
	reads := 0
	client := sourceTestClient(t, func(request *http.Request, body map[string]any) *http.Response {
		if request.URL.Host != "127.0.0.1:41001" || request.Header.Get(companionTokenHeader) != "first" {
			t.Fatalf("source request routed to %s with token %q", request.URL, request.Header.Get(companionTokenHeader))
		}
		params := body["params"].(map[string]any)
		if params["cursor"] == cursor {
			return sourceResponse(body["request_id"].(string), `{"state":"ok","cursor":"`+cursor+`","page":{"content":"second","start_line":2,"line_count":1,"eof":true}}`)
		}
		return sourceResponse(body["request_id"].(string), `{"state":"ok","cursor":"`+cursor+`","page":{"content":"first","start_line":1,"line_count":1,"eof":false}}`)
	})
	client.tokens = targetSourceFunc(func(context.Context) (companionTarget, bool) {
		reads++
		if reads == 1 {
			return companionTarget{endpoint: "http://127.0.0.1:41001", token: "first", instance: "first", generation: 1}, true
		}
		return companionTarget{endpoint: "http://127.0.0.1:41002", token: "second", instance: "second", generation: 1}, true
	})
	if _, err := client.PageSource(context.Background(), SourcePageRequest{Candidate: &candidate, StartLine: 1, MaxLines: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.PageSource(context.Background(), SourcePageRequest{Cursor: cursor, StartLine: 2, MaxLines: 1}); err != nil {
		t.Fatal(err)
	}
	if reads != 1 {
		t.Fatalf("focused Companion discovery calls = %d, want 1", reads)
	}
	if _, ok := client.sourceTarget(context.Background(), cursor); ok {
		t.Fatal("EOF cursor retained its Companion owner")
	}
}

func TestClientSourceCursorDisposalAndUnavailableOwnerDoNotDiscoverFallback(t *testing.T) {
	candidate, cursor := sourceCandidate(), strings.Repeat("A", 43)
	for _, tt := range []struct {
		name      string
		status    int
		want      error
		wantCalls int
	}{
		{"disposal uses issuing Companion", http.StatusOK, nil, 2},
		{"unavailable owner has no fallback", http.StatusServiceUnavailable, ErrSourceUnavailable, 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			reads, calls := 0, 0
			client := sourceTestClient(t, func(request *http.Request, body map[string]any) *http.Response {
				calls++
				if request.URL.Host != "127.0.0.1:41001" || request.Header.Get(companionTokenHeader) != "first" {
					t.Fatalf("source request routed to %s with token %q", request.URL, request.Header.Get(companionTokenHeader))
				}
				if calls == 1 {
					return sourceResponse(body["request_id"].(string), `{"state":"ok","cursor":"`+cursor+`","page":{"content":"first","start_line":1,"line_count":1,"eof":false}}`)
				}
				if tt.status != http.StatusOK {
					return &http.Response{StatusCode: tt.status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(""))}
				}
				return sourceResponse(body["request_id"].(string), `{"state":"disposed"}`)
			})
			client.tokens = targetSourceFunc(func(context.Context) (companionTarget, bool) {
				reads++
				if reads == 1 {
					return companionTarget{endpoint: "http://127.0.0.1:41001", token: "first", instance: "first", generation: 1}, true
				}
				return companionTarget{endpoint: "http://127.0.0.1:41002", token: "second", instance: "second", generation: 2}, true
			})
			if _, err := client.PageSource(context.Background(), SourcePageRequest{Candidate: &candidate, StartLine: 1, MaxLines: 1}); err != nil {
				t.Fatal(err)
			}
			_, err := client.DisposeSource(context.Background(), cursor)
			if !errors.Is(err, tt.want) || calls != tt.wantCalls || reads != 1 {
				t.Fatalf("DisposeSource() error=%v calls=%d discovery=%d", err, calls, reads)
			}
			if _, ok := client.sourceTarget(context.Background(), cursor); ok {
				t.Fatal("terminal cursor retained its Companion owner")
			}
			if _, err := client.PageSource(context.Background(), SourcePageRequest{Cursor: cursor, StartLine: 2, MaxLines: 1}); !errors.Is(err, ErrSourceUnavailable) || calls != tt.wantCalls || reads != 1 {
				t.Fatalf("stale cursor error=%v calls=%d discovery=%d", err, calls, reads)
			}
		})
	}
}

func TestClientSourceCursorCannotResumeAfterClientRestartOrExpiry(t *testing.T) {
	candidate, cursor := sourceCandidate(), strings.Repeat("A", 43)
	client := sourceTestClient(t, func(_ *http.Request, body map[string]any) *http.Response {
		return sourceResponse(body["request_id"].(string), `{"state":"ok","cursor":"`+cursor+`","page":{"content":"first","start_line":1,"line_count":1,"eof":false}}`)
	})
	client.tokens = targetSourceFunc(func(context.Context) (companionTarget, bool) {
		return companionTarget{endpoint: "http://127.0.0.1:41001", token: "first", instance: "first", generation: 1}, true
	})
	if _, err := client.PageSource(context.Background(), SourcePageRequest{Candidate: &candidate, StartLine: 1, MaxLines: 1}); err != nil {
		t.Fatal(err)
	}
	restarted := NewClient()
	restarted.httpClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("restarted client discovered a Companion for an old cursor")
		return nil, nil
	})}
	if _, err := restarted.PageSource(context.Background(), SourcePageRequest{Cursor: cursor, StartLine: 2, MaxLines: 1}); !errors.Is(err, ErrSourceUnavailable) {
		t.Fatalf("restarted cursor error = %v", err)
	}
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var body map[string]any
		if err := json.Unmarshal(mustReadAll(t, request.Body), &body); err != nil {
			t.Fatal(err)
		}
		return sourceResponse(body["request_id"].(string), `{"state":"expired"}`), nil
	})}
	if result, err := client.PageSource(context.Background(), SourcePageRequest{Cursor: cursor, StartLine: 2, MaxLines: 1}); err != nil || result.State != SourceExpired {
		t.Fatalf("expired cursor result = %#v, %v", result, err)
	}
	if _, ok := client.sourceTarget(context.Background(), cursor); ok {
		t.Fatal("expired cursor retained its Companion owner")
	}
}

func TestClientSourceOwnerAffinityExpiresWithoutFallbackAndPrunesOnRegistration(t *testing.T) {
	candidate, first, second := sourceCandidate(), sourceCursor(1), sourceCursor(2)
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	calls, reads := 0, 0
	client := sourceTestClient(t, func(request *http.Request, body map[string]any) *http.Response {
		calls++
		if request.URL.Host != "127.0.0.1:41001" || request.Header.Get(companionTokenHeader) != "first" {
			t.Fatalf("source request routed to %s with token %q", request.URL, request.Header.Get(companionTokenHeader))
		}
		cursor := first
		if calls == 2 {
			cursor = second
		}
		return sourceResponse(body["request_id"].(string), `{"state":"ok","cursor":"`+cursor+`","page":{"content":"first","start_line":1,"line_count":1,"eof":false}}`)
	})
	client.sourceNow = func() time.Time { return now }
	client.tokens = targetSourceFunc(func(context.Context) (companionTarget, bool) {
		reads++
		return companionTarget{endpoint: "http://127.0.0.1:41001", token: "first", instance: "first", generation: 1}, true
	})
	if _, err := client.PageSource(context.Background(), SourcePageRequest{Candidate: &candidate, StartLine: 1, MaxLines: 1}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(sourceOwnerTTL)
	if _, err := client.PageSource(context.Background(), SourcePageRequest{Cursor: first, StartLine: 2, MaxLines: 1}); !errors.Is(err, ErrSourceUnavailable) || calls != 1 || reads != 1 {
		t.Fatalf("expired continuation error=%v calls=%d discovery=%d", err, calls, reads)
	}
	if _, err := client.PageSource(context.Background(), SourcePageRequest{Candidate: &candidate, StartLine: 1, MaxLines: 1}); err != nil {
		t.Fatal(err)
	}
	client.sourceMu.Lock()
	owners := len(client.sourceOwners)
	client.sourceMu.Unlock()
	if owners != 1 {
		t.Fatalf("active owner count = %d, want 1 after expiry pruning", owners)
	}
}

func TestClientSourceOwnerCapacityDisposesRejectedArtifactWithoutFallback(t *testing.T) {
	candidate := sourceCandidate()
	calls, reads := 0, 0
	rejected := sourceCursor(maxSourceOwners + 1)
	disposed := ""
	client := sourceTestClient(t, func(request *http.Request, body map[string]any) *http.Response {
		calls++
		if request.URL.Host != "127.0.0.1:41001" || request.Header.Get(companionTokenHeader) != "first" {
			t.Fatalf("source request routed to %s with token %q", request.URL, request.Header.Get(companionTokenHeader))
		}
		if body["method"] == methodSourceDispose {
			disposed = body["params"].(map[string]any)["cursor"].(string)
			return sourceResponse(body["request_id"].(string), `{"state":"disposed"}`)
		}
		return sourceResponse(body["request_id"].(string), `{"state":"ok","cursor":"`+sourceCursor(calls)+`","page":{"content":"first","start_line":1,"line_count":1,"eof":false}}`)
	})
	client.tokens = targetSourceFunc(func(context.Context) (companionTarget, bool) {
		reads++
		if reads <= maxSourceOwners+1 {
			return companionTarget{endpoint: "http://127.0.0.1:41001", token: "first", instance: "first", generation: 1}, true
		}
		return companionTarget{endpoint: "http://127.0.0.1:41002", token: "second", instance: "second", generation: 2}, true
	})
	for range maxSourceOwners {
		if _, err := client.PageSource(context.Background(), SourcePageRequest{Candidate: &candidate, StartLine: 1, MaxLines: 1}); err != nil {
			t.Fatal(err)
		}
	}
	if result, err := client.PageSource(context.Background(), SourcePageRequest{Candidate: &candidate, StartLine: 1, MaxLines: 1}); !errors.Is(err, ErrSourceFailed) || result != (SourcePageResult{}) {
		t.Fatalf("capacity result=%#v error=%v", result, err)
	}
	if calls != maxSourceOwners+2 || reads != maxSourceOwners+1 || disposed != rejected {
		t.Fatalf("calls=%d discovery=%d disposed=%q", calls, reads, disposed)
	}
	client.sourceMu.Lock()
	owners := len(client.sourceOwners)
	client.sourceMu.Unlock()
	if owners != maxSourceOwners {
		t.Fatalf("active owner count = %d, want %d", owners, maxSourceOwners)
	}
}

func TestClientSourceRejectsInvalidInputBeforeIO(t *testing.T) {
	candidate := sourceCandidate()
	client := sourceTestClient(t, func(*http.Request, map[string]any) *http.Response {
		t.Fatal("invalid request reached HTTP")
		return nil
	})
	for _, request := range []SourcePageRequest{{Candidate: &candidate, StartLine: 0, MaxLines: 1}, {Candidate: &candidate, StartLine: 1, MaxLines: 201}, {Candidate: &catalog.Candidate{Item: "bad"}, StartLine: 1, MaxLines: 1}, {Cursor: "invalid", StartLine: 1, MaxLines: 1}} {
		if _, err := client.PageSource(context.Background(), request); !errors.Is(err, ErrSourceInvalidRequest) {
			t.Fatalf("PageSource(%#v) = %v", request, err)
		}
	}
	if _, err := client.DisposeSource(context.Background(), "invalid"); !errors.Is(err, ErrSourceInvalidRequest) {
		t.Fatalf("DisposeSource() = %v", err)
	}
}

func TestClientSourceStrictFailuresAndStates(t *testing.T) {
	candidate, cursor := sourceCandidate(), strings.Repeat("A", 43)
	for _, tt := range []struct {
		name, result string
		want         SourceState
		fail         bool
	}{
		{"expired cursor", `{"state":"expired"}`, SourceExpired, false}, {"cleanup failed", `{"state":"cleanup_failed"}`, SourceCleanupFailed, false}, {"unexpected field", `{"state":"expired","detail":"secret"}`, "", true}, {"mismatched page start", `{"state":"ok","cursor":"` + cursor + `","page":{"content":"x","start_line":2,"line_count":1,"eof":false}}`, "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := sourceTestClient(t, func(_ *http.Request, body map[string]any) *http.Response {
				return sourceResponse(body["request_id"].(string), tt.result)
			})
			result, err := client.PageSource(context.Background(), SourcePageRequest{Candidate: &candidate, StartLine: 1, MaxLines: 1})
			if tt.fail && !errors.Is(err, ErrSourceFailed) {
				t.Fatalf("error = %v", err)
			}
			if !tt.fail && (err != nil || result.State != tt.want) {
				t.Fatalf("result = %#v, %v", result, err)
			}
		})
	}
	client := sourceTestClient(t, func(_ *http.Request, body map[string]any) *http.Response {
		if body["method"] == methodSourcePage {
			return sourceResponse(body["request_id"].(string), `{"state":"ok","cursor":"`+cursor+`","page":{"content":"x","start_line":1,"line_count":1,"eof":false}}`)
		}
		return sourceResponse(body["request_id"].(string), `{"state":"disposed"}`)
	})
	if _, err := client.PageSource(context.Background(), SourcePageRequest{Candidate: &candidate, StartLine: 1, MaxLines: 1}); err != nil {
		t.Fatalf("first page = %v", err)
	}
	if result, err := client.DisposeSource(context.Background(), cursor); err != nil || result.State != SourceDisposed {
		t.Fatalf("dispose = %#v, %v", result, err)
	}
}

func TestClientSourceTransportCorrelationAndSecrecy(t *testing.T) {
	candidate, first, second := sourceCandidate(), testToken(t), testToken(t)
	calls, reads := 0, 0
	client := sourceTestClient(t, func(request *http.Request, body map[string]any) *http.Response {
		calls++
		if strings.Contains(request.URL.String(), first) || strings.Contains(body["params"].(map[string]any)["candidate"].(map[string]any)["description"].(string), "secret") {
			t.Fatal("sensitive value escaped")
		}
		if calls == 1 {
			return &http.Response{StatusCode: http.StatusUnauthorized, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(""))}
		}
		if request.Header.Get(companionTokenHeader) != second {
			t.Fatal("token was not rotated")
		}
		return sourceResponse(body["request_id"].(string), `{"state":"ok","cursor":"`+strings.Repeat("A", 43)+`","page":{"content":"x","start_line":1,"line_count":1,"eof":true}}`)
	})
	client.tokens = tokenSourceFunc(func(context.Context) (string, bool) {
		values := []string{first, second}
		value := values[reads]
		reads++
		return value, true
	})
	if _, err := client.PageSource(context.Background(), SourcePageRequest{Candidate: &candidate, StartLine: 1, MaxLines: 1}); err != nil || calls != 2 {
		t.Fatalf("rotation error = %v, calls=%d", err, calls)
	}
	for _, tt := range []struct {
		name string
		body []byte
		want error
	}{
		{"oversized", []byte(strings.Repeat("x", maxSourceResponseBytes+1)), ErrSourceFailed},
		{"mismatched correlation", []byte(`{"version":1,"request_id":"other","result":{"state":"expired"}}`), ErrSourceUnavailable},
		{"invalid UTF-8", []byte{0xff}, ErrSourceFailed},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client = sourceTestClient(t, func(*http.Request, map[string]any) *http.Response { return sourceRawResponse(tt.body) })
			_, err := client.PageSource(context.Background(), SourcePageRequest{Candidate: &candidate, StartLine: 1, MaxLines: 1})
			if !errors.Is(err, tt.want) || strings.Contains(err.Error(), first) || strings.Contains(err.Error(), "PISA061") {
				t.Fatalf("body error = %v", err)
			}
		})
	}
}

func TestClientSourceCancellationTimeoutAndUnavailable(t *testing.T) {
	candidate := sourceCandidate()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := sourceTestClient(t, func(*http.Request, map[string]any) *http.Response {
		t.Fatal("cancelled request reached HTTP")
		return nil
	})
	if _, err := client.PageSource(ctx, SourcePageRequest{Candidate: &candidate, StartLine: 1, MaxLines: 1}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel = %v", err)
	}
	client = sourceTestClient(t, func(request *http.Request, _ map[string]any) *http.Response {
		if _, ok := request.Context().Deadline(); !ok {
			t.Fatal("source request lacks a timeout")
		}
		<-request.Context().Done()
		return nil
	})
	deadline, stop := context.WithTimeout(context.Background(), time.Millisecond)
	defer stop()
	if _, err := client.PageSource(deadline, SourcePageRequest{Candidate: &candidate, StartLine: 1, MaxLines: 1}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout = %v", err)
	}
	client.httpClient = nil
	if _, err := client.PageSource(context.Background(), SourcePageRequest{Candidate: &candidate, StartLine: 1, MaxLines: 1}); !errors.Is(err, ErrSourceUnavailable) {
		t.Fatalf("unavailable = %v", err)
	}
}

func sourceCandidate() catalog.Candidate {
	return catalog.Candidate{Item: "PISA061", SourceLibrary: "SRCLIB", SourceFileBase: "QRPG", ObjectType: "M", SourceType: "RPGLE", Application: "APP", Version: "V1", ProductionLibrary: "PRODLIB", Description: "description"}
}

func sourceCursor(value int) string {
	var raw [32]byte
	binary.BigEndian.PutUint64(raw[24:], uint64(value))
	return base64.RawURLEncoding.EncodeToString(raw[:])
}

func sourceTestClient(t *testing.T, handler func(*http.Request, map[string]any) *http.Response) *Client {
	t.Helper()
	client := NewClient()
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var body map[string]any
		if err := json.Unmarshal(mustReadAll(t, request.Body), &body); err != nil {
			t.Fatal(err)
		}
		return handler(request, body), nil
	})}
	return client
}

func sourceResponse(id, result string) *http.Response {
	return sourceRawResponse([]byte(`{"version":1,"request_id":"` + id + `","result":` + result + `}`))
}

func sourceRawResponse(body []byte) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body)))}
}
