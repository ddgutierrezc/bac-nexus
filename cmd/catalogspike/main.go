package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"bac-nexus/internal/catalog"
	"bac-nexus/internal/configuration"
	"bac-nexus/internal/credential"
	"bac-nexus/internal/mapepire"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/remote"
	"bac-nexus/internal/source"
)

// OrphanVaultError is re-exported for backward compatibility with
// the original catalogspike test contract. The canonical type
// lives in internal/configuration.
type OrphanVaultError = configuration.OrphanVaultError

// CommittedOutputError is re-exported for backward compatibility
// with the original catalogspike test contract. The canonical type
// lives in internal/configuration.
type CommittedOutputError = configuration.CommittedOutputError

type diagnostic struct {
	Status             string               `json:"status"`
	Query              queryDiagnostic      `json:"query"`
	MapepireVersion    string               `json:"mapepireVersion"`
	MapepireSHA256     string               `json:"mapepireSha256"`
	RemoteComponent    string               `json:"remoteComponent"`
	LaunchEnvironment  []string             `json:"launchEnvironment"`
	JavaArguments      []string             `json:"javaArguments"`
	ProtocolRequests   []protocolDiagnostic `json:"protocolRequests"`
	ArtifactVerified   bool                 `json:"artifactVerified"`
	LiveOperationBlock string               `json:"liveOperationBlock,omitempty"`
}

type queryDiagnostic struct {
	Statement      string `json:"statement"`
	ParameterCount int    `json:"parameterCount"`
	RowCap         int    `json:"rowCap"`
}

type protocolDiagnostic struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	RowCap         int    `json:"rowCap,omitempty"`
	ParameterCount int    `json:"parameterCount,omitempty"`
	ContinuationID string `json:"continuationId,omitempty"`
}

type liveOutput struct {
	Status         string              `json:"status"`
	Classification string              `json:"classification"`
	CandidateCount int                 `json:"candidateCount"`
	Candidates     []catalog.Candidate `json:"candidates,omitempty"`
	Selected       *catalog.Candidate  `json:"selected,omitempty"`
	Source         *sourceOutput       `json:"source,omitempty"`
}

type sourceOutput struct {
	RemoteSize int64  `json:"remoteSize"`
	Bytes      int    `json:"bytes"`
	Lines      int    `json:"lines"`
	Truncated  bool   `json:"truncated"`
	Cleanup    string `json:"cleanup"`
	Content    string `json:"content,omitempty"`
}

func main() {
	err := runCommand(os.Args[1:])
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		fail(err)
	}
}

func runCommand(args []string) error {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		printRootHelp(os.Stdout)
		return flag.ErrHelp
	}
	if len(args) == 0 {
		printRootHelp(os.Stderr)
		return errors.New("explicit subcommand is required")
	}
	if strings.HasPrefix(args[0], "-") {
		printRootHelp(os.Stderr)
		return fmt.Errorf("root flag %q is invalid; explicit subcommand is required", args[0])
	}
	command, args := args[0], args[1:]
	switch command {
	case "offline":
		return runOffline(args)
	case "configure":
		return runConfigure(args)
	case "live":
		return runLive(args)
	case "credentials":
		return runCredentials(args)
	case "setup":
		return runSetup(args)
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

func printRootHelp(writer io.Writer) {
	fmt.Fprintln(writer, "Usage: catalogspike <offline|configure|setup|credentials|live> [flags]")
}

func runOffline(args []string) error {
	flags := flag.NewFlagSet("offline", flag.ContinueOnError)
	item := flags.String("item", "", "catalog item name")
	productionLibrary := flags.String("production-library", "", "optional production library")
	jar := flags.String("mapepire-jar", "", "optional local Mapepire Server 2.3.5 JAR to verify")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("offline does not accept positional arguments")
	}
	query, err := catalog.BuildQuery(*item, *productionLibrary)
	if err != nil {
		return err
	}
	verified := false
	if *jar != "" {
		if err := mapepire.VerifyServerJAR(*jar); err != nil {
			return err
		}
		verified = true
	}
	return writeOfflineOutput(os.Stdout, query, verified)
}

func writeOfflineOutput(writer io.Writer, query catalog.Query, verified bool) error {
	result := diagnostic{
		Status: "offline-diagnostic", Query: queryDiagnostic{Statement: "catalogados.search.v1", ParameterCount: len(query.Parameters), RowCap: query.RowLimit},
		MapepireVersion: mapepire.ServerVersion, MapepireSHA256: mapepire.ServerSHA256,
		RemoteComponent: mapepire.RemoteJar, ProtocolRequests: protocolDiagnostics(query),
		LaunchEnvironment: mapepire.SingleModeEnvironment, JavaArguments: mapepire.SingleModeJavaArguments,
		ArtifactVerified:   verified,
		LiveOperationBlock: "use the explicit live subcommand after configuration and approvals",
	}
	return json.NewEncoder(writer).Encode(result)
}

func runConfigure(args []string) error {
	flags := flag.NewFlagSet("configure", flag.ContinueOnError)
	name := flags.String("name", "", "connection profile name")
	host := flags.String("host", "", "SSH host without a port")
	port := flags.Int("port", 22, "SSH port")
	username := flags.String("username", "", "SSH username")
	fingerprint := flags.String("host-key-sha256", "", "expected OpenSSH SHA256 host-key fingerprint")
	hostKeyTrust := flags.String("host-key-trust", "", "required host-key trust provenance: tofu or verified")
	javaHome := flags.String("java-home", "", "optional approved IBM i Java home")
	jar := flags.String("mapepire-jar", "", "optional verified local Mapepire Server 2.3.5 JAR")
	credentialMode := flags.String("credential-mode", "", "required credential mode: vault or prompt")
	configRoot := flags.String("config-root", "", "test-only profile root override")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("configure does not accept positional arguments")
	}
	root, err := resolveConfigRoot(*configRoot)
	if err != nil {
		return err
	}
	if *jar != "" {
		if err := mapepire.VerifyServerJAR(*jar); err != nil {
			return err
		}
	}
	path, err := (profile.Store{Root: root}).Save(profile.Profile{
		Name: *name, Host: *host, Port: *port, Username: *username,
		HostKeyFingerprint: *fingerprint, HostKeyTrust: profile.HostKeyTrust(*hostKeyTrust), JavaHome: *javaHome, MapepireJAR: *jar, CredentialMode: profile.CredentialMode(*credentialMode),
	})
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Status string `json:"status"`
		Name   string `json:"name"`
		Path   string `json:"path"`
	}{"configured", *name, path})
}

func runLive(args []string) error {
	flags := flag.NewFlagSet("live", flag.ContinueOnError)
	profileName := flags.String("profile", "", "configured connection profile")
	item := flags.String("item", "", "catalog item name")
	productionLibrary := flags.String("production-library", "", "optional production library")
	jar := flags.String("mapepire-jar", "", "explicit local Mapepire Server 2.3.5 JAR override")
	selectorLibrary := flags.String("selector-library", "", "exact returned source library")
	selectorFileBase := flags.String("selector-file-base", "", "exact returned source file base")
	selectorObjectType := flags.String("selector-object-type", "", "exact returned object type")
	selectorSourceType := flags.String("selector-source-type", "", "exact returned source type")
	showSource := flags.Bool("show-source", false, "print bounded source content (sensitive)")
	maxBytes := flags.Int("max-bytes", source.DefaultMaxBytes, "source byte limit")
	maxLines := flags.Int("max-lines", source.DefaultMaxLines, "source line limit")
	configRoot := flags.String("config-root", "", "test-only profile root override")
	credentialsRoot := flags.String("credentials-root", "", "test-only credential root override")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("live does not accept positional arguments")
	}
	if *profileName == "" {
		return errors.New("live requires -profile")
	}
	query, err := catalog.BuildQuery(*item, *productionLibrary)
	if err != nil {
		return err
	}
	root, err := resolveConfigRoot(*configRoot)
	if err != nil {
		return err
	}
	p, err := (profile.Store{Root: root}).Load(*profileName)
	if err != nil {
		return err
	}
	jarPath := *jar
	if jarPath == "" {
		jarPath = p.MapepireJAR
	}
	if jarPath == "" {
		return errors.New("live requires a configured Mapepire JAR or explicit -mapepire-jar override")
	}
	if err := mapepire.VerifyServerJAR(jarPath); err != nil {
		return err
	}
	vaultRoot, err := resolveCredentialsRoot(*credentialsRoot)
	if err != nil {
		return err
	}
	password, err := acquireLivePassword(credential.Store{Root: vaultRoot}, p.Name, p.CredentialMode, remote.TerminalSecretPrompt().Prompt)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	client, err := func() (*remote.Client, error) {
		defer remote.Zero(password)
		return remote.Dial(ctx, p, password)
	}()
	if err != nil {
		return err
	}
	defer client.Close()
	remoteJar, err := mapepire.EnsureServerJAR(client, jarPath)
	if err != nil {
		return err
	}
	channel, err := client.StartMapepire(ctx, p.JavaHome, remoteJar)
	if err != nil {
		return err
	}
	candidates, err := mapepire.NewSession(channel).Catalog(ctx, query)
	if err != nil {
		return err
	}
	output := liveOutput{Status: "live-complete", CandidateCount: len(candidates), Candidates: candidates}
	selected, proceed, err := selectCandidate(candidates, *item, *selectorLibrary, *selectorFileBase, *selectorObjectType, *selectorSourceType)
	if err != nil {
		return err
	}
	if !proceed {
		if len(candidates) == 0 {
			output.Classification = "not-found"
		} else if len(candidates) == 1 && !strings.EqualFold(strings.TrimSpace(candidates[0].Item), strings.TrimSpace(*item)) {
			output.Classification = "not-exact"
		} else {
			output.Classification = "ambiguous"
		}
		return writeLiveOutput(os.Stdout, output, nil, false)
	}
	output.Classification = "selected"
	output.Selected = &selected
	result, err := (source.Retriever{Files: client, Runner: client}).Retrieve(ctx, selected, *maxBytes, *maxLines)
	if err != nil {
		return err
	}
	if *showSource {
		fmt.Fprintln(os.Stderr, "WARNING: source output is sensitive and was explicitly enabled")
	}
	return writeLiveOutput(os.Stdout, output, &result, *showSource)
}

type vaultReader interface {
	Status(string) (bool, error)
	Get(string, []byte) ([]byte, error)
}

// secretReader is a type alias for configuration.SecretReader so
// the existing test contract compiles unchanged while the
// orchestration is owned by the configuration package.
type secretReader = configuration.SecretReader

func acquireLivePassword(vault vaultReader, profileName string, mode profile.CredentialMode, prompt secretReader) ([]byte, error) {
	return configuration.AcquireLivePassword(vault, profileName, mode, configuration.SecretPromptFunc(prompt))
}

func runCredentials(args []string) error {
	if len(args) == 0 {
		return errors.New("credentials requires set, status, or delete")
	}
	if args[0] == "-h" || args[0] == "--help" {
		fmt.Fprintln(os.Stdout, "Usage: catalogspike credentials <set|status|delete> [flags]")
		return flag.ErrHelp
	}
	action, args := args[0], args[1:]
	if action != "set" && action != "status" && action != "delete" {
		return fmt.Errorf("credentials unknown action %q; expected set, status, or delete", action)
	}
	flags := flag.NewFlagSet("credentials "+action, flag.ContinueOnError)
	profileName := flags.String("profile", "", "connection profile name")
	rootOverride := flags.String("credentials-root", "", "test-only credential root override")
	var replace *bool
	if action == "set" {
		replace = flags.Bool("replace", false, "explicitly rotate an existing credential")
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("credentials %s does not accept positional arguments", action)
	}
	if *profileName == "" {
		return errors.New("credentials requires -profile")
	}
	root, err := resolveCredentialsRoot(*rootOverride)
	if err != nil {
		return err
	}
	store := credential.Store{Root: root}
	switch action {
	case "set":
		password, err := remote.TerminalSecretPrompt().Prompt("IBM i password for " + *profileName)
		if err != nil {
			return err
		}
		defer credential.Zero(password)
		master, err := remote.TerminalSecretPrompt().Prompt("Vault master passphrase for " + *profileName)
		if err != nil {
			return err
		}
		defer credential.Zero(master)
		result, err := store.Set(*profileName, password, master, *replace)
		if err != nil {
			return err
		}
		return writeCredentialSetResult(os.Stdout, os.Stderr, *profileName, result)
	case "status":
		return writeCredentialStatus(os.Stdout, store, *profileName)
	case "delete":
		return writeCredentialDelete(os.Stdout, store, *profileName)
	}
	return errors.New("unreachable credentials action")
}

func writeCredentialStatus(output io.Writer, store credential.Store, profileName string) error {
	exists, err := store.Status(profileName)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(struct {
		Exists bool `json:"exists"`
	}{exists})
}

func writeCredentialDelete(output io.Writer, store credential.Store, profileName string) error {
	deleted, err := store.Delete(profileName)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(struct {
		Deleted bool `json:"deleted"`
	}{deleted})
}

func writeCredentialSetResult(stdout, stderr io.Writer, profileName string, result credential.SetResult) error {
	if !result.Committed {
		return errors.New("credential set result was not committed")
	}
	output, err := json.Marshal(struct {
		Status         string `json:"status"`
		Profile        string `json:"profile"`
		CleanupPending bool   `json:"cleanupPending"`
	}{"stored", profileName, result.CleanupWarning != nil})
	if err != nil {
		return err
	}
	output = append(output, '\n')
	operation := "credential set"
	if result.CleanupWarning != nil {
		operation = "credential rotation"
	}
	if n, err := stdout.Write(output); err != nil {
		return &CommittedOutputError{Operation: operation, Output: "stdout result", Err: err}
	} else if n != len(output) {
		return &CommittedOutputError{Operation: operation, Output: "stdout result", Err: io.ErrShortWrite}
	}
	if result.CleanupWarning != nil {
		warning := []byte("WARNING: credential rotation committed; rollback cleanup remains pending and will be retried\n")
		if n, err := stderr.Write(warning); err != nil {
			return &CommittedOutputError{Operation: "credential rotation", Output: "stderr warning", Err: err}
		} else if n != len(warning) {
			return &CommittedOutputError{Operation: "credential rotation", Output: "stderr warning", Err: io.ErrShortWrite}
		}
	}
	return nil
}

type profileStore interface {
	Save(profile.Profile) (string, error)
	List(int) ([]profile.Profile, error)
	Read(string) (profile.Profile, error)
	Update(profile.Profile, string) (profile.ProfileUpdateResult, error)
	Delete(string, profile.DeleteConfirmation) (profile.ProfileDeleteResult, error)
	Restore(string) error
}

type vaultStore interface {
	Set(string, []byte, []byte, bool) (credential.SetResult, error)
	Delete(string) (bool, error)
}

type setupDependencies struct {
	Profiles    profileStore
	Vaults      vaultStore
	ReadLine    func(string) (string, error)
	ReadExact   func(string) (string, error)
	ReadSecret  secretReader
	DiscoverJAR func() mapepire.DiscoveryResult
	VerifyJAR   func(string) error
	InspectKey  func(context.Context, string, int) (remote.HostKeyObservation, error)
	Output      io.Writer
	Notices     io.Writer
}

func runSetup(args []string) error {
	flags := flag.NewFlagSet("setup", flag.ContinueOnError)
	configRoot := flags.String("config-root", "", "test-only profile root override")
	credentialsRoot := flags.String("credentials-root", "", "test-only credential root override")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("setup does not accept positional arguments")
	}
	profileRoot, err := resolveConfigRoot(*configRoot)
	if err != nil {
		return err
	}
	vaultRoot, err := resolveCredentialsRoot(*credentialsRoot)
	if err != nil {
		return err
	}
	readLine, readExact := setupLineReaders(os.Stdin, os.Stderr)
	return executeSetup(setupDependencies{
		Profiles: profile.Store{Root: profileRoot}, Vaults: credential.Store{Root: vaultRoot},
		ReadLine: readLine, ReadExact: readExact, ReadSecret: remote.TerminalSecretPrompt().Prompt,
		DiscoverJAR: mapepire.DiscoverInstalledServerJAR, VerifyJAR: mapepire.VerifyServerJAR,
		InspectKey: remote.InspectHostKey, Output: os.Stdout, Notices: os.Stderr,
	})
}

func setupLineReaders(input io.Reader, prompts io.Writer) (func(string) (string, error), func(string) (string, error)) {
	reader := bufio.NewReader(input)
	read := func(label string) (string, error) {
		if _, err := fmt.Fprint(prompts, label+": "); err != nil {
			return "", err
		}
		value, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		if strings.HasSuffix(value, "\n") {
			value = strings.TrimSuffix(value, "\n")
			value = strings.TrimSuffix(value, "\r")
		}
		return value, nil
	}
	return func(label string) (string, error) {
		value, err := read(label)
		return strings.TrimSpace(value), err
	}, read
}

func executeSetup(deps setupDependencies) error {
	serviceDeps := configuration.Dependencies{
		Profiles:    deps.Profiles,
		Vaults:      deps.Vaults,
		ReadLine:    deps.ReadLine,
		ReadExact:   deps.ReadExact,
		ReadSecret:  deps.ReadSecret,
		DiscoverJAR: deps.DiscoverJAR,
		VerifyJAR:   deps.VerifyJAR,
		InspectKey:  deps.InspectKey,
		Output:      deps.Output,
		Notices:     deps.Notices,
	}
	return configuration.NewService(serviceDeps).Configure(context.Background())
}

func selectCandidate(candidates []catalog.Candidate, item, library, fileBase, objectType, sourceType string) (catalog.Candidate, bool, error) {
	if len(candidates) == 0 {
		return catalog.Candidate{}, false, nil
	}
	selectorCount := 0
	for _, value := range []string{library, fileBase, objectType, sourceType} {
		if value != "" {
			selectorCount++
		}
	}
	if len(candidates) == 1 && selectorCount == 0 && strings.EqualFold(strings.TrimSpace(candidates[0].Item), strings.TrimSpace(item)) {
		return candidates[0], true, nil
	}
	if selectorCount == 0 {
		return catalog.Candidate{}, false, nil
	}
	if selectorCount != 4 {
		return catalog.Candidate{}, false, errors.New("an explicit selector requires library, file base, object type, and source type")
	}
	selected, err := catalog.Select(candidates, catalog.Candidate{Item: strings.ToUpper(strings.TrimSpace(item)), SourceLibrary: library, SourceFileBase: fileBase, ObjectType: objectType, SourceType: sourceType})
	return selected, err == nil, err
}

func resolveConfigRoot(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	return profile.DefaultRoot()
}

func resolveCredentialsRoot(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	return credential.DefaultRoot()
}

func protocolDiagnostics(query catalog.Query) []protocolDiagnostic {
	return []protocolDiagnostic{
		{ID: "connect-1", Type: "connect"},
		{ID: "query-1", Type: "prepare_sql_execute", RowCap: query.RowLimit, ParameterCount: len(query.Parameters)},
		{ID: "close-1", Type: "sqlclose", ContinuationID: "query-1"},
		{ID: "exit-1", Type: "exit"},
	}
}

func writeLiveOutput(writer io.Writer, output liveOutput, result *source.Result, showSource bool) error {
	if result != nil {
		output.Source = &sourceOutput{
			RemoteSize: result.RemoteSize, Bytes: result.Bytes, Lines: result.Lines,
			Truncated: result.Truncated, Cleanup: result.Cleanup,
		}
		if showSource {
			output.Source.Content = string(result.Content)
		}
	}
	return json.NewEncoder(writer).Encode(output)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "catalog spike:", err)
	os.Exit(2)
}
