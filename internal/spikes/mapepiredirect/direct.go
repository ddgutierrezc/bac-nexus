// Package mapepiredirect implements the disposable direct Mapepire SDK spike.
package mapepiredirect

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	mapepire "github.com/deady54/mapepire-go"

	"bac-nexus/internal/catalog"
	"bac-nexus/internal/connectors/ibmi/catalogados"
)

// Config contains the ephemeral connection and bounded Catalogados lookup input.
type Config struct {
	Host, Port, User, Password string
	Search                     catalog.Search
}

// InsecureTLSWarning is emitted by the disposable spike CLI before it connects.
const InsecureTLSWarning = "WARNING: TLS certificate verification is DISABLED. This insecure behavior is SPIKE-ONLY and must not be used in production."

// Run verifies the control handshake before the SDK connection and bounded Catalogados query.
func Run(cfg Config, out io.Writer) error {
	if cfg.Host == "" || cfg.Port == "" || cfg.User == "" || cfg.Password == "" || out == nil {
		return errors.New("configuration: unavailable")
	}
	return runDiagnostic(cfg, out, controlHandshake, runSDKProof)
}

type diagnosticStep func(Config, io.Writer) error

func runDiagnostic(cfg Config, out io.Writer, control, sdk diagnosticStep) error {
	if err := control(cfg, out); err != nil {
		return safeError("control_handshake")
	}
	if err := sdk(cfg, out); err != nil {
		stage := "proof"
		var failure sdkFailure
		if errors.As(err, &failure) {
			stage = string(failure)
		}
		fmt.Fprintf(out, "sdk: failure classification=%s\n", stage)
		if stage == "connect" {
			fmt.Fprintln(out, "diagnostic: control_handshake=success sdk_connect=failed conclusion=unofficial_sdk_path_failed_after_control_success")
		}
		return safeError("sdk")
	}
	return nil
}

type sdkFailure string

func (s sdkFailure) Error() string { return "SDK proof unavailable" }

func runSDKProof(cfg Config, out io.Writer) (err error) {
	server := daemonServer(cfg)
	job := mapepire.NewSQLJob("nexus-spike-direct")
	connected := false
	defer func() {
		if connected {
			if closeErr := job.Close(); closeErr != nil && err == nil {
				err = sdkFailure("cleanup")
			}
		}
	}()

	started := time.Now()
	if err = job.Connect(server); err != nil {
		return sdkFailure("connect")
	}
	connected = true
	fmt.Fprintf(out, "sdk: success tls=certificate-verification-disabled elapsed_ms=%d\n", time.Since(started).Milliseconds())
	started = time.Now()
	if err = executeAndValidate(job, "VALUES 1", nil, 1, validateValues); err != nil {
		return sdkFailure("values")
	}
	fmt.Fprintf(out, "values: success elapsed_ms=%d\n", time.Since(started).Milliseconds())

	statement, bindings, limit := catalogados.PreparedSearch(cfg.Search)
	started = time.Now()
	query, queryErr := job.QueryWithOptions(statement, queryOptions(bindings, limit))
	if queryErr != nil {
		return sdkFailure("catalogados")
	}
	response, queryErr := query.Execute()
	if queryErr != nil || validateCatalog(response, len(bindings)) != nil {
		return sdkFailure("catalogados")
	}
	fmt.Fprintf(out, "catalogados: success parameters=%d elapsed_ms=%d\n", response.ParameterCount, time.Since(started).Milliseconds())
	return nil
}

func daemonServer(cfg Config) mapepire.DaemonServer {
	return mapepire.DaemonServer{
		Host: cfg.Host, Port: cfg.Port, User: cfg.User, Password: cfg.Password,
		IgnoreUnauthorized: true,
	}
}

func executeAndValidate(job *mapepire.SQLJob, statement string, bindings []string, rows int, validate func(*mapepire.ServerResponse) error) error {
	query, err := job.QueryWithOptions(statement, queryOptions(bindings, rows))
	if err != nil {
		return err
	}
	response, err := query.Execute()
	if err != nil {
		return err
	}
	return validate(response)
}

func queryOptions(bindings []string, rows int) mapepire.QueryOptions {
	if bindings == nil {
		return mapepire.QueryOptions{Rows: rows}
	}
	values := make([]any, len(bindings))
	for i := range bindings {
		values[i] = bindings[i]
	}
	return mapepire.QueryOptions{Rows: rows, Parameters: [][]any{values}}
}

func validateValues(response *mapepire.ServerResponse) error {
	return validateCorrelation(response, "1")
}

func validateCorrelation(response *mapepire.ServerResponse, expected string) error {
	value, err := singleValue(response)
	if err != nil || value != expected {
		return errors.New("unexpected result")
	}
	return nil
}

func singleValue(response *mapepire.ServerResponse) (string, error) {
	if response == nil || !response.Success || !response.HasResults || !response.IsDone || len(response.Data) != 1 {
		return "", errors.New("incomplete result")
	}
	for _, row := range response.Data {
		if len(row) != 1 {
			return "", errors.New("invalid result")
		}
		for _, value := range row {
			return fmt.Sprint(value), nil
		}
	}
	return "", errors.New("invalid result")
}

func validateCatalog(response *mapepire.ServerResponse, bindings int) error {
	if response == nil || !response.Success || !response.HasResults || !response.IsDone || response.ParameterCount != bindings || len(response.Data) == 0 || len(response.Data) > catalog.MaxCandidates {
		return errors.New("invalid catalog result")
	}
	for _, row := range response.Data {
		for _, coordinate := range []string{"ITEM", "TIPO_DE_FUENTE", "TIPO_OBJETO", "BIBLIOTECA_FUENTES", "ARCHIVO_FUENTES"} {
			if !validCoordinate(row, coordinate) {
				return errors.New("invalid catalog result")
			}
		}
	}
	return nil
}

func validCoordinate(row map[string]interface{}, coordinate string) bool {
	for key, value := range row {
		if strings.EqualFold(key, coordinate) {
			text, ok := value.(string)
			return ok && strings.TrimSpace(text) != ""
		}
	}
	return false
}

func safeError(stage string) error { return fmt.Errorf("%s: unavailable", stage) }
