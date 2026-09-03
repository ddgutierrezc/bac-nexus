// Package catalogados owns the BAC Catalogados SQL contract and maps its IBM i
// rows into the connector-neutral catalog domain.
package catalogados

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"bac-nexus/internal/catalog"
	"bac-nexus/internal/mapepire"
)

const rowLimit = catalog.MaxCandidates + 1

var (
	ErrCatalogUnavailable = errors.New("catalog operation unavailable")
	ErrCatalogCancelled   = errors.New("catalog operation cancelled")
	ErrCatalogTimedOut    = errors.New("catalog operation timed out")
)

// Executor is the narrow Catalogados-domain seam. It deliberately accepts no
// SQL, row limit, or parameters from callers.
type Executor interface {
	Resolve(context.Context, catalog.Search) ([]map[string]json.RawMessage, error)
}

type fixedSession interface {
	Execute(context.Context, string, int, []string) ([]map[string]json.RawMessage, error)
}

// NewFixedSessionExecutor adapts the legacy fixed-session client without
// restoring a caller-controlled SQL surface.
func NewFixedSessionExecutor(session fixedSession) Executor {
	return fixedSessionExecutor{session: session}
}

type fixedSessionExecutor struct{ session fixedSession }

func (e fixedSessionExecutor) Resolve(ctx context.Context, search catalog.Search) ([]map[string]json.RawMessage, error) {
	if err := validateSearch(search); err != nil {
		return nil, err
	}
	if e.session == nil {
		return nil, ErrCatalogUnavailable
	}
	statement, parameters, limit := PreparedSearch(search)
	return e.session.Execute(ctx, statement, limit, parameters)
}

// AuthenticatedSession is the fixed typed Mapepire surface needed for one
// Catalogados request. It deliberately has no generic execution operation.
type AuthenticatedSession interface {
	Connect(context.Context, string, []byte) error
	Call(context.Context, mapepire.Request) (mapepire.Response, error)
	Close() error
}

// AuthenticatedExecutor opens and closes one authenticated session per request.
// The resolver remains the owner of the only Catalogados SQL statement.
type AuthenticatedExecutor struct {
	Open func(context.Context) (AuthenticatedSession, string, []byte, error)
}

func NewAuthenticatedExecutor(open func(context.Context) (AuthenticatedSession, string, []byte, error)) AuthenticatedExecutor {
	return AuthenticatedExecutor{Open: open}
}

// NewOwnedSession joins a typed Mapepire session to its request-owned remote
// client. Closing catalog work releases both resources.
func NewOwnedSession(client *mapepire.Client, remote io.Closer) AuthenticatedSession {
	return &ownedSession{client: client, remote: remote}
}

type ownedSession struct {
	client *mapepire.Client
	remote io.Closer
}

func (s *ownedSession) Connect(ctx context.Context, username string, password []byte) error {
	if s == nil || s.client == nil {
		return ErrCatalogUnavailable
	}
	return s.client.Connect(ctx, username, password)
}

func (s *ownedSession) Call(ctx context.Context, request mapepire.Request) (mapepire.Response, error) {
	if s == nil || s.client == nil {
		return mapepire.Response{}, ErrCatalogUnavailable
	}
	return s.client.Call(ctx, request)
}

func (s *ownedSession) Close() error {
	if s == nil {
		return nil
	}
	if s.client != nil {
		_ = s.client.Close()
	}
	if s.remote != nil {
		return s.remote.Close()
	}
	return nil
}

func (e AuthenticatedExecutor) Resolve(ctx context.Context, search catalog.Search) ([]map[string]json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, sanitizeCatalogError(err)
	}
	if err := validateSearch(search); err != nil {
		return nil, err
	}
	if e.Open == nil {
		return nil, ErrCatalogUnavailable
	}
	session, username, password, err := e.Open(ctx)
	if err != nil || session == nil {
		zero(password)
		return nil, sanitizeCatalogError(err)
	}
	defer zero(password)
	defer session.Close()
	if username == "" || len(password) == 0 {
		return nil, ErrCatalogUnavailable
	}
	if err := session.Connect(ctx, username, password); err != nil {
		return nil, sanitizeCatalogError(err)
	}
	statement, parameters, limit := PreparedSearch(search)
	query, err := session.Call(ctx, mapepire.PreparedQueryRequest("", statement, limit, parameters))
	if err != nil {
		return nil, sanitizeCatalogError(err)
	}
	if !query.HasResults || !query.IsDone {
		return nil, ErrCatalogUnavailable
	}
	if query.ContID != "" {
		if _, err := session.Call(ctx, mapepire.CloseCursorRequest("", query.ContID)); err != nil {
			return nil, sanitizeCatalogError(err)
		}
	}
	if _, err := session.Call(ctx, mapepire.ExitRequest("")); err != nil {
		return nil, sanitizeCatalogError(err)
	}
	return query.Data, nil
}

func validateSearch(search catalog.Search) error {
	canonical, err := catalog.NewSearch(search.Item, search.ProductionLibrary)
	if err != nil || canonical != search {
		return ErrCatalogUnavailable
	}
	return nil
}

func zero(secret []byte) {
	for i := range secret {
		secret[i] = 0
	}
}

func sanitizeCatalogError(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return ErrCatalogCancelled
	case errors.Is(err, context.DeadlineExceeded):
		return ErrCatalogTimedOut
	default:
		return ErrCatalogUnavailable
	}
}

// Resolver adapts Catalogados to the app-facing semantic catalog resolver.
type Resolver struct{ Executor Executor }

func (r Resolver) Resolve(ctx context.Context, search catalog.Search) ([]catalog.Candidate, error) {
	if r.Executor == nil {
		return nil, fmt.Errorf("Catalogados executor is required")
	}
	rows, err := r.Executor.Resolve(ctx, search)
	if err != nil {
		return nil, err
	}
	return mapRows(rows)
}

// PreparedSearch returns the fixed parameterized IBM i query contract for one
// already-validated semantic search. It exists for offline diagnostics only;
// callers must not interpolate user input into the returned statement.
func PreparedSearch(search catalog.Search) (string, []string, int) {
	statement, parameters := buildQuery(search)
	return statement, parameters, rowLimit
}

func buildQuery(search catalog.Search) (string, []string) {
	productionPredicate := "UPPER(PDNAME) = UPPER(PDNAME)"
	parameters := []string{"%" + search.Item + "%"}
	if search.ProductionLibrary != "" {
		productionPredicate = "UPPER(PDNAME) = UPPER(?)"
		parameters = append(parameters, search.ProductionLibrary)
	}
	return fmt.Sprintf(`SELECT SHSNAM AS Item,
  SHSTYP AS Tipo_de_Fuente,
  SHOTYP AS Tipo_Objeto,
  PDAPPL AS Aplicacion,
  PDVERS AS Version,
  PDNAME AS Biblioteca_Produccion,
  PDSLIB AS Biblioteca_Fuentes,
  PDSFIL AS Archivo_Fuentes,
  PDDESC AS Descripcion
FROM pde400.SAHDR, pde400.SIHDR, pde400.prdmst
WHERE SHIIDÑ = SIIIDÑ
  AND SHENDR = 99999
  AND SIDCOD = ''
  AND SHAVPÑ = PDAVPÑ
  AND UPPER(SHSNAM) LIKE UPPER(?)
  AND %s
ORDER BY SHSNAM, PDSLIB, PDSFIL, SHOTYP, SHSTYP, PDNAME, PDAPPL, PDVERS
FETCH FIRST %d ROWS ONLY`, productionPredicate, rowLimit), parameters
}

func mapRows(rows []map[string]json.RawMessage) ([]catalog.Candidate, error) {
	result := make([]catalog.Candidate, 0, len(rows))
	for index, row := range rows {
		values := make(map[string]string, len(row))
		for key, raw := range row {
			if string(raw) == "null" {
				values[strings.ToUpper(key)] = ""
				continue
			}
			var value string
			if err := json.Unmarshal(raw, &value); err != nil {
				return nil, fmt.Errorf("Catalogados row %d column %s is not a string", index+1, key)
			}
			values[strings.ToUpper(key)] = strings.TrimSpace(value)
		}
		candidate := catalog.Candidate{Item: values["ITEM"], SourceType: values["TIPO_DE_FUENTE"], ObjectType: values["TIPO_OBJETO"], Application: values["APLICACION"], Version: values["VERSION"], ProductionLibrary: values["BIBLIOTECA_PRODUCCION"], SourceLibrary: values["BIBLIOTECA_FUENTES"], SourceFileBase: values["ARCHIVO_FUENTES"], Description: values["DESCRIPCION"]}
		if candidate.Item == "" || candidate.SourceLibrary == "" || candidate.SourceFileBase == "" || candidate.ObjectType == "" || candidate.SourceType == "" {
			return nil, fmt.Errorf("Catalogados row %d omitted required coordinates", index+1)
		}
		result = append(result, candidate)
	}
	return catalog.BoundedCandidates(result)
}
