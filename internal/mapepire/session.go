package mapepire

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
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
	channel     ProtocolChannel
	application string
	mu          sync.Mutex
}

func NewSession(channel ProtocolChannel, application ...string) *Session {
	name := "Mapepire client"
	if len(application) > 0 && application[0] != "" {
		name = application[0]
	}
	return &Session{channel: channel, application: name}
}

// Execute runs one bounded prepared query over a fresh Mapepire single-mode
// session. It returns protocol rows without interpreting application columns.
func (s *Session) Execute(ctx context.Context, statement string, rowLimit int, parameters []string) ([]map[string]json.RawMessage, error) {
	if rowLimit < 1 || rowLimit > MaxQueryRows {
		return nil, ErrInvalidQueryRowLimit
	}
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

	connect, err := s.exchange(ConnectRequest("connect-1", s.application))
	if err != nil {
		return nil, contextError(ctx, err)
	}
	if connect.Job == "" {
		return nil, errors.New("Mapepire connect response omitted job identifier")
	}
	queryResponse, err := s.exchange(PreparedQueryRequest("query-1", statement, rowLimit, parameters))
	if err != nil {
		return nil, contextError(ctx, err)
	}
	if !queryResponse.HasResults {
		return nil, errors.New("Mapepire query returned no result set")
	}
	if !queryResponse.IsDone {
		return nil, errors.New("Mapepire query did not prove result-set completion")
	}
	if _, err := s.exchange(CloseCursorRequest("close-1", "query-1")); err != nil {
		return nil, fmt.Errorf("close Mapepire cursor: %w", err)
	}
	if _, err := s.exchange(ExitRequest("exit-1")); err != nil {
		return nil, fmt.Errorf("exit Mapepire session: %w", err)
	}
	return queryResponse.Data, nil
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
