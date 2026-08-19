package source

import (
	"encoding/json"
	"errors"
	"unicode/utf8"
)

const (
	// MaxPageLines caps how many complete UTF-8 lines a single source-page response contains.
	MaxPageLines = 200
	// MaxPageBytes caps how many marshaled bytes a single source-page response emits.
	MaxPageBytes = 128 << 10
)

var (
	// ErrInvalidRequest signals a malformed range before any source work:
	// startLine < 1, maxLines < 1, or maxLines > MaxPageLines.
	ErrInvalidRequest = errors.New("invalid source page request")
	// ErrRangeStartOutOfBounds signals startLine > the final record of a non-empty member.
	ErrRangeStartOutOfBounds = errors.New("source page start line is out of bounds")
	// ErrResponseTooLarge signals the marshaled source page would exceed MaxPageBytes
	// even after packing the largest contiguous prefix of complete lines that fits.
	ErrResponseTooLarge = errors.New("source page response exceeds marshaled size limit")
	// ErrInvalidSourceEncoding signals the supplied member bytes are not valid UTF-8.
	ErrInvalidSourceEncoding = errors.New("source is not valid UTF-8")
)

type lineOffset struct {
	start int
	end   int
}

// Snapshot retains an immutable UTF-8 source member and its complete-line offsets.
// Records are LF-terminated and a final unterminated record counts as one line.
type Snapshot struct {
	content []byte
	lines   []lineOffset
}

// Page is a one-based source range; Lines carry no LF delimiters.
// When EOF is true, NextStartLine is omitted; any error returns a zero Page with no lines.
type Page struct {
	StartLine     int      `json:"startLine"`
	LineCount     int      `json:"lineCount"`
	Lines         []string `json:"lines"`
	EOF           bool     `json:"eof"`
	NextStartLine int      `json:"nextStartLine,omitempty"`
}

// NewSnapshot copies the supplied member bytes (never retaining the caller's slice),
// validates UTF-8, and indexes LF-terminated records plus a final unterminated record (if any).
func NewSnapshot(content []byte) (*Snapshot, error) {
	if !utf8.Valid(content) {
		return nil, ErrInvalidSourceEncoding
	}
	copyOfContent := append([]byte(nil), content...)
	snapshot := &Snapshot{content: copyOfContent}
	lineStart := 0
	for index, value := range copyOfContent {
		if value == '\n' {
			snapshot.lines = append(snapshot.lines, lineOffset{start: lineStart, end: index})
			lineStart = index + 1
		}
	}
	if lineStart < len(copyOfContent) {
		snapshot.lines = append(snapshot.lines, lineOffset{start: lineStart, end: len(copyOfContent)})
	}
	return snapshot, nil
}

// Page returns the contiguous prefix of complete UTF-8 lines starting at startLine (one-based)
// bounded by maxLines and the marshaled-size cap (MaxPageBytes). It rejects malformed input
// before touching content; any error returns a zero Page with no partial content.
func (s *Snapshot) Page(startLine, maxLines int) (Page, error) {
	if s == nil || startLine < 1 || maxLines < 1 || maxLines > MaxPageLines {
		return Page{}, ErrInvalidRequest
	}
	if len(s.lines) == 0 {
		if startLine == 1 {
			return Page{StartLine: 1, EOF: true}, nil
		}
		return Page{}, ErrRangeStartOutOfBounds
	}
	start := startLine - 1
	if start >= len(s.lines) {
		return Page{}, ErrRangeStartOutOfBounds
	}
	page := Page{StartLine: startLine, Lines: make([]string, 0, maxLines)}
	for index := start; index < len(s.lines) && len(page.Lines) < maxLines; index++ {
		offset := s.lines[index]
		candidate := page
		candidate.Lines = append(append([]string(nil), page.Lines...), string(s.content[offset.start:offset.end]))
		candidate.LineCount = len(candidate.Lines)
		candidate.EOF = index+1 == len(s.lines)
		candidate.NextStartLine = 0
		if !candidate.EOF {
			candidate.NextStartLine = index + 2
		}
		encoded, err := json.Marshal(candidate)
		if err != nil {
			return Page{}, err
		}
		if len(encoded) > MaxPageBytes {
			if len(page.Lines) == 0 {
				return Page{}, ErrResponseTooLarge
			}
			break
		}
		page = candidate
	}
	return page, nil
}
