package source

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"bac-nexus/internal/catalog"
)

const (
	DefaultMaxBytes  = 1 << 20
	DefaultMaxLines  = 10000
	AbsoluteMaxBytes = 4 << 20
	AbsoluteMaxLines = 50000
)

type RemoteFiles interface {
	Stat(string) (os.FileInfo, error)
	OpenRead(string) (io.ReadCloser, error)
	Remove(string) error
}

type FixedRunner interface {
	CopyToUTF8(context.Context, string, string) error
}

type Result struct {
	Coordinates catalog.Candidate `json:"coordinates"`
	RemoteSize  int64             `json:"remoteSize"`
	Bytes       int               `json:"bytes"`
	Lines       int               `json:"lines"`
	Truncated   bool              `json:"truncated"`
	Cleanup     string            `json:"cleanup"`
	Content     []byte            `json:"-"`
}

type Retriever struct {
	Files  RemoteFiles
	Runner FixedRunner
	Random io.Reader
}

func (r Retriever) Retrieve(ctx context.Context, candidate catalog.Candidate, maxBytes, maxLines int) (result Result, err error) {
	if r.Files == nil || r.Runner == nil {
		return result, errors.New("source retrieval dependencies are required")
	}
	if maxBytes < 1 || maxBytes > AbsoluteMaxBytes || maxLines < 1 || maxLines > AbsoluteMaxLines {
		return result, errors.New("source limits exceed allowed bounds")
	}
	qsysPath, err := candidate.QSYSPath()
	if err != nil {
		return result, err
	}
	random := r.Random
	if random == nil {
		random = rand.Reader
	}
	token := make([]byte, 16)
	if _, err := io.ReadFull(random, token); err != nil {
		return result, fmt.Errorf("generate remote temporary name: %w", err)
	}
	temporary := "/tmp/bac-nexus-catalog-" + hex.EncodeToString(token) + ".utf8"
	result.Coordinates = candidate
	result.Cleanup = "not-created"
	created := false
	defer func() {
		if !created {
			return
		}
		if cleanupErr := r.Files.Remove(temporary); cleanupErr != nil {
			result.Cleanup = "failed"
			cleanupErr = fmt.Errorf("remote source temporary-file cleanup failed: %w", cleanupErr)
			err = errors.Join(err, cleanupErr)
		} else {
			result.Cleanup = "removed"
		}
	}()
	created = true
	if err := r.Runner.CopyToUTF8(ctx, qsysPath, temporary); err != nil {
		return result, err
	}
	info, err := r.Files.Stat(temporary)
	if err != nil {
		return result, fmt.Errorf("stat remote source temporary file: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() < 0 {
		return result, errors.New("remote source temporary path is not a regular file")
	}
	result.RemoteSize = info.Size()
	file, err := r.Files.OpenRead(temporary)
	if err != nil {
		return result, fmt.Errorf("open remote source temporary file: %w", err)
	}
	defer file.Close()
	content, lines, truncated, err := readBounded(file, maxBytes, maxLines)
	if err != nil {
		return result, err
	}
	result.Content = content
	result.Bytes = len(content)
	result.Lines = lines
	result.Truncated = truncated || info.Size() > int64(len(content))
	return result, nil
}

func BuildCopyCommand(qsysPath, temporary string) (string, error) {
	if !strings.HasPrefix(qsysPath, "/QSYS.LIB/") || strings.Contains(qsysPath, "..") || strings.ContainsAny(qsysPath, "'\"\r\n ") {
		return "", errors.New("invalid QSYS.LIB source path")
	}
	if !strings.HasPrefix(temporary, "/") || strings.Contains(temporary, "..") || !strings.HasSuffix(temporary, ".utf8") || strings.ContainsAny(temporary, "'\"\r\n ") {
		return "", errors.New("invalid Nexus remote temporary path")
	}
	cl := "CPYTOSTMF FROMMBR('" + qsysPath + "') TOSTMF('" + temporary + "') STMFOPT(*REPLACE) STMFCCSID(1208)"
	return shellQuote("/QOpenSys/usr/bin/system") + " " + shellQuote(cl), nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func readBounded(reader io.Reader, maxBytes, maxLines int) ([]byte, int, bool, error) {
	data, err := io.ReadAll(io.LimitReader(reader, int64(maxBytes+utf8.UTFMax)))
	if err != nil {
		return nil, 0, false, err
	}
	truncated := len(data) > maxBytes
	if truncated {
		boundary := maxBytes
		for boundary > 0 && boundary > maxBytes-utf8.UTFMax && !utf8.Valid(data[:boundary]) {
			boundary--
		}
		if !utf8.Valid(data[:boundary]) {
			return nil, 0, false, errors.New("source is not valid UTF-8")
		}
		if boundary < maxBytes {
			_, size := utf8.DecodeRune(data[boundary:])
			if size == 1 {
				return nil, 0, false, errors.New("source is not valid UTF-8")
			}
		}
		data = data[:boundary]
	} else if !utf8.Valid(data) {
		return nil, 0, false, errors.New("source is not valid UTF-8")
	}
	lineEnd := len(data)
	lines := 0
	for index, value := range data {
		if value == '\n' {
			lines++
			if lines == maxLines {
				lineEnd = index + 1
				if lineEnd < len(data) {
					truncated = true
				}
				break
			}
		}
	}
	data = data[:lineEnd]
	if len(data) > 0 && !bytes.HasSuffix(data, []byte{'\n'}) {
		lines++
	}
	return data, lines, truncated, nil
}
