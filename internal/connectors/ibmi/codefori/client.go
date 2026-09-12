package codefori

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"sync"
	"time"
	"unicode/utf8"

	"bac-nexus/internal/catalog"
	"bac-nexus/internal/inspection"
	"bac-nexus/internal/provider"
)

const (
	sourceOwnerTTL = 10 * time.Minute
	// maxSourceOwners bounds volatile cursor affinity during bursts of new pages.
	maxSourceOwners = 256
)

var (
	ErrCatalogUnavailable = errors.New("catalog operation unavailable")
	ErrCatalogFailed      = errors.New("catalog operation failed")
)

const (
	fixedRPCURL           = "http://127.0.0.1:64139/v1/rpc"
	statusTimeout         = time.Second
	responseHeaderTimeout = 6 * time.Second
	queryTimeout          = 7 * time.Second
)

type Client struct {
	httpClient   *http.Client
	tokens       tokenSource
	sourceMu     sync.Mutex
	sourceOwners map[string]sourceOwner
	sourceNow    func() time.Time
}

type sourceOwner struct {
	target    companionTarget
	expiresAt time.Time
}

var _ provider.Provider = (*Client)(nil)
var _ inspection.Provider = (*Client)(nil)

// NewClient constructs the fixed loopback provider. It remains compatible with
// Companion versions that have not yet published a private token state.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Transport: &http.Transport{
			ResponseHeaderTimeout: responseHeaderTimeout,
		}},
		tokens:       newFileTokenSource(defaultTokenStatePath),
		sourceOwners: make(map[string]sourceOwner),
		sourceNow:    time.Now,
	}
}

func (client *Client) SessionStatus(ctx context.Context) provider.SessionStatusResult {
	ctx, cancel := context.WithTimeout(ctx, statusTimeout)
	defer cancel()
	envelope, state := client.post(ctx, methodStatus, nil, maxResponseBytes)
	if state != provider.QueryOK {
		return provider.SessionStatusResult{State: provider.SessionCompanionUnavailable}
	}
	status, err := decodeSessionState(envelope.Result)
	if err != nil {
		return provider.SessionStatusResult{State: provider.SessionCompanionUnavailable}
	}
	return provider.SessionStatusResult{State: status}
}

func (client *Client) Query(ctx context.Context, request provider.QueryRequest) provider.QueryResult {
	if ctx.Err() != nil {
		return contextResult(ctx)
	}
	canonical, ok := provider.CanonicalizeQuery(request.SQL)
	if !ok {
		return provider.QueryResult{State: provider.QueryInvalidQuery}
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	envelope, state := client.post(ctx, methodSQLQuery, map[string]string{"sql": canonical}, maxResponseBytes)
	if state != provider.QueryOK {
		return provider.QueryResult{State: state}
	}
	result, err := decodeQueryResult(envelope.Result)
	if err != nil || !provider.ValidateQueryResult(result) {
		return provider.QueryResult{State: provider.QueryFailed}
	}
	return result
}

func (client *Client) ResolveProgram(ctx context.Context, request inspection.ResolveRequest) inspection.ResolveResult {
	if ctx.Err() != nil {
		return inspection.ResolveResult{State: inspection.StateUnavailable, Completeness: "complete"}
	}
	params := map[string]string{"name": request.Name}
	if request.Library != "" {
		params["library"] = request.Library
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	envelope, state := client.post(ctx, methodResolveProgram, params, maxResponseBytes)
	if state != provider.QueryOK {
		return inspection.ResolveResult{State: inspection.StateUnavailable, Completeness: "complete", Reason: "companion_unavailable"}
	}
	result, err := decodeResolveProgramResult(envelope.Result)
	if err != nil {
		return inspection.ResolveResult{State: inspection.StateUnavailable, Completeness: "complete", Reason: "invalid_companion_response"}
	}
	return result
}

func (client *Client) FindProgramSource(ctx context.Context, program inspection.ResolvedProgram) inspection.SourceResult {
	if ctx.Err() != nil {
		return inspection.SourceResult{State: inspection.StateUnavailable, Certainty: "unavailable", Completeness: "complete"}
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	envelope, state := client.post(ctx, methodFindProgramSource, map[string]string{"library": program.Library, "name": program.Name, "objectType": program.ObjectType}, maxResponseBytes)
	if state != provider.QueryOK {
		return inspection.SourceResult{State: inspection.StateUnavailable, Reason: "companion_unavailable", Certainty: "unavailable", Completeness: "complete"}
	}
	result, err := decodeSourceResult(envelope.Result)
	if err != nil {
		return inspection.SourceResult{State: inspection.StateUnavailable, Reason: "invalid_companion_response", Certainty: "unavailable", Completeness: "complete"}
	}
	return result
}

// ResolveCatalog invokes only the fixed metadata RPC and preserves response order.
func (client *Client) ResolveCatalog(ctx context.Context, search catalog.Search) ([]catalog.Candidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	canonical, err := catalog.NewSearch(search.Item, search.ProductionLibrary)
	if err != nil {
		return nil, ErrCatalogFailed
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	envelope, state := client.post(ctx, methodResolveCatalog, map[string]string{"item": canonical.Item, "productionLibrary": canonical.ProductionLibrary}, maxCatalogResponseBytes)
	if state != provider.QueryOK {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if state == provider.QueryFailed {
			return nil, ErrCatalogFailed
		}
		return nil, ErrCatalogUnavailable
	}
	result, err := decodeCatalogResult(envelope.Result)
	if err != nil {
		return nil, ErrCatalogFailed
	}
	switch result.State {
	case "ok":
		if len(result.Candidates) == 0 {
			return nil, catalog.ErrCandidateNotFound
		}
		return result.Candidates, nil
	case "candidate_limit_exceeded":
		return nil, catalog.ErrCandidateLimit
	case "unavailable":
		return nil, ErrCatalogUnavailable
	default:
		return nil, ErrCatalogFailed
	}
}

func (client *Client) post(ctx context.Context, method string, params map[string]string, maximum int) (rpcEnvelope, provider.QueryState) {
	if ctx.Err() != nil {
		return rpcEnvelope{}, contextOrUnavailable(ctx)
	}
	requestID, err := newRequestID()
	if err != nil {
		return rpcEnvelope{}, provider.QueryFailed
	}
	request := rpcRequest{Version: protocolVersion, RequestID: requestID, Method: method, Params: params}
	body, err := encodeRequest(request)
	if err != nil {
		return rpcEnvelope{}, provider.QueryFailed
	}
	if client.httpClient == nil {
		return rpcEnvelope{}, provider.QueryUnavailable
	}
	target, present := client.target(ctx)
	envelope, authRejected, state := client.send(ctx, body, requestID, target.endpoint, target.token, maximum)
	if !authRejected || !present || ctx.Err() != nil {
		return envelope, state
	}
	rotated, valid := client.target(ctx)
	if !valid || rotated.instance != target.instance || rotated.generation != target.generation || rotated.endpoint != target.endpoint || rotated.token == target.token {
		return envelope, state
	}
	envelope, _, state = client.send(ctx, body, requestID, rotated.endpoint, rotated.token, maximum)
	return envelope, state
}

func (client *Client) token(ctx context.Context) (string, bool) {
	if client.tokens == nil {
		return "", false
	}
	return client.tokens.Token(ctx)
}

func (client *Client) target(ctx context.Context) (companionTarget, bool) {
	if client.tokens == nil {
		return companionTarget{}, false
	}
	if source, ok := client.tokens.(targetSource); ok {
		target, found := source.Target(ctx)
		if found {
			return target, true
		}
		if target.instance == "blocked" {
			return companionTarget{}, false
		}
		return companionTarget{endpoint: fixedRPCURL[:len(fixedRPCURL)-len("/v1/rpc")], instance: "v1"}, false
	}
	token, ok := client.tokens.Token(ctx)
	return companionTarget{endpoint: fixedRPCURL[:len(fixedRPCURL)-len("/v1/rpc")], token: token, instance: "test"}, ok
}

func (client *Client) send(ctx context.Context, body []byte, requestID, endpoint, token string, maximum int) (rpcEnvelope, bool, provider.QueryState) {
	if endpoint == "" {
		return rpcEnvelope{}, false, provider.QueryUnavailable
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/v1/rpc", bytes.NewReader(body))
	if err != nil {
		return rpcEnvelope{}, false, provider.QueryFailed
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	if token != "" {
		httpRequest.Header.Set(companionTokenHeader, token)
	}
	response, err := client.httpClient.Do(httpRequest)
	if err != nil {
		return rpcEnvelope{}, false, contextOrUnavailable(ctx)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return rpcEnvelope{}, true, provider.QueryUnavailable
	}
	if response.StatusCode != http.StatusOK {
		return rpcEnvelope{}, false, provider.QueryUnavailable
	}
	responseBody, err := readBoundedBody(response.Body, maximum)
	if err != nil || !utf8.Valid(responseBody) {
		return rpcEnvelope{}, false, provider.QueryFailed
	}
	envelope, err := decodeEnvelopeWithLimit(responseBody, maximum)
	if err != nil || envelope.Version != protocolVersion || envelope.RequestID != requestID {
		return rpcEnvelope{}, false, provider.QueryUnavailable
	}
	return envelope, false, provider.QueryOK
}

func newRequestID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func contextOrUnavailable(ctx context.Context) provider.QueryState {
	if ctx.Err() != nil {
		return contextResult(ctx).State
	}
	return provider.QueryUnavailable
}

func contextResult(ctx context.Context) provider.QueryResult {
	if ctx.Err() == context.DeadlineExceeded {
		return provider.QueryResult{State: provider.QueryTimeout}
	}
	return provider.QueryResult{State: provider.QueryCancelled}
}
