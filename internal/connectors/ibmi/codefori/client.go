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
	readDescriptor DescriptorReader
	httpClient     *http.Client
}

var _ provider.Provider = (*Client)(nil)

// NewClient constructs the fixed-loopback provider with a descriptor reader.
// A nil reader fails closed, which is the deterministic unsupported-platform path.
func NewClient(reader DescriptorReader) *Client {
	return &Client{
		readDescriptor: reader,
		httpClient: &http.Client{Transport: &http.Transport{
			ResponseHeaderTimeout: responseHeaderTimeout,
		}},
	}
}

func (client *Client) SessionStatus(ctx context.Context) provider.SessionStatusResult {
	ctx, cancel := context.WithTimeout(ctx, statusTimeout)
	defer cancel()
	descriptor, ok := client.descriptor()
	if !ok {
		return provider.SessionStatusResult{State: provider.SessionCompanionUnavailable}
	}
	envelope, state := client.post(ctx, descriptor, methodStatus, "")
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
	descriptor, ok := client.descriptor()
	if !ok {
		return provider.QueryResult{State: provider.QueryUnavailable}
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	envelope, state := client.post(ctx, descriptor, methodSQLQuery, canonical)
	if state != provider.QueryOK {
		return provider.QueryResult{State: state}
	}
	result, err := decodeQueryResult(envelope.Result)
	if err != nil || !provider.ValidateQueryResult(result) {
		return provider.QueryResult{State: provider.QueryFailed}
	}
	return result
}

func (client *Client) descriptor() (Descriptor, bool) {
	if client == nil || client.readDescriptor == nil {
		return Descriptor{}, false
	}
	descriptor, err := client.readDescriptor()
	if err != nil || !descriptor.valid() {
		return Descriptor{}, false
	}
	return descriptor, true
}

func (client *Client) post(ctx context.Context, descriptor Descriptor, method, sql string) (rpcEnvelope, provider.QueryState) {
	requestID, err := newRequestID()
	if err != nil {
		return rpcEnvelope{}, provider.QueryFailed
	}
	request := rpcRequest{Version: protocolVersion, Generation: descriptor.Generation, RequestID: requestID, Method: method}
	request.Params.SQL = sql
	body, err := encodeRequest(request)
	if err != nil {
		return rpcEnvelope{}, provider.QueryFailed
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, fixedRPCURL, bytes.NewReader(body))
	if err != nil {
		return rpcEnvelope{}, provider.QueryFailed
	}
	httpRequest.Header.Set("Authorization", "Bearer "+descriptor.Token)
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
	if err != nil || envelope.Version != protocolVersion || envelope.Generation != descriptor.Generation || envelope.RequestID != requestID {
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
