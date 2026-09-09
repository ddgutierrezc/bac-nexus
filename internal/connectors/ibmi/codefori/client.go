package codefori

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"bac-nexus/internal/inspection"
	"bac-nexus/internal/provider"
)

const (
	fixedRPCURL           = "http://127.0.0.1:64139/v1/rpc"
	statusTimeout         = time.Second
	responseHeaderTimeout = 6 * time.Second
	queryTimeout          = 7 * time.Second
)

type Client struct {
	httpClient *http.Client
	tokens     tokenSource
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
		tokens: newFileTokenSource(defaultTokenStatePath),
	}
}

func (client *Client) SessionStatus(ctx context.Context) provider.SessionStatusResult {
	ctx, cancel := context.WithTimeout(ctx, statusTimeout)
	defer cancel()
	envelope, state := client.post(ctx, methodStatus, nil)
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
	envelope, state := client.post(ctx, methodSQLQuery, map[string]string{"sql": canonical})
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
	envelope, state := client.post(ctx, methodResolveProgram, params)
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
	envelope, state := client.post(ctx, methodFindProgramSource, map[string]string{"library": program.Library, "name": program.Name, "objectType": program.ObjectType})
	if state != provider.QueryOK {
		return inspection.SourceResult{State: inspection.StateUnavailable, Reason: "companion_unavailable", Certainty: "unavailable", Completeness: "complete"}
	}
	result, err := decodeSourceResult(envelope.Result)
	if err != nil {
		return inspection.SourceResult{State: inspection.StateUnavailable, Reason: "invalid_companion_response", Certainty: "unavailable", Completeness: "complete"}
	}
	return result
}

func (client *Client) post(ctx context.Context, method string, params map[string]string) (rpcEnvelope, provider.QueryState) {
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
	token, present := client.token(ctx)
	envelope, authRejected, state := client.send(ctx, body, requestID, token)
	if !authRejected || !present || ctx.Err() != nil {
		return envelope, state
	}
	rotated, valid := client.token(ctx)
	if !valid || rotated == token {
		return envelope, state
	}
	envelope, _, state = client.send(ctx, body, requestID, rotated)
	return envelope, state
}

func (client *Client) token(ctx context.Context) (string, bool) {
	if client.tokens == nil {
		return "", false
	}
	return client.tokens.Token(ctx)
}

func (client *Client) send(ctx context.Context, body []byte, requestID, token string) (rpcEnvelope, bool, provider.QueryState) {
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, fixedRPCURL, bytes.NewReader(body))
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
	responseBody, err := readBoundedBody(response.Body, maxResponseBytes)
	if err != nil {
		return rpcEnvelope{}, false, provider.QueryFailed
	}
	envelope, err := decodeEnvelope(responseBody)
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
