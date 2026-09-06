package codefori

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

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
}

var _ provider.Provider = (*Client)(nil)

// NewClient constructs the fixed unauthenticated loopback provider.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Transport: &http.Transport{
			ResponseHeaderTimeout: responseHeaderTimeout,
		}},
	}
}

func (client *Client) SessionStatus(ctx context.Context) provider.SessionStatusResult {
	ctx, cancel := context.WithTimeout(ctx, statusTimeout)
	defer cancel()
	envelope, state := client.post(ctx, methodStatus, "")
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
	envelope, state := client.post(ctx, methodSQLQuery, canonical)
	if state != provider.QueryOK {
		return provider.QueryResult{State: state}
	}
	result, err := decodeQueryResult(envelope.Result)
	if err != nil || !provider.ValidateQueryResult(result) {
		return provider.QueryResult{State: provider.QueryFailed}
	}
	return result
}

func (client *Client) post(ctx context.Context, method, sql string) (rpcEnvelope, provider.QueryState) {
	requestID, err := newRequestID()
	if err != nil {
		return rpcEnvelope{}, provider.QueryFailed
	}
	request := rpcRequest{Version: protocolVersion, RequestID: requestID, Method: method}
	request.Params.SQL = sql
	body, err := encodeRequest(request)
	if err != nil {
		return rpcEnvelope{}, provider.QueryFailed
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, fixedRPCURL, bytes.NewReader(body))
	if err != nil {
		return rpcEnvelope{}, provider.QueryFailed
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	if client.httpClient == nil {
		return rpcEnvelope{}, provider.QueryUnavailable
	}
	response, err := client.httpClient.Do(httpRequest)
	if err != nil {
		return rpcEnvelope{}, contextOrUnavailable(ctx)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return rpcEnvelope{}, provider.QueryUnavailable
	}
	responseBody, err := readBoundedBody(response.Body, maxResponseBytes)
	if err != nil {
		return rpcEnvelope{}, provider.QueryFailed
	}
	envelope, err := decodeEnvelope(responseBody)
	if err != nil || envelope.Version != protocolVersion || envelope.RequestID != requestID {
		return rpcEnvelope{}, provider.QueryUnavailable
	}
	return envelope, provider.QueryOK
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
