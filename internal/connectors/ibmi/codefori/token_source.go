package codefori

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	companionTokenHeader = "X-Nexus-Companion-Token"
	tokenStateFilename   = "codefori-loopback-token-v1.json"
	registryDirectory    = "codefori-loopback-v2"
	maxTokenStateBytes   = 256
	maxRegistrationBytes = 1024
	tokenBytes           = 32
)

// tokenSource supplies an optional credential for the fixed local Companion
// endpoint. It is deliberately private to this connector: no MCP input or
// output can name, read, or expose its state.
type tokenSource interface {
	Token(context.Context) (string, bool)
}

type companionTarget struct {
	endpoint, token, instance string
	generation                int64
}
type targetSource interface {
	Target(context.Context) (companionTarget, bool)
}

type tokenState struct {
	Version int    `json:"version"`
	Token   string `json:"token"`
}

type registrationState struct {
	Version    int    `json:"version"`
	Instance   string `json:"instance"`
	Endpoint   string `json:"endpoint"`
	Token      string `json:"token"`
	Generation int64  `json:"generation"`
	Connected  bool   `json:"connected"`
	Focused    bool   `json:"focused"`
	UpdatedAt  int64  `json:"updatedAt"`
	LeaseMS    int64  `json:"leaseMs"`
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
	target, ok := s.Target(ctx)
	return target.token, ok
}

// Target discovers only private v2 registrations. It chooses a single focused,
// connected, unexpired instance; any tie fails closed. Legacy v1 is consulted
// only when no valid v2 registration exists.
func (s fileTokenSource) Target(ctx context.Context) (companionTarget, bool) {
	if ctx.Err() != nil || s.path == nil {
		return companionTarget{}, false
	}
	path, err := s.path()
	if err != nil || !filepath.IsAbs(path) {
		return companionTarget{}, false
	}
	valid, candidates := s.v2(ctx, filepath.Join(filepath.Dir(path), registryDirectory))
	if valid {
		target, selected := selectTarget(candidates)
		if !selected {
			return companionTarget{instance: "blocked"}, false
		}
		return target, true
	}
	return s.v1(ctx, path)
}

func (s fileTokenSource) v2(ctx context.Context, directory string) (bool, []companionTarget) {
	info, err := os.Lstat(directory)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || !privateTokenEntry(info) {
		return true, nil
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) > 64 {
		return true, nil
	}
	if len(entries) == 0 {
		return false, nil
	}
	candidates := make([]companionTarget, 0, len(entries))
	for _, entry := range entries {
		if ctx.Err() != nil {
			return true, nil
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || len(entry.Name()) > 128 {
			return true, nil
		}
		path := filepath.Join(directory, entry.Name())
		data, ok := readPrivateTokenFile(path, maxRegistrationBytes)
		if !ok {
			return true, nil
		}
		fields, err := decodeExactObject(data, "version", "instance", "endpoint", "token", "generation", "connected", "focused", "updatedAt", "leaseMs")
		if err != nil {
			return true, nil
		}
		var state registrationState
		if json.Unmarshal(data, &state) != nil || state.Version != 2 || !validToken(state.Token) || !validRegistration(fields, state) {
			return true, nil
		}
		age := time.Now().UnixMilli() - state.UpdatedAt
		if age < 0 {
			return true, nil
		}
		if age > state.LeaseMS {
			continue
		}
		if state.Connected && state.Focused {
			candidates = append(candidates, companionTarget{"http://" + state.Endpoint, state.Token, state.Instance, state.Generation})
		}
	}
	// Expired-only records are stale crash residue, not active v2 state; legacy
	// fallback is permissible only when there are no current valid records.
	if len(candidates) == 0 {
		allExpired := true
		for _, entry := range entries {
			data, ok := readPrivateTokenFile(filepath.Join(directory, entry.Name()), maxRegistrationBytes)
			if !ok {
				return true, nil
			}
			var state registrationState
			if json.Unmarshal(data, &state) != nil {
				return true, nil
			}
			if time.Now().UnixMilli()-state.UpdatedAt <= state.LeaseMS {
				allExpired = false
			}
		}
		if allExpired {
			return false, nil
		}
	}
	return true, candidates
}

func validRegistration(_ map[string]json.RawMessage, state registrationState) bool {
	if len(state.Instance) != 43 || !validToken(state.Instance) || state.Generation < 0 || state.LeaseMS < 1000 || state.LeaseMS > 60000 || state.UpdatedAt <= 0 {
		return false
	}
	if strings.Contains(state.Endpoint, "://") {
		return false
	}
	u, err := url.Parse("http://" + state.Endpoint)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	port, err := strconv.Atoi(u.Port())
	return err == nil && port >= 1 && port <= 65535
}

func selectTarget(candidates []companionTarget) (companionTarget, bool) {
	if len(candidates) != 1 {
		return companionTarget{}, false
	}
	return candidates[0], true
}

func (s fileTokenSource) v1(ctx context.Context, path string) (companionTarget, bool) {
	directory, err := os.Lstat(filepath.Dir(path))
	if err != nil || directory.Mode()&os.ModeSymlink != 0 || !directory.IsDir() || !privateTokenEntry(directory) {
		return companionTarget{}, false
	}
	data, ok := readPrivateTokenFile(path, maxTokenStateBytes)
	if !ok || ctx.Err() != nil {
		return companionTarget{}, false
	}
	if _, err := decodeExactObject(data, "version", "token"); err != nil {
		return companionTarget{}, false
	}
	var state tokenState
	if json.Unmarshal(data, &state) != nil || state.Version != 1 || !validToken(state.Token) {
		return companionTarget{}, false
	}
	return companionTarget{endpoint: "http://127.0.0.1:64139", token: state.Token, instance: "v1"}, true
}

func validToken(value string) bool {
	if len(value) != 43 || strings.Trim(value, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_") != "" {
		return false
	}
	return true
}
