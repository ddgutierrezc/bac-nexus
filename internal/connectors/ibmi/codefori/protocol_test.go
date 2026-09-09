package codefori

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"bac-nexus/internal/catalog"
	"bac-nexus/internal/provider"
)

func TestCatalogProtocolIsStrictAndBounded(t *testing.T) {
	valid := `{"version":1,"request_id":"request-1","method":"catalog.resolve_candidates.v1","params":{"item":"PISA061","productionLibrary":"PRODLIB"}}`
	for _, body := range []string{valid, strings.Replace(valid, `"params":`, `"extra":true,"params":`, 1), strings.Replace(valid, `"item":`, `"item":"OTHER","item":`, 1), valid + `{}`} {
		request, err := decodeRequest([]byte(body))
		if body == valid {
			if err != nil || request.Method != methodResolveCatalog || request.Params["item"] != "PISA061" || request.Params["productionLibrary"] != "PRODLIB" {
				t.Fatalf("decodeRequest() = %#v, %v", request, err)
			}
		} else if err == nil {
			t.Fatalf("decodeRequest accepted %s", body)
		}
	}
	validResult := `{"state":"ok","candidates":[{"item":"PISA061","sourceLibrary":"SRCLIB","sourceFileBase":"QRPG","objectType":"M","sourceType":"RPGLE","application":"APP","version":"V1","productionLibrary":"PRODLIB","description":"description"}]}`
	for _, raw := range []string{validResult, strings.Replace(validResult, `"description":`, `"raw":"forbidden","description":`, 1), `{"state":"ok","candidates":null}`, `{"state":"candidate_limit_exceeded","candidates":[]}`} {
		result, err := decodeCatalogResult([]byte(raw))
		if raw == validResult {
			if err != nil || len(result.Candidates) != 1 || result.Candidates[0].SourceLibrary != "SRCLIB" {
				t.Fatalf("decodeCatalogResult() = %#v, %v", result, err)
			}
		} else if err == nil {
			t.Fatalf("decodeCatalogResult accepted %s", raw)
		}
	}
	candidate := `{"item":"PISA061","sourceLibrary":"SRCLIB","sourceFileBase":"QRPG","objectType":"M","sourceType":"RPGLE","application":"APP","version":"V1","productionLibrary":"PRODLIB","description":"` + strings.Repeat("x", 256) + `"}`
	rows := strings.TrimSuffix(strings.Repeat(candidate+",", catalog.MaxCandidates), ",")
	if result, err := decodeCatalogResult([]byte(`{"state":"ok","candidates":[` + rows + `]}`)); err != nil || len(result.Candidates) != catalog.MaxCandidates {
		t.Fatalf("50 candidates = %#v, %v", result, err)
	}
	if _, err := decodeCatalogResult([]byte(`{"state":"ok","candidates":[` + rows + `,` + candidate + `]}`)); err == nil {
		t.Fatal("51 candidates accepted")
	}
	for _, id := range []string{"", "é", strings.Repeat("a", 129)} {
		request := rpcRequest{Version: protocolVersion, RequestID: id, Method: methodResolveCatalog, Params: map[string]string{"item": "PISA061"}}
		if _, err := encodeRequest(request); err == nil {
			t.Fatalf("encodeRequest accepted request ID %q", id)
		}
		body := `{"version":1,"request_id":` + strconv.Quote(id) + `,"result":{"state":"unavailable"}}`
		if _, err := decodeEnvelope([]byte(body)); err == nil {
			t.Fatalf("decodeEnvelope accepted request ID %q", id)
		}
	}
	base := `{"state":"ok","candidates":[{"item":"PISA061","sourceLibrary":"SRCLIB","sourceFileBase":"QRPG","objectType":"M","sourceType":"RPGLE","application":"APP","version":"V1","productionLibrary":"PRODLIB","description":"%s"}]}`
	for _, tt := range []struct {
		name, value string
		want        bool
	}{{"accepts 256 UTF-8 bytes", strings.Repeat("é", 128), true}, {"rejects 257 UTF-8 bytes", strings.Repeat("é", 128) + "a", false}} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := decodeCatalogResult([]byte(fmt.Sprintf(base, tt.value)))
			if (err == nil) != tt.want {
				t.Fatalf("decodeCatalogResult() error = %v, want success %t", err, tt.want)
			}
		})
	}
}

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
