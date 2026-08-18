package mapepire

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"bac-nexus/internal/catalog"
)

type ProtocolChannel interface {
	io.Reader
	io.Writer
	io.Closer
}

type Response struct {
	ID          string                       `json:"id"`
	Success     bool                         `json:"success"`
	ErrorText   string                       `json:"error,omitempty"`
	SQLCode     int                          `json:"sql_rc"`
	SQLState    string                       `json:"sql_state"`
	Job         string                       `json:"job,omitempty"`
	IsDone      bool                         `json:"is_done,omitempty"`
	HasResults  bool                         `json:"has_results,omitempty"`
	UpdateCount int                          `json:"update_count,omitempty"`
	Data        []map[string]json.RawMessage `json:"data,omitempty"`
}

type SQLError struct {
	State string
	Code  int
}

func (e *SQLError) Error() string {
	return fmt.Sprintf("Mapepire SQL request failed (SQLSTATE %s, SQL code %d)", e.State, e.Code)
}

type Session struct {
	channel ProtocolChannel
	mu      sync.Mutex
}

func NewSession(channel ProtocolChannel) *Session { return &Session{channel: channel} }

func (s *Session) Catalog(ctx context.Context, query catalog.Query) ([]catalog.Candidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	closed := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = s.channel.Close()
		case <-closed:
		}
	}()
	defer close(closed)
	defer s.channel.Close()

	connect, err := s.exchange(ConnectRequest("connect-1"))
	if err != nil {
		return nil, contextError(ctx, err)
	}
	if connect.Job == "" {
		return nil, errors.New("Mapepire connect response omitted job identifier")
	}
	queryResponse, err := s.exchange(PreparedQueryRequest("query-1", query.Statement, query.RowLimit, query.Parameters))
	if err != nil {
		return nil, contextError(ctx, err)
	}
	if !queryResponse.HasResults {
		return nil, errors.New("Mapepire Catalogados query returned no result set")
	}
	if !queryResponse.IsDone {
		return nil, errors.New("Mapepire Catalogados query did not prove result-set completion")
	}
	rows, err := decodeCandidates(queryResponse.Data)
	if err != nil {
		return nil, err
	}
	if _, err := s.exchange(CloseCursorRequest("close-1", "query-1")); err != nil {
		return nil, fmt.Errorf("close Mapepire cursor: %w", err)
	}
	if _, err := s.exchange(ExitRequest("exit-1")); err != nil {
		return nil, fmt.Errorf("exit Mapepire session: %w", err)
	}
	return catalog.BoundedCandidates(rows)
}

func contextError(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return err
}

func (s *Session) exchange(request Request) (Response, error) {
	encoded, err := Encode(request)
	if err != nil {
		return Response{}, err
	}
	if _, err := s.channel.Write(encoded); err != nil {
		return Response{}, err
	}
	var response Response
	if err := DecodeFrame(s.channel, &response); err != nil {
		return Response{}, err
	}
	if response.ID != request.ID {
		return Response{}, fmt.Errorf("Mapepire response correlation mismatch: expected %s", request.ID)
	}
	if !response.Success {
		return Response{}, &SQLError{State: response.SQLState, Code: response.SQLCode}
	}
	return response, nil
}

func decodeCandidates(rows []map[string]json.RawMessage) ([]catalog.Candidate, error) {
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
				return nil, fmt.Errorf("Mapepire row %d column %s is not a string", index+1, key)
			}
			values[strings.ToUpper(key)] = strings.TrimSpace(value)
		}
		candidate := catalog.Candidate{
			Item: values["ITEM"], SourceType: values["TIPO_DE_FUENTE"], ObjectType: values["TIPO_OBJETO"],
			Application: values["APLICACION"], Version: values["VERSION"], ProductionLibrary: values["BIBLIOTECA_PRODUCCION"],
			SourceLibrary: values["BIBLIOTECA_FUENTES"], SourceFileBase: values["ARCHIVO_FUENTES"], Description: values["DESCRIPCION"],
		}
		if candidate.Item == "" || candidate.SourceLibrary == "" || candidate.SourceFileBase == "" || candidate.ObjectType == "" || candidate.SourceType == "" {
			return nil, fmt.Errorf("Mapepire row %d omitted required Catalogados coordinates", index+1)
		}
		result = append(result, candidate)
	}
	return result, nil
}
