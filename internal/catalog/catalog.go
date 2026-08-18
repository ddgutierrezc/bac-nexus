package catalog

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	MaxCandidates = 50
	queryRowLimit = MaxCandidates + 1
)

var systemNamePattern = regexp.MustCompile(`^[A-Z$#@][A-Z0-9_$#@]{0,9}$`)

var ErrCandidateLimit = errors.New("catalog candidate limit exceeded")
var ErrCandidateNotFound = errors.New("catalog candidate not found")

type Query struct {
	Statement  string   `json:"statement"`
	Parameters []string `json:"parameters"`
	RowLimit   int      `json:"rowLimit"`
}

type Candidate struct {
	Item              string `json:"item"`
	SourceLibrary     string `json:"sourceLibrary"`
	SourceFileBase    string `json:"sourceFileBase"`
	ObjectType        string `json:"objectType"`
	SourceType        string `json:"sourceType"`
	Application       string `json:"application"`
	Version           string `json:"version"`
	ProductionLibrary string `json:"productionLibrary"`
	Description       string `json:"description"`
}

type AmbiguousError struct {
	Candidates []Candidate
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("catalog item is ambiguous: %d candidates", len(e.Candidates))
}

func IsSystemName(value string) bool {
	return systemNamePattern.MatchString(strings.ToUpper(value))
}

func BuildQuery(item, productionLibrary string) (Query, error) {
	item = strings.ToUpper(strings.TrimSpace(item))
	productionLibrary = strings.ToUpper(strings.TrimSpace(productionLibrary))
	if !IsSystemName(item) {
		return Query{}, fmt.Errorf("invalid catalog item %q", item)
	}
	if productionLibrary != "" && !IsSystemName(productionLibrary) {
		return Query{}, fmt.Errorf("invalid production library %q", productionLibrary)
	}

	productionPredicate := "UPPER(PDNAME) = UPPER(PDNAME)"
	parameters := []string{"%" + item + "%"}
	if productionLibrary != "" {
		productionPredicate = "UPPER(PDNAME) = UPPER(?)"
		parameters = append(parameters, productionLibrary)
	}

	statement := fmt.Sprintf(`SELECT SHSNAM AS Item,
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
FETCH FIRST %d ROWS ONLY`, productionPredicate, queryRowLimit)

	return Query{Statement: statement, Parameters: parameters, RowLimit: queryRowLimit}, nil
}

func BoundedCandidates(rows []Candidate) ([]Candidate, error) {
	if len(rows) > MaxCandidates {
		return nil, fmt.Errorf("%w: maximum is %d", ErrCandidateLimit, MaxCandidates)
	}
	return rows, nil
}

func ResolveUnique(rows []Candidate) (Candidate, error) {
	rows, err := BoundedCandidates(rows)
	if err != nil {
		return Candidate{}, err
	}
	if len(rows) == 0 {
		return Candidate{}, ErrCandidateNotFound
	}
	if len(rows) > 1 {
		return Candidate{}, &AmbiguousError{Candidates: rows}
	}
	return rows[0], nil
}

func Select(candidates []Candidate, selector Candidate) (Candidate, error) {
	matches := make([]Candidate, 0, 1)
	for _, candidate := range candidates {
		if candidate.Item == selector.Item &&
			candidate.SourceLibrary == selector.SourceLibrary &&
			candidate.SourceFileBase == selector.SourceFileBase &&
			candidate.ObjectType == selector.ObjectType &&
			candidate.SourceType == selector.SourceType {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 0 {
		return Candidate{}, fmt.Errorf("%w: explicit selector was not returned by search", ErrCandidateNotFound)
	}
	if len(matches) > 1 {
		return Candidate{}, &AmbiguousError{Candidates: matches}
	}
	return matches[0], nil
}

func (candidate Candidate) MemberPath() (string, error) {
	parts := map[string]string{
		"item": candidate.Item, "source library": candidate.SourceLibrary,
		"source file base": candidate.SourceFileBase, "object type": candidate.ObjectType,
		"source type": candidate.SourceType,
	}
	for name, value := range parts {
		if !IsSystemName(value) {
			return "", fmt.Errorf("invalid %s %q", name, value)
		}
	}
	file := strings.ToUpper(candidate.SourceFileBase + candidate.ObjectType)
	if !IsSystemName(file) {
		return "", fmt.Errorf("derived source file %q is not a valid IBM i system name", file)
	}
	return fmt.Sprintf("%s/%s/%s.%s",
		strings.ToUpper(candidate.SourceLibrary), file,
		strings.ToUpper(candidate.Item), strings.ToUpper(candidate.SourceType)), nil
}

func (candidate Candidate) QSYSPath() (string, error) {
	if _, err := candidate.MemberPath(); err != nil {
		return "", err
	}
	file := strings.ToUpper(candidate.SourceFileBase + candidate.ObjectType)
	return fmt.Sprintf("/QSYS.LIB/%s.LIB/%s.FILE/%s.MBR",
		strings.ToUpper(candidate.SourceLibrary), file, strings.ToUpper(candidate.Item)), nil
}
