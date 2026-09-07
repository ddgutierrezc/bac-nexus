package codefori

import (
	"strings"
	"testing"

	"bac-nexus/internal/provider"
)

func TestDecodeResponseRejectsUnknownDuplicateAndTrailingJSON(t *testing.T) {
	valid := `{"version":1,"request_id":"request-1","result":{"state":"ok","rows":[{"value":"BACUSER"}]}}`
	tests := []struct {
		name string
		body string
	}{
		{name: "unknown field", body: strings.Replace(valid, `"result":`, `"extra":true,"result":`, 1)},
		{name: "duplicate field", body: strings.Replace(valid, `"version":1,`, `"version":1,"version":1,`, 1)},
		{name: "trailing document", body: valid + `{}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := decodeResponse([]byte(tt.body)); err == nil {
				t.Fatal("decodeResponse() succeeded, want rejection")
			}
		})
	}
}

func TestDecodeResponseRequiresExactNormalizedResult(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "unknown result field", body: `{"version":1,"request_id":"request-1","result":{"state":"ok","rows":[{"value":"BACUSER"}],"raw":"forbidden"}}`},
		{name: "duplicate normalized value", body: `{"version":1,"request_id":"request-1","result":{"state":"ok","rows":[{"value":"BACUSER","value":"OTHER"}]}}`},
		{name: "non-success rows", body: `{"version":1,"request_id":"request-1","result":{"state":"unavailable","rows":[]}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := decodeResponse([]byte(tt.body)); err == nil {
				t.Fatal("decodeResponse() succeeded, want rejection")
			}
		})
	}
}

func TestProtocolRoundTripPreservesBoundedNormalizedResult(t *testing.T) {
	encoded, err := encodeResponse(rpcResponse{
		Version:   protocolVersion,
		RequestID: "request-1",
		Result:    provider.QueryResult{State: provider.QueryOK, Rows: []provider.NormalizedRow{{Value: "BACUSER"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeResponse(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Result.State != provider.QueryOK || len(decoded.Result.Rows) != 1 || decoded.Result.Rows[0].Value != "BACUSER" {
		t.Fatalf("decoded result = %#v, want normalized success", decoded.Result)
	}
}

func TestDecodeRequestRejectsUnknownDuplicateAndInvalidParams(t *testing.T) {
	valid := `{"version":1,"request_id":"request-1","method":"sql.query","params":{"sql":"SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1"}}`
	tests := []struct {
		name string
		body string
	}{
		{name: "unknown outer field", body: strings.Replace(valid, `"method":`, `"extra":true,"method":`, 1)},
		{name: "duplicate request ID", body: strings.Replace(valid, `"request_id":"request-1",`, `"request_id":"request-1","request_id":"request-2",`, 1)},
		{name: "unknown params field", body: strings.Replace(valid, `"sql":`, `"extra":true,"sql":`, 1)},
		{name: "trailing document", body: valid + `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := decodeRequest([]byte(tt.body)); err == nil {
				t.Fatal("decodeRequest() succeeded, want rejection")
			}
		})
	}
}

func TestProtocolRejectsRemovedGenerationFields(t *testing.T) {
	request := `{"version":1,"generation":"obsolete","request_id":"request-1","method":"session.status","params":{}}`
	if _, err := decodeRequest([]byte(request)); err == nil {
		t.Fatal("decodeRequest() accepted removed generation field")
	}
	response := `{"version":1,"generation":"obsolete","request_id":"request-1","result":{"state":"unavailable"}}`
	if _, err := decodeResponse([]byte(response)); err == nil {
		t.Fatal("decodeResponse() accepted removed generation field")
	}
}

func TestProtocolEnforcesRequestAndResponseByteLimits(t *testing.T) {
	overgrown := rpcRequest{Version: protocolVersion, RequestID: strings.Repeat("r", maxRequestBytes), Method: methodSQLQuery, Params: map[string]string{}}
	overgrown.Params["sql"] = provider.CanonicalProofQuery
	if _, err := encodeRequest(overgrown); err == nil {
		t.Fatal("encodeRequest() succeeded for an oversized body")
	}
	if _, err := readBoundedBody(strings.NewReader(strings.Repeat("x", maxResponseBytes+1)), maxResponseBytes); err == nil {
		t.Fatal("readBoundedBody() succeeded for an oversized response")
	}
	if body, err := readBoundedBody(strings.NewReader(`{"state":"ok"}`), maxResponseBytes); err != nil || string(body) != `{"state":"ok"}` {
		t.Fatalf("readBoundedBody() = %q, %v", body, err)
	}
	validEnvelope := `{"version":1,"request_id":"request-1","result":{"state":"unavailable"}}`
	if _, err := decodeEnvelope([]byte(validEnvelope + strings.Repeat(" ", maxResponseBytes))); err == nil {
		t.Fatal("decodeEnvelope() succeeded for an oversized response")
	}
}
