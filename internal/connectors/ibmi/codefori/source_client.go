package codefori

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"unicode/utf8"

	"bac-nexus/internal/catalog"
)

const (
	methodSourcePage       = "source_artifact.page.v1"
	methodSourceDispose    = "source_artifact.dispose.v1"
	maxSourceRequestBytes  = 4096
	maxSourceResponseBytes = 128 * 1024
	maxSourcePageLines     = 200
)

var (
	ErrSourceInvalidRequest = errors.New("invalid source request")
	ErrSourceUnavailable    = errors.New("source operation unavailable")
	ErrSourceFailed         = errors.New("source operation failed")
)

type SourceState string

const (
	SourceOK               SourceState = "ok"
	SourceDisposed         SourceState = "disposed"
	SourceInvalidRequest   SourceState = "invalid_request"
	SourceNotFound         SourceState = "not_found"
	SourceAmbiguous        SourceState = "ambiguous"
	SourceUnavailable      SourceState = "unavailable"
	SourceExpired          SourceState = "expired"
	SourceInvalidEncoding  SourceState = "invalid_source_encoding"
	SourceResponseTooLarge SourceState = "response_too_large"
	SourceCleanupFailed    SourceState = "cleanup_failed"
)

type SourcePageRequest struct {
	Candidate *catalog.Candidate
	Cursor    string
	StartLine int
	MaxLines  int
}

type SourcePage struct {
	Content   string
	StartLine int
	LineCount int
	EOF       bool
}

type SourcePageResult struct {
	State  SourceState
	Cursor string
	Page   SourcePage
}

type SourceDisposeResult struct{ State SourceState }

// PageSource invokes the fixed source paging RPC. A first page needs a complete
// Catalogados candidate; a later page needs only its opaque cursor and range.
func (client *Client) PageSource(ctx context.Context, request SourcePageRequest) (SourcePageResult, error) {
	params, err := sourcePageParams(request)
	if err != nil {
		return SourcePageResult{}, ErrSourceInvalidRequest
	}
	if err := ctx.Err(); err != nil {
		return SourcePageResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	envelope, err := client.postSource(ctx, methodSourcePage, params)
	if err != nil {
		return SourcePageResult{}, err
	}
	result, err := decodeSourcePageResult(envelope.Result)
	if err != nil {
		return SourcePageResult{}, ErrSourceFailed
	}
	if result.State == SourceOK && (result.Page.StartLine != request.StartLine || result.Page.LineCount > request.MaxLines || result.Page.LineCount == 0 && !result.Page.EOF || request.Cursor != "" && result.Cursor != request.Cursor) {
		return SourcePageResult{}, ErrSourceFailed
	}
	return result, nil
}

// DisposeSource invokes the fixed source disposal RPC for an opaque cursor.
func (client *Client) DisposeSource(ctx context.Context, cursor string) (SourceDisposeResult, error) {
	if !validSourceCursor(cursor) {
		return SourceDisposeResult{}, ErrSourceInvalidRequest
	}
	if err := ctx.Err(); err != nil {
		return SourceDisposeResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	envelope, err := client.postSource(ctx, methodSourceDispose, map[string]any{"cursor": cursor})
	if err != nil {
		return SourceDisposeResult{}, err
	}
	result, err := decodeSourceDisposeResult(envelope.Result)
	if err != nil {
		return SourceDisposeResult{}, ErrSourceFailed
	}
	return result, nil
}

func sourcePageParams(request SourcePageRequest) (map[string]any, error) {
	if request.StartLine < 1 || request.MaxLines < 1 || request.MaxLines > maxSourcePageLines {
		return nil, ErrSourceInvalidRequest
	}
	if request.Candidate != nil {
		if request.Cursor != "" || !validSourceCandidate(*request.Candidate) {
			return nil, ErrSourceInvalidRequest
		}
		return map[string]any{"candidate": *request.Candidate, "start_line": request.StartLine, "max_lines": request.MaxLines}, nil
	}
	if !validSourceCursor(request.Cursor) {
		return nil, ErrSourceInvalidRequest
	}
	return map[string]any{"cursor": request.Cursor, "start_line": request.StartLine, "max_lines": request.MaxLines}, nil
}

func validSourceCandidate(candidate catalog.Candidate) bool {
	if _, err := candidate.MemberPath(); err != nil {
		return false
	}
	for _, value := range []string{candidate.Item, candidate.SourceLibrary, candidate.SourceFileBase, candidate.ObjectType, candidate.SourceType, candidate.Application, candidate.Version, candidate.ProductionLibrary, candidate.Description} {
		if len(value) > 256 || !utf8.ValidString(value) {
			return false
		}
	}
	return true
}

func validSourceCursor(cursor string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	return err == nil && len(decoded) == 32 && len(cursor) == 43
}

func (client *Client) postSource(ctx context.Context, method string, params map[string]any) (rpcEnvelope, error) {
	requestID, err := newRequestID()
	if err != nil {
		return rpcEnvelope{}, ErrSourceFailed
	}
	body, err := json.Marshal(struct {
		Version   int            `json:"version"`
		RequestID string         `json:"request_id"`
		Method    string         `json:"method"`
		Params    map[string]any `json:"params"`
	}{protocolVersion, requestID, method, params})
	if err != nil || len(body) > maxSourceRequestBytes {
		return rpcEnvelope{}, ErrSourceInvalidRequest
	}
	if client.httpClient == nil {
		return rpcEnvelope{}, ErrSourceUnavailable
	}
	target, present := client.target(ctx)
	envelope, rejected, state := client.send(ctx, body, requestID, target.endpoint, target.token, maxSourceResponseBytes)
	if rejected && present && ctx.Err() == nil {
		if rotated, valid := client.target(ctx); valid && rotated.instance == target.instance && rotated.generation == target.generation && rotated.endpoint == target.endpoint && rotated.token != target.token {
			envelope, _, state = client.send(ctx, body, requestID, rotated.endpoint, rotated.token, maxSourceResponseBytes)
		}
	}
	if err := ctx.Err(); err != nil {
		return rpcEnvelope{}, err
	}
	if state == "ok" {
		return envelope, nil
	}
	if state == "failed" {
		return rpcEnvelope{}, ErrSourceFailed
	}
	return rpcEnvelope{}, ErrSourceUnavailable
}
