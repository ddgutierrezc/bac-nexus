package codefori

import (
	"encoding/json"
	"unicode/utf8"
)

func decodeSourcePageResult(raw json.RawMessage) (SourcePageResult, error) {
	fields, err := decodeAllowedObject(raw, "state", "cursor", "page")
	if err != nil {
		return SourcePageResult{}, err
	}
	state, err := decodeSourceState(fields["state"])
	if err != nil {
		return SourcePageResult{}, err
	}
	if state != SourceOK {
		if len(fields) != 1 {
			return SourcePageResult{}, errInvalidProtocol
		}
		return SourcePageResult{State: state}, nil
	}
	if len(fields) != 3 {
		return SourcePageResult{}, errInvalidProtocol
	}
	cursor, err := decodeString(fields["cursor"])
	if err != nil || !validSourceCursor(cursor) {
		return SourcePageResult{}, errInvalidProtocol
	}
	pageFields, err := decodeExactObject(fields["page"], "content", "start_line", "line_count", "eof")
	if err != nil {
		return SourcePageResult{}, err
	}
	var page SourcePage
	if page.Content, err = decodeString(pageFields["content"]); err != nil || !utf8.ValidString(page.Content) || json.Unmarshal(pageFields["start_line"], &page.StartLine) != nil || json.Unmarshal(pageFields["line_count"], &page.LineCount) != nil || json.Unmarshal(pageFields["eof"], &page.EOF) != nil || page.StartLine < 1 || page.LineCount < 0 || page.LineCount > maxSourcePageLines {
		return SourcePageResult{}, errInvalidProtocol
	}
	return SourcePageResult{State: state, Cursor: cursor, Page: page}, nil
}

func decodeSourceDisposeResult(raw json.RawMessage) (SourceDisposeResult, error) {
	fields, err := decodeExactObject(raw, "state")
	if err != nil {
		return SourceDisposeResult{}, err
	}
	state, err := decodeSourceState(fields["state"])
	if err != nil || state == SourceOK {
		return SourceDisposeResult{}, errInvalidProtocol
	}
	return SourceDisposeResult{State: state}, nil
}

func decodeSourceState(raw json.RawMessage) (SourceState, error) {
	value, err := decodeString(raw)
	if err != nil {
		return "", err
	}
	state := SourceState(value)
	switch state {
	case SourceOK, SourceDisposed, SourceInvalidRequest, SourceNotFound, SourceAmbiguous, SourceUnavailable, SourceExpired, SourceInvalidEncoding, SourceResponseTooLarge, SourceCleanupFailed:
		return state, nil
	}
	return "", errInvalidProtocol
}
