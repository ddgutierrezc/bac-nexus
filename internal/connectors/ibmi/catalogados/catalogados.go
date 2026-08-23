// Package catalogados owns the BAC Catalogados SQL contract and maps its IBM i
// rows into the connector-neutral catalog domain.
package catalogados

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"bac-nexus/internal/catalog"
)

const rowLimit = catalog.MaxCandidates + 1

// Executor is the narrow, consumer-owned seam for a parameterized bounded SQL
// request. Protocol clients and future SDK adapters need not import Nexus.
type Executor interface {
	Execute(context.Context, string, int, []string) ([]map[string]json.RawMessage, error)
}

// Resolver adapts Catalogados to the app-facing semantic catalog resolver.
type Resolver struct{ Executor Executor }

func (r Resolver) Resolve(ctx context.Context, search catalog.Search) ([]catalog.Candidate, error) {
	if r.Executor == nil {
		return nil, fmt.Errorf("Catalogados executor is required")
	}
	statement, parameters, limit := PreparedSearch(search)
	rows, err := r.Executor.Execute(ctx, statement, limit, parameters)
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
