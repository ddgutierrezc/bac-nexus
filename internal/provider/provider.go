// Package provider defines the transport-neutral Companion proof-query boundary.
package provider

import (
	"context"
	"strings"
	"unicode/utf8"
)

const CanonicalProofQuery = "SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1"

const maxQueryBytes = 128
const maxValueBytes = 256

type SessionState string

const (
	SessionConnected                    SessionState = "connected"
	SessionCompanionUnavailable         SessionState = "companion_unavailable"
	SessionCodeForIExtensionUnavailable SessionState = "codefori_extension_unavailable"
	SessionConnectionUnavailable        SessionState = "connection_unavailable"
)

func (state SessionState) Valid() bool {
	switch state {
	case SessionConnected, SessionCompanionUnavailable, SessionCodeForIExtensionUnavailable, SessionConnectionUnavailable:
		return true
	default:
		return false
	}
}

type QueryState string

const (
	QueryOK            QueryState = "ok"
	QueryUnavailable   QueryState = "unavailable"
	QueryInvalidQuery  QueryState = "invalid_query"
	QueryLimitExceeded QueryState = "limit_exceeded"
	QueryTimeout       QueryState = "timeout"
	QueryCancelled     QueryState = "cancelled"
	QueryFailed        QueryState = "failed"
)

func (state QueryState) Valid() bool {
	switch state {
	case QueryOK, QueryUnavailable, QueryInvalidQuery, QueryLimitExceeded, QueryTimeout, QueryCancelled, QueryFailed:
		return true
	default:
		return false
	}
}

type SessionStatusResult struct {
	State SessionState
}

type QueryRequest struct {
	SQL string
}

type QueryResult struct {
	State QueryState
	Rows  []NormalizedRow
}

type NormalizedRow struct {
	Value string
}

// Provider supplies status and the already validated proof query without knowing its transport.
type Provider interface {
	SessionStatus(context.Context) SessionStatusResult
	Query(context.Context, QueryRequest) QueryResult
}

// CanonicalizeQuery recognizes only the bounded ASCII proof query.
func CanonicalizeQuery(input string) (string, bool) {
	if len(input) > maxQueryBytes {
		return "", false
	}
	for i := range len(input) {
		if input[i] > 0x7f {
			return "", false
		}
	}
	tokens := strings.FieldsFunc(input, func(r rune) bool { return strings.ContainsRune(" \t\r\n", r) })
	if !strings.EqualFold(strings.Join(tokens, " "), CanonicalProofQuery) {
		return "", false
	}
	return CanonicalProofQuery, true
}

// ValidateQueryResult enforces the normalized, bounded result shape.
func ValidateQueryResult(result QueryResult) bool {
	if !result.State.Valid() {
		return false
	}
	if result.State != QueryOK {
		return len(result.Rows) == 0
	}
	return len(result.Rows) == 1 && len(result.Rows[0].Value) <= maxValueBytes && utf8.ValidString(result.Rows[0].Value)
}

// RunProofQuery prevents invalid or cancelled work from reaching a Provider.
// A nil Provider is unavailable; typed-nil implementations must make their methods safe.
func RunProofQuery(ctx context.Context, source Provider, request QueryRequest) QueryResult {
	if ctx.Err() != nil {
		return QueryResult{State: queryStateForContext(ctx)}
	}
	canonical, ok := CanonicalizeQuery(request.SQL)
	if !ok {
		return QueryResult{State: QueryInvalidQuery}
	}
	if source == nil {
		return QueryResult{State: QueryUnavailable}
	}
	result := source.Query(ctx, QueryRequest{SQL: canonical})
	if ctx.Err() != nil {
		return QueryResult{State: queryStateForContext(ctx)}
	}
	if !ValidateQueryResult(result) {
		return QueryResult{State: QueryFailed}
	}
	return result
}

func queryStateForContext(ctx context.Context) QueryState {
	if ctx.Err() == context.DeadlineExceeded {
		return QueryTimeout
	}
	return QueryCancelled
}
