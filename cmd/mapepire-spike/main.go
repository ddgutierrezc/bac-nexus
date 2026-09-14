// mapepire-spike is a disposable, direct Mapepire Go viability experiment.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"

	"bac-nexus/internal/catalog"
	"bac-nexus/internal/credential"
	"bac-nexus/internal/spikes/mapepiredirect"
)

type configValues struct{ host, port, user, item, productionLibrary string }

type cliDependencies struct {
	getenv      func(string) string
	prompt      func(string) (string, error)
	password    func() ([]byte, error)
	interactive func() bool
	stdout      io.Writer
}

const usage = "usage: mapepire-spike configure | run [-host HOST] [-port PORT] [-user USER] [-item ITEM] [-production-library LIBRARY]"

func main() {
	err := runCLI(os.Args[1:], cliDependencies{
		getenv:      os.Getenv,
		prompt:      terminalPrompt(os.Stdin, os.Stdout),
		password:    terminalPassword(os.Stdin, os.Stdout),
		interactive: func() bool { return term.IsTerminal(int(os.Stdin.Fd())) },
		stdout:      os.Stdout,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runCLI(args []string, deps cliDependencies) error {
	cfg, err := prepareConfig(args, deps)
	if err != nil {
		return err
	}
	if err := emitInsecureTLSWarning(deps.stdout); err != nil {
		return err
	}
	if err := mapepiredirect.Run(cfg, deps.stdout); err != nil {
		return err
	}
	if err := mapepiredirect.RunPool(cfg, deps.stdout); err != nil {
		return err
	}
	fmt.Fprintln(deps.stdout, "sdk_limits: no_context_connect_query_or_pool_wait; TLS_certificate_verification_disabled_for_spike_only; pool_error_path_may_lose_a_job")
	fmt.Fprintln(deps.stdout, "shutdown: success")
	return nil
}

func emitInsecureTLSWarning(out io.Writer) error {
	_, err := fmt.Fprintln(out, mapepiredirect.InsecureTLSWarning)
	return err
}

func prepareConfig(args []string, deps cliDependencies) (mapepiredirect.Config, error) {
	if len(args) == 0 || !deps.interactive() {
		if len(args) == 0 {
			return mapepiredirect.Config{}, errors.New(usage)
		}
		return mapepiredirect.Config{}, errors.New("terminal input unavailable")
	}
	if deps.getenv == nil || deps.prompt == nil || deps.password == nil || deps.stdout == nil {
		return mapepiredirect.Config{}, errors.New("input unavailable")
	}

	var values configValues
	switch args[0] {
	case "configure":
		if len(args) != 1 {
			return mapepiredirect.Config{}, errors.New(usage)
		}
		var err error
		values, err = promptValues(deps.prompt, configValues{})
		if err != nil {
			return mapepiredirect.Config{}, safeError("configuration")
		}
	case "run":
		parsed, err := parseRun(args[1:])
		if err != nil {
			return mapepiredirect.Config{}, err
		}
		values, err = promptValues(deps.prompt, mergedValues(parsed, deps.getenv))
		if err != nil {
			return mapepiredirect.Config{}, safeError("configuration")
		}
	default:
		return mapepiredirect.Config{}, errors.New(usage)
	}

	cfg, err := newConfig(values)
	if err != nil {
		return mapepiredirect.Config{}, safeError("configuration")
	}
	password, err := deps.password()
	defer credential.Zero(password)
	if err != nil {
		return mapepiredirect.Config{}, errors.New("password input unavailable")
	}
	if strings.TrimSpace(string(password)) == "" {
		return mapepiredirect.Config{}, errors.New("configuration: unavailable")
	}
	cfg.Password = string(password)
	return cfg, nil
}

func parseRun(args []string) (configValues, error) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var values configValues
	fs.StringVar(&values.host, "host", "", "")
	fs.StringVar(&values.port, "port", "", "")
	fs.StringVar(&values.user, "user", "", "")
	fs.StringVar(&values.item, "item", "", "")
	fs.StringVar(&values.productionLibrary, "production-library", "", "")
	if err := fs.Parse(args); err != nil || len(fs.Args()) != 0 {
		return configValues{}, errors.New(usage)
	}
	return values, nil
}

func mergedValues(flags configValues, getenv func(string) string) configValues {
	return configValues{
		host:              firstNonBlank(flags.host, getenv("BAC_NEXUS_IBMI_HOST")),
		port:              firstNonBlank(flags.port, getenv("BAC_NEXUS_IBMI_PORT")),
		user:              firstNonBlank(flags.user, getenv("BAC_NEXUS_IBMI_USER")),
		item:              firstNonBlank(flags.item, getenv("BAC_NEXUS_CATALOG_ITEM")),
		productionLibrary: firstNonBlank(flags.productionLibrary, getenv("BAC_NEXUS_CATALOG_PRODUCTION_LIBRARY")),
	}
}

func promptValues(prompt func(string) (string, error), values configValues) (configValues, error) {
	for _, field := range []struct {
		value        *string
		label        string
		defaultValue string
	}{
		{&values.host, "Host", ""},
		{&values.port, "Port", "8076"},
		{&values.user, "Username", ""},
		{&values.item, "Catalogados item", ""},
		{&values.productionLibrary, "Production library (optional)", ""},
	} {
		if strings.TrimSpace(*field.value) != "" {
			continue
		}
		value, err := prompt(field.label)
		if err != nil {
			return configValues{}, err
		}
		if strings.TrimSpace(value) == "" {
			value = field.defaultValue
		}
		*field.value = value
	}
	return values, nil
}

func newConfig(values configValues) (mapepiredirect.Config, error) {
	cfg := mapepiredirect.Config{Host: strings.TrimSpace(values.host), Port: strings.TrimSpace(values.port), User: strings.TrimSpace(values.user)}
	if cfg.Host == "" || cfg.User == "" || strings.TrimSpace(values.item) == "" {
		return mapepiredirect.Config{}, errors.New("missing required value")
	}
	port, err := strconv.Atoi(cfg.Port)
	if err != nil || port < 1 || port > 65535 {
		return mapepiredirect.Config{}, errors.New("invalid port")
	}
	cfg.Search, err = catalog.NewSearch(values.item, values.productionLibrary)
	if err != nil {
		return mapepiredirect.Config{}, errors.New("invalid catalog search")
	}
	return cfg, nil
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func terminalPrompt(in io.Reader, out io.Writer) func(string) (string, error) {
	reader := bufio.NewReader(in)
	return func(label string) (string, error) {
		if _, err := fmt.Fprintf(out, "%s: ", label); err != nil {
			return "", err
		}
		value, err := reader.ReadString('\n')
		if err != nil && len(value) == 0 {
			return "", err
		}
		return strings.TrimSuffix(strings.TrimSuffix(value, "\n"), "\r"), nil
	}
}

func terminalPassword(in *os.File, out io.Writer) func() ([]byte, error) {
	return func() ([]byte, error) {
		if _, err := fmt.Fprint(out, "Password: "); err != nil {
			return nil, err
		}
		password, err := term.ReadPassword(int(in.Fd()))
		fmt.Fprintln(out)
		return password, err
	}
}

func safeError(stage string) error { return fmt.Errorf("%s: unavailable", stage) }
