package codefori

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
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
	target, present := client.sourceTarget(ctx, request.Cursor)
	if request.Cursor != "" && !present {
		return SourcePageResult{}, ErrSourceUnavailable
	}
	envelope, target, err := client.postSource(ctx, methodSourcePage, params, target, present, request.Cursor == "")
	if err != nil {
		if request.Cursor != "" && errors.Is(err, ErrSourceUnavailable) {
			client.forgetSourceOwner(request.Cursor)
		}
		return SourcePageResult{}, err
	}
	result, err := decodeSourcePageResult(envelope.Result)
	if err != nil {
		if request.Cursor != "" {
			client.forgetSourceOwner(request.Cursor)
		}
		return SourcePageResult{}, ErrSourceFailed
	}
	if result.State == SourceOK && (result.Page.StartLine != request.StartLine || result.Page.LineCount > request.MaxLines || result.Page.LineCount == 0 && !result.Page.EOF || request.Cursor != "" && result.Cursor != request.Cursor) {
		if request.Cursor != "" {
			client.forgetSourceOwner(request.Cursor)
		}
		return SourcePageResult{}, ErrSourceFailed
	}
	if request.Cursor != "" && result.State != SourceOK {
		client.forgetSourceOwner(request.Cursor)
	}
	if result.State == SourceOK && result.Page.EOF {
		client.forgetSourceOwner(result.Cursor)
	} else if request.Cursor == "" && result.State == SourceOK {
		if !client.rememberSourceOwner(result.Cursor, target) {
			client.discardSource(result.Cursor, target)
			return SourcePageResult{}, ErrSourceFailed
		}
	}
	return result, nil
}

// DisposeSource invokes the fixed source disposal RPC for an opaque cursor.
func (client *Client) DisposeSource(ctx context.Context, cursor string) (SourceDisposeResult, error) {
	if !validSourceCursor(cursor) {
		return SourceDisposeResult{}, ErrSourceInvalidRequest
	}
	defer client.forgetSourceOwner(cursor)
	if err := ctx.Err(); err != nil {
		return SourceDisposeResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	target, present := client.sourceTarget(ctx, cursor)
	if !present {
		return SourceDisposeResult{}, ErrSourceUnavailable
	}
	envelope, _, err := client.postSource(ctx, methodSourceDispose, map[string]any{"cursor": cursor}, target, true, false)
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

func (client *Client) postSource(ctx context.Context, method string, params map[string]any, target companionTarget, present, refreshOnAuthFailure bool) (rpcEnvelope, companionTarget, error) {
	requestID, err := newRequestID()
	if err != nil {
		return rpcEnvelope{}, target, ErrSourceFailed
	}
	body, err := json.Marshal(struct {
		Version   int            `json:"version"`
		RequestID string         `json:"request_id"`
		Method    string         `json:"method"`
		Params    map[string]any `json:"params"`
	}{protocolVersion, requestID, method, params})
	if err != nil || len(body) > maxSourceRequestBytes {
		return rpcEnvelope{}, target, ErrSourceInvalidRequest
	}
	if client.httpClient == nil {
		return rpcEnvelope{}, target, ErrSourceUnavailable
	}
	envelope, rejected, state := client.send(ctx, body, requestID, target.endpoint, target.token, maxSourceResponseBytes)
	if refreshOnAuthFailure && rejected && present && ctx.Err() == nil {
		if rotated, valid := client.target(ctx); valid && rotated.instance == target.instance && rotated.generation == target.generation && rotated.endpoint == target.endpoint && rotated.token != target.token {
			target = rotated
			envelope, _, state = client.send(ctx, body, requestID, rotated.endpoint, rotated.token, maxSourceResponseBytes)
		}
	}
	if err := ctx.Err(); err != nil {
		return rpcEnvelope{}, target, err
	}
	if state == "ok" {
		return envelope, target, nil
	}
	if state == "failed" {
		return rpcEnvelope{}, target, ErrSourceFailed
	}
	return rpcEnvelope{}, target, ErrSourceUnavailable
}

func (client *Client) sourceTarget(ctx context.Context, cursor string) (companionTarget, bool) {
	if cursor == "" {
		return client.target(ctx)
	}
	client.sourceMu.Lock()
	defer client.sourceMu.Unlock()
	now := client.sourceOwnerNow()
	for key, owner := range client.sourceOwners {
		if !now.Before(owner.expiresAt) {
			delete(client.sourceOwners, key)
		}
	}
	target, ok := client.sourceOwners[cursor]
	if !ok {
		return companionTarget{}, false
	}
	return target.target, true
}

func (client *Client) rememberSourceOwner(cursor string, target companionTarget) bool {
	client.sourceMu.Lock()
	defer client.sourceMu.Unlock()
	if client.sourceOwners == nil {
		client.sourceOwners = make(map[string]sourceOwner)
	}
	now := client.sourceOwnerNow()
	for key, owner := range client.sourceOwners {
		if !now.Before(owner.expiresAt) {
			delete(client.sourceOwners, key)
		}
	}
	if _, exists := client.sourceOwners[cursor]; !exists && len(client.sourceOwners) >= maxSourceOwners {
		return false
	}
	client.sourceOwners[cursor] = sourceOwner{target: target, expiresAt: now.Add(sourceOwnerTTL)}
	return true
}

func (client *Client) forgetSourceOwner(cursor string) {
	client.sourceMu.Lock()
	defer client.sourceMu.Unlock()
	delete(client.sourceOwners, cursor)
}

func (client *Client) sourceOwnerNow() time.Time {
	if client.sourceNow != nil {
		return client.sourceNow()
	}
	return time.Now()
}

func (client *Client) discardSource(cursor string, target companionTarget) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	_, _, _ = client.postSource(ctx, methodSourceDispose, map[string]any{"cursor": cursor}, target, true, false)
}
