package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"bac-nexus/internal/spikes/mapepiredirect"
)

func TestEmitInsecureTLSWarning(t *testing.T) {
	var out bytes.Buffer
	if err := emitInsecureTLSWarning(&out); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), mapepiredirect.InsecureTLSWarning+"\n"; got != want {
		t.Fatalf("warning = %q, want %q", got, want)
	}
	for _, forbidden := range []string{"host", "port", "user", "password", "secret"} {
		if strings.Contains(strings.ToLower(out.String()), forbidden) {
			t.Fatalf("warning leaked %q", forbidden)
		}
	}
}

func TestParseRunRejectsSecretsAndUnsupportedArguments(t *testing.T) {
	for _, args := range [][]string{{"-password", "secret"}, {"-insecure-tls"}, {"-unknown"}, {"unexpected"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			if _, err := parseRun(args); err == nil || err.Error() != usage {
				t.Fatalf("parseRun(%q) error = %v, want sanitized usage", args, err)
			}
		})
	}
}

func TestPrepareConfigCommandParsingAndPrecedence(t *testing.T) {
	env := map[string]string{
		"BAC_NEXUS_IBMI_HOST":                  "env-host",
		"BAC_NEXUS_IBMI_PORT":                  "9000",
		"BAC_NEXUS_IBMI_USER":                  "env-user",
		"BAC_NEXUS_CATALOG_ITEM":               "ENVITEM",
		"BAC_NEXUS_CATALOG_PRODUCTION_LIBRARY": "ENVLIB",
		"BAC_NEXUS_IBMI_PASSWORD":              "environment-password-must-be-ignored",
	}
	for _, tt := range []struct {
		name, host, port, user, item, library string
		args                                  []string
		prompts                               []string
	}{
		{"configure guides values", "guided-host", "8076", "guided-user", "GUIDEDITEM", "", []string{"configure"}, []string{"guided-host", "", "guided-user", "GUIDEDITEM", ""}},
		{"flags precede environment", "flag-host", "8076", "flag-user", "FLAGITEM", "FLAGLIB", []string{"run", "-host", "flag-host", "-port", "8076", "-user", "flag-user", "-item", "FLAGITEM", "-production-library", "FLAGLIB"}, nil},
		{"environment precedes prompts", "env-host", "9000", "env-user", "ENVITEM", "ENVLIB", []string{"run"}, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := prepareConfig(tt.args, testDependencies(env, tt.prompts))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Host != tt.host || cfg.Port != tt.port || cfg.User != tt.user || cfg.Search.Item != tt.item || cfg.Search.ProductionLibrary != tt.library || cfg.Password != " secret " {
				t.Fatalf("config = %#v", cfg)
			}
		})
	}
}

func TestPrepareConfigValidatesBeforeAndAfterPasswordCapture(t *testing.T) {
	deps := testDependencies(nil, nil)
	passwordCalls := 0
	deps.password = func() ([]byte, error) { passwordCalls++; return []byte("secret"), nil }
	if _, err := prepareConfig([]string{"run", "-password", "secret"}, deps); err == nil || passwordCalls != 0 {
		t.Fatal("invalid input reached password capture")
	}

	args := []string{"run", "-host", "host", "-port", "8076", "-user", "user", "-item", "ITEM"}
	for _, tt := range []struct {
		name  string
		value []byte
		err   error
	}{
		{"blank password", []byte(" \t "), nil},
		{"provider error", []byte("provider-error"), errors.New("unavailable")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			password := tt.value
			deps := testDependencies(nil, nil)
			deps.password = func() ([]byte, error) { return password, tt.err }
			if _, err := prepareConfig(args, deps); err == nil {
				t.Fatal("prepareConfig unexpectedly succeeded")
			}
			assertZeroed(t, password)
		})
	}
}

func TestPrepareConfigZeroesSuccessfulPassword(t *testing.T) {
	password := []byte("\t secret \t")
	deps := testDependencies(nil, nil)
	deps.password = func() ([]byte, error) { return password, nil }
	cfg, err := prepareConfig([]string{"run", "-host", "host", "-port", "8076", "-user", "user", "-item", "ITEM"}, deps)
	if err != nil || cfg.Password != "\t secret \t" {
		t.Fatalf("config = %#v, error = %v", cfg, err)
	}
	assertZeroed(t, password)
}

func TestPrepareConfigRequiresPasswordProvider(t *testing.T) {
	deps := testDependencies(nil, nil)
	deps.password = nil
	if _, err := prepareConfig([]string{"run", "-host", "host", "-port", "8076", "-user", "user", "-item", "ITEM"}, deps); err == nil || err.Error() != "input unavailable" {
		t.Fatalf("missing password provider error = %v", err)
	}
}

func TestPrepareConfigSanitizesInputFailures(t *testing.T) {
	secretHost, secretUser, secretPassword := "private-host", "private-user", "private-password"
	deps := testDependencies(nil, nil)
	deps.interactive = func() bool { return false }
	for _, args := range [][]string{{"run", "-host", secretHost, "-user", secretUser, "-item", "ITEM"}, {"run", "-password", secretPassword}} {
		_, err := prepareConfig(args, deps)
		for _, forbidden := range []string{secretHost, secretUser, secretPassword} {
			if err == nil || strings.Contains(err.Error(), forbidden) {
				t.Fatalf("error leaked %q: %v", forbidden, err)
			}
		}
	}
}

func testDependencies(env map[string]string, prompts []string) cliDependencies {
	return cliDependencies{
		getenv: func(key string) string { return env[key] },
		prompt: func(string) (string, error) {
			if len(prompts) == 0 {
				return "", nil
			}
			value := prompts[0]
			prompts = prompts[1:]
			return value, nil
		},
		password:    func() ([]byte, error) { return []byte(" secret "), nil },
		interactive: func() bool { return true },
		stdout:      io.Discard,
	}
}

func assertZeroed(t *testing.T, value []byte) {
	t.Helper()
	for _, b := range value {
		if b != 0 {
			t.Fatal("password buffer was not zeroed")
		}
	}
}
