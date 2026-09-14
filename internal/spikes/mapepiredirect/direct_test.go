package mapepiredirect

import (
	"strings"
	"testing"

	mapepire "github.com/deady54/mapepire-go"

	"bac-nexus/internal/catalog"
	"bac-nexus/internal/connectors/ibmi/catalogados"
)

func TestCatalogadosContract(t *testing.T) {
	search, err := catalog.NewSearch("pisa061", "prod")
	if err != nil {
		t.Fatal(err)
	}
	statement, bindings, limit := catalogados.PreparedSearch(search)
	if !strings.Contains(statement, "UPPER(PDNAME) = UPPER(?)") || strings.Contains(statement, "PISA061") || limit != 51 {
		t.Fatal("non-canonical Catalogados statement")
	}
	if strings.Join(bindings, ",") != "%PISA061%,PROD" {
		t.Fatalf("bindings = %q", bindings)
	}
}

func TestResultValidation(t *testing.T) {
	good := &mapepire.ServerResponse{Success: true, HasResults: true, IsDone: true, Data: []map[string]interface{}{{"VALUE": float64(1)}}}
	if err := validateValues(good); err != nil {
		t.Fatal(err)
	}
	if err := validateValues(nil); err == nil {
		t.Fatal("nil response unexpectedly passed")
	}
	validCatalog := &mapepire.ServerResponse{Success: true, HasResults: true, IsDone: true, ParameterCount: 2, Data: []map[string]interface{}{{"item": "PISA061", "tipo_de_fuente": "RPGLE", "tipo_objeto": "PGM", "biblioteca_fuentes": "SRC", "archivo_fuentes": "QRPGLESRC"}}}
	if err := validateCatalog(validCatalog, 2); err != nil {
		t.Fatal(err)
	}
	for _, response := range []*mapepire.ServerResponse{
		nil,
		{Success: true, HasResults: true, IsDone: true, ParameterCount: 2},
		{Success: true, HasResults: true, IsDone: true, ParameterCount: 1, Data: validCatalog.Data},
		{Success: true, HasResults: true, IsDone: true, ParameterCount: 2, Data: make([]map[string]interface{}, catalog.MaxCandidates+1)},
	} {
		if err := validateCatalog(response, 2); err == nil {
			t.Fatal("invalid catalog response unexpectedly passed")
		}
	}
}

func TestSafeErrorRedactsInputs(t *testing.T) {
	message := safeError("connect").Error()
	for _, forbidden := range []string{"host", "secret", "user"} {
		if strings.Contains(message, forbidden) {
			t.Fatalf("leaked %q", forbidden)
		}
	}
}

func TestDaemonServerDisablesCertificateVerification(t *testing.T) {
	cfg := Config{Host: "host", Port: "8076", User: "user", Password: "password"}
	server := daemonServer(cfg)

	if !server.IgnoreUnauthorized {
		t.Fatal("daemon server must disable certificate verification for the disposable spike")
	}
	if server.Host != cfg.Host || server.Port != cfg.Port || server.User != cfg.User || server.Password != cfg.Password {
		t.Fatalf("daemon server = %#v, want configuration preserved", server)
	}
}
