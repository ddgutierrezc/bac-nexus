package codefori

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	companionTokenHeader = "X-Nexus-Companion-Token"
	tokenStateFilename   = "codefori-loopback-token-v1.json"
	maxTokenStateBytes   = 256
	tokenBytes           = 32
)

// tokenSource supplies an optional credential for the fixed local Companion
// endpoint. It is deliberately private to this connector: no MCP input or
// output can name, read, or expose its state.
type tokenSource interface {
	Token(context.Context) (string, bool)
}

type tokenState struct {
	Version int    `json:"version"`
	Token   string `json:"token"`
}

type fileTokenSource struct {
	path func() (string, error)
}

func newFileTokenSource(path func() (string, error)) tokenSource {
	return fileTokenSource{path: path}
}

// defaultTokenStatePath is the v1 private-state contract for the Companion:
// XDG_CONFIG_HOME/$HOME/.config on Unix, ~/Library/Application Support on
// macOS, and %AppData% on Windows, beneath bac-nexus. It is never user input.
func defaultTokenStatePath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "bac-nexus", tokenStateFilename), nil
}

func (s fileTokenSource) Token(ctx context.Context) (string, bool) {
	if ctx.Err() != nil || s.path == nil {
		return "", false
	}
	path, err := s.path()
	if err != nil || !filepath.IsAbs(path) {
		return "", false
	}
	directory, err := os.Lstat(filepath.Dir(path))
	if err != nil || directory.Mode()&os.ModeSymlink != 0 || !directory.IsDir() || !privateTokenEntry(directory) {
		return "", false
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || !privateTokenEntry(info) {
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) > maxTokenStateBytes || ctx.Err() != nil {
		return "", false
	}
	if _, err := decodeExactObject(data, "version", "token"); err != nil {
		return "", false
	}
	var state tokenState
	if json.Unmarshal(data, &state) != nil || state.Version != 1 || !validToken(state.Token) {
		return "", false
	}
	return state.Token, true
}

func validToken(value string) bool {
	if len(value) != 43 || strings.Trim(value, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_") != "" {
		return false
	}
	return true
}
