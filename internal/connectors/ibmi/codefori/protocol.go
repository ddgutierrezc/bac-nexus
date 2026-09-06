package codefori

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"bac-nexus/internal/provider"
)

const (
	protocolVersion  = 1
	methodStatus     = "session.status"
	methodSQLQuery   = "sql.query"
	maxRequestBytes  = 512
	maxResponseBytes = 1024
)

var (
	errInvalidProtocol = errors.New("invalid companion protocol message")
	errBodyLimit       = errors.New("companion protocol message exceeds limit")
)

type rpcRequest struct {
	Version   int
	RequestID string
	Method    string
	Params    struct {
		SQL string
	}
}

type rpcResponse struct {
	Version   int
	RequestID string
	Result    provider.QueryResult
}

type rpcEnvelope struct {
	Version   int
	RequestID string
	Result    json.RawMessage
}

func encodeRequest(request rpcRequest) ([]byte, error) {
	if request.Version != protocolVersion || request.RequestID == "" {
		return nil, errInvalidProtocol
	}
	params := map[string]string{}
	switch request.Method {
	case methodStatus:
		if request.Params.SQL != "" {
			return nil, errInvalidProtocol
		}
	case methodSQLQuery:
		if request.Params.SQL != provider.CanonicalProofQuery {
			return nil, errInvalidProtocol
		}
		params["sql"] = request.Params.SQL
	default:
		return nil, errInvalidProtocol
	}
	body, err := json.Marshal(struct {
		Version   int               `json:"version"`
		RequestID string            `json:"request_id"`
		Method    string            `json:"method"`
		Params    map[string]string `json:"params"`
	}{request.Version, request.RequestID, request.Method, params})
	if err != nil {
		return nil, errInvalidProtocol
	}
	if len(body) > maxRequestBytes {
		return nil, errBodyLimit
	}
	return body, nil
}

func decodeRequest(body []byte) (rpcRequest, error) {
	if len(body) > maxRequestBytes {
		return rpcRequest{}, errBodyLimit
	}
	fields, err := decodeExactObject(body, "version", "request_id", "method", "params")
	if err != nil {
		return rpcRequest{}, err
	}
	request := rpcRequest{}
	if err := json.Unmarshal(fields["version"], &request.Version); err != nil {
		return rpcRequest{}, errInvalidProtocol
	}
	if request.RequestID, err = decodeString(fields["request_id"]); err != nil {
		return rpcRequest{}, err
	}
	if request.Method, err = decodeString(fields["method"]); err != nil {
		return rpcRequest{}, err
	}
	params, err := decodeAllowedObject(fields["params"], "sql")
	if err != nil {
		return rpcRequest{}, err
	}
	switch request.Method {
	case methodStatus:
		if len(params) != 0 {
			return rpcRequest{}, errInvalidProtocol
		}
	case methodSQLQuery:
		if len(params) != 1 {
			return rpcRequest{}, errInvalidProtocol
		}
		if request.Params.SQL, err = decodeString(params["sql"]); err != nil || request.Params.SQL != provider.CanonicalProofQuery {
			return rpcRequest{}, errInvalidProtocol
		}
	default:
		return rpcRequest{}, errInvalidProtocol
	}
	if request.Version != protocolVersion || request.RequestID == "" {
		return rpcRequest{}, errInvalidProtocol
	}
	return request, nil
}

func encodeResponse(response rpcResponse) ([]byte, error) {
	if !provider.ValidateQueryResult(response.Result) {
		return nil, errInvalidProtocol
	}
	result := map[string]any{"state": response.Result.State}
	if response.Result.State == provider.QueryOK {
		result["rows"] = []map[string]string{{"value": response.Result.Rows[0].Value}}
	}
	body, err := json.Marshal(struct {
		Version   int    `json:"version"`
		RequestID string `json:"request_id"`
		Result    any    `json:"result"`
	}{response.Version, response.RequestID, result})
	if err != nil {
		return nil, errInvalidProtocol
	}
	if len(body) > maxResponseBytes {
		return nil, errBodyLimit
	}
	return body, nil
}

func decodeResponse(body []byte) (rpcResponse, error) {
	envelope, err := decodeEnvelope(body)
	if err != nil {
		return rpcResponse{}, err
	}
	result, err := decodeQueryResult(envelope.Result)
	if err != nil {
		return rpcResponse{}, err
	}
	return rpcResponse{Version: envelope.Version, RequestID: envelope.RequestID, Result: result}, nil
}

func decodeEnvelope(body []byte) (rpcEnvelope, error) {
	if len(body) > maxResponseBytes {
		return rpcEnvelope{}, errBodyLimit
	}
	fields, err := decodeExactObject(body, "version", "request_id", "result")
	if err != nil {
		return rpcEnvelope{}, err
	}
	envelope := rpcEnvelope{Result: fields["result"]}
	if err := json.Unmarshal(fields["version"], &envelope.Version); err != nil {
		return rpcEnvelope{}, errInvalidProtocol
	}
	if envelope.RequestID, err = decodeString(fields["request_id"]); err != nil {
		return rpcEnvelope{}, err
	}
	if envelope.Version != protocolVersion || envelope.RequestID == "" {
		return rpcEnvelope{}, errInvalidProtocol
	}
	return envelope, nil
}

func decodeQueryResult(raw json.RawMessage) (provider.QueryResult, error) {
	fields, err := decodeAllowedObject(raw, "state", "rows")
	if err != nil {
		return provider.QueryResult{}, err
	}
	state, err := decodeString(fields["state"])
	if err != nil {
		return provider.QueryResult{}, err
	}
	result := provider.QueryResult{State: provider.QueryState(state)}
	if result.State == provider.QueryOK {
		if len(fields) != 2 {
			return provider.QueryResult{}, errInvalidProtocol
		}
		var rows []json.RawMessage
		if err := json.Unmarshal(fields["rows"], &rows); err != nil || len(rows) != 1 {
			return provider.QueryResult{}, errInvalidProtocol
		}
		row, err := decodeExactObject(rows[0], "value")
		if err != nil || len(row) != 1 {
			return provider.QueryResult{}, errInvalidProtocol
		}
		value, err := decodeString(row["value"])
		if err != nil {
			return provider.QueryResult{}, err
		}
		result.Rows = []provider.NormalizedRow{{Value: value}}
	} else if len(fields) != 1 {
		return provider.QueryResult{}, errInvalidProtocol
	}
	if !provider.ValidateQueryResult(result) {
		return provider.QueryResult{}, errInvalidProtocol
	}
	return result, nil
}

func decodeSessionState(raw json.RawMessage) (provider.SessionState, error) {
	fields, err := decodeExactObject(raw, "state")
	if err != nil || len(fields) != 1 {
		return "", errInvalidProtocol
	}
	state, err := decodeString(fields["state"])
	if err != nil || !provider.SessionState(state).Valid() {
		return "", errInvalidProtocol
	}
	return provider.SessionState(state), nil
}

func decodeExactObject(body []byte, allowed ...string) (map[string]json.RawMessage, error) {
	fields, err := decodeAllowedObject(body, allowed...)
	if err != nil {
		return nil, err
	}
	for _, field := range allowed {
		if _, found := fields[field]; !found {
			return nil, errInvalidProtocol
		}
	}
	return fields, nil
}

func decodeAllowedObject(body []byte, allowed ...string) (map[string]json.RawMessage, error) {
	allowedFields := make(map[string]struct{}, len(allowed))
	for _, field := range allowed {
		allowedFields[field] = struct{}{}
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	token, err := decoder.Token()
	if err != nil {
		return nil, errInvalidProtocol
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return nil, errInvalidProtocol
	}
	fields := make(map[string]json.RawMessage, len(allowed))
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return nil, errInvalidProtocol
		}
		key, ok := token.(string)
		if !ok {
			return nil, errInvalidProtocol
		}
		if _, duplicate := fields[key]; duplicate {
			return nil, errInvalidProtocol
		}
		if _, permitted := allowedFields[key]; !permitted {
			return nil, errInvalidProtocol
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, errInvalidProtocol
		}
		fields[key] = value
	}
	if token, err = decoder.Token(); err != nil {
		return nil, errInvalidProtocol
	} else if delimiter, ok := token.(json.Delim); !ok || delimiter != '}' {
		return nil, errInvalidProtocol
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errInvalidProtocol
	}
	return fields, nil
}

func decodeString(raw json.RawMessage) (string, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", errInvalidProtocol
	}
	return value, nil
}

func readBoundedBody(reader io.Reader, maximum int) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, int64(maximum+1)))
	if err != nil {
		return nil, errInvalidProtocol
	}
	if len(body) > maximum {
		return nil, errBodyLimit
	}
	return body, nil
}
