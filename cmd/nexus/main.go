// Package main wires the v1 MCP stdio server entry point. The
// command composes the catalog-context service from phases 2, 3B.3,
// 5A, 5B, and 6, invokes the pre-acquire recovery gate during
// startup, and runs the MCP server over stdio. It must never
// expose a generic remote, path, shell, SQL, or SSH tool.
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"bac-nexus/internal/app"
	"bac-nexus/internal/audit"
	"bac-nexus/internal/configuration"
	"bac-nexus/internal/connectors/ibmi/catalogados"
	"bac-nexus/internal/credential"
	"bac-nexus/internal/localstate"
	"bac-nexus/internal/mapepire"
	"bac-nexus/internal/mcp"
	ownershipsqlite "bac-nexus/internal/ownership/sqlite"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/release"
	"bac-nexus/internal/remote"
	"bac-nexus/internal/security"
	"bac-nexus/internal/source"
	"bac-nexus/internal/tui"
	"golang.org/x/term"
)

var releaseVersion = "dev"
var vcsRevision = "unknown"
var runConfigureTUI = tui.RunWithOnboarding
var errServeMCPUnavailable = errors.New("serve mcp unavailable")

type ownershipOpenResult struct {
	Ledger         source.OwnershipLedger
	RecoveryLedger source.RecoveryLedger
	Closer         io.Closer
}

type ownershipState struct {
	Ledger   source.OwnershipLedger
	Recovery app.RecoveryCoordinator
	Closer   io.Closer
}

var openDurableOwnershipLedger = func(root string) (ownershipOpenResult, error) {
	ledger, err := ownershipsqlite.Open(root)
	if err != nil {
		return ownershipOpenResult{}, err
	}
	return ownershipOpenResult{Ledger: ledger, RecoveryLedger: ledger, Closer: ledger}, nil
}

var loadRecoveryProfile = func(ctx context.Context, name string) (profile.Profile, error) {
	if err := ctx.Err(); err != nil {
		return profile.Profile{}, err
	}
	root, err := profile.DefaultRoot()
	if err != nil {
		return profile.Profile{}, err
	}
	loaded, err := (profile.Store{Root: root}).Load(name)
	if err != nil {
		return profile.Profile{}, err
	}
	if err := ctx.Err(); err != nil {
		return profile.Profile{}, err
	}
	return loaded, nil
}

var newRecoveryCredentialStore = func() credential.CredentialStore {
	return credential.NewNativeCredentialStore()
}

var openRecoveryCleanup = func(ctx context.Context, loaded profile.Profile, secret []byte) (source.RecoveryRemote, error) {
	client, err := remote.Dial(ctx, loaded, secret)
	if err != nil {
		return nil, err
	}
	return remote.NewRecoveryRemote(client), nil
}

var loadCatalogProfile = loadRecoveryProfile

var newCatalogCredentialStore = func() credential.CredentialStore {
	return credential.NewNativeCredentialStore()
}

var openCatalogSession = func(ctx context.Context, loaded profile.Profile, secret []byte) (catalogados.AuthenticatedSession, error) {
	client, err := remote.Dial(ctx, loaded, secret)
	if err != nil {
		return nil, err
	}
	receipt, err := client.EnsureMapepireServerJAR(ctx, loaded.MapepireJAR)
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	transport, err := client.StartMapepireTransport(ctx, receipt)
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	return catalogados.NewOwnedSession(mapepire.NewMessageSession(transport, "BAC Nexus"), client), nil
}

var loadSourceProfile = loadRecoveryProfile

var newSourceCredentialStore = func() credential.CredentialStore {
	return credential.NewNativeCredentialStore()
}

var openSourceAcquisition = func(ctx context.Context, loaded profile.Profile, secret []byte) (source.AcquisitionRemote, io.Closer, error) {
	client, err := remote.Dial(ctx, loaded, secret)
	if err != nil {
		return nil, nil, err
	}
	return remote.NewSourceAcquisitionRemote(client), client, nil
}

// newProductionCatalogResolver prepares one request-scoped, authenticated
// Catalogados resolver. Final MCP graph composition remains owned by 6.1e.
func newProductionCatalogResolver(name string) app.CatalogResolver {
	return catalogados.Resolver{Executor: catalogados.NewAuthenticatedExecutor(func(ctx context.Context) (catalogados.AuthenticatedSession, string, []byte, error) {
		loaded, err := loadCatalogProfile(ctx, name)
		if err != nil || loaded.SchemaVersion != profile.SchemaVersionV3 || loaded.CredentialMode != profile.CredentialModeKeyring || loaded.Name != name {
			return nil, "", nil, errors.New("catalog profile unavailable")
		}
		credentials := newCatalogCredentialStore()
		if credentials == nil {
			return nil, "", nil, errors.New("catalog credential unavailable")
		}
		secret, err := credentials.Get(loaded.Name)
		if err != nil {
			return nil, "", nil, errors.New("catalog credential unavailable")
		}
		if err := ctx.Err(); err != nil {
			credential.Zero(secret)
			return nil, "", nil, err
		}
		session, err := openCatalogSession(ctx, loaded, secret)
		if err != nil {
			credential.Zero(secret)
			return nil, "", nil, err
		}
		return session, loaded.Username, secret, nil
	})}
}

func main() {
	err := runCommand(os.Args[1:], os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "nexus:", err)
		os.Exit(1)
	}
}

// mainDeps groups every dependency the composition root needs.
type mainDeps struct {
	Profile          string
	Credentials      credential.CredentialStore
	Authorizer       security.Authorizer
	Auditor          audit.Auditor
	Resolver         app.CatalogResolver
	Acquirer         app.SnapshotAcquirer
	Recovery         app.RecoveryCoordinator
	Leases           app.LeaseStore
	LoadProfile      func(string) (profile.Profile, error)
	CheckEligibility func(profile.Profile, profile.EligibilityBinding, bool, time.Time) profile.EligibilityRejection
	KeyringAvailable func() bool
	Admission        func(context.Context) error
	OpenAudit        func(context.Context, profile.Profile) (audit.Auditor, io.Closer, error)
	OpenOwnership    func(context.Context, profile.Profile) (ownershipState, error)
	BuildResolver    func(profile.Profile) app.CatalogResolver
	BuildAcquirer    func(profile.Profile, source.OwnershipLedger, app.RecoveryCoordinator) app.SnapshotAcquirer
	NewLeases        func() app.LeaseStore
	ServerFactory    func(s *service) (runner, error)
	Now              func() time.Time
}

// service wraps app.Service with the mcp-facing surface so the
// composition root has a single, testable unit.
type service struct {
	app     *app.Service
	server  *mcp.Server
	profile string
}

// runner is the minimal surface the MCP server exposes. The
// production mcp.Server implements it; tests can substitute a double.
type runner interface {
	Run(ctx context.Context) error
}

// runWithDeps is the composition root. It builds the app service,
// invokes the pre-acquire recovery gate, constructs the MCP server,
// and runs the server over the supplied transport.
func runWithDeps(ctx context.Context, deps mainDeps) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(deps.Profile) == "" {
		return errors.New("nexus serve requires a non-empty profile")
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Admission != nil {
		if err := deps.Admission(ctx); err != nil {
			return fmt.Errorf("serve admission: %w", err)
		}
	}
	loaded, err := admitServeEligibility(deps)
	if err != nil {
		return err
	}
	if deps.OpenAudit == nil || deps.OpenOwnership == nil {
		return errors.New("serve local state unavailable")
	}
	durableAudit, closeAudit, err := deps.OpenAudit(ctx, loaded)
	if err != nil || durableAudit == nil || closeAudit == nil {
		return errors.New("serve audit unavailable")
	}
	defer func() {
		if closeAudit.Close() != nil && err == nil {
			err = errors.New("serve local state unavailable")
		}
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	ownership, err := deps.OpenOwnership(ctx, loaded)
	if err != nil || ownership.Ledger == nil || ownership.Recovery == nil || ownership.Closer == nil {
		return errors.New("serve ownership unavailable")
	}
	shutdownReady := false
	defer func() {
		if shutdownReady {
			deps.Leases.EvictAll()
		}
		if ownership.Closer.Close() != nil && err == nil {
			err = errors.New("serve local state unavailable")
		}
		if shutdownReady {
			if auditErr := durableAudit.Record(context.Background(), audit.Event{
				Capability: audit.CapabilityLifecycleCompletion, Connector: audit.ConnectorIBMi,
				TargetClass: audit.TargetClassLifecycle, PolicyID: audit.PolicyIDVerifiedReadOnly,
				Result: audit.ResultClassAllow, Timestamp: deps.Now(), Reason: "service lifecycle completed",
			}); auditErr != nil && err == nil {
				err = errors.New("serve lifecycle audit unavailable")
			}
		}
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ownership.Recovery.Recover(ctx); err != nil {
		return errors.New("serve recovery unavailable")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if deps.Resolver == nil {
		if deps.BuildResolver == nil {
			return errors.New("serve composition unavailable")
		}
		deps.Resolver = deps.BuildResolver(loaded)
	}
	if deps.Acquirer == nil {
		if deps.BuildAcquirer == nil {
			return errors.New("serve composition unavailable")
		}
		deps.Acquirer = deps.BuildAcquirer(loaded, ownership.Ledger, ownership.Recovery)
	}
	if deps.Leases == nil {
		if deps.NewLeases == nil {
			return errors.New("serve composition unavailable")
		}
		deps.Leases = deps.NewLeases()
	}
	if deps.ServerFactory == nil {
		deps.ServerFactory = newMCPServer
	}
	svc := app.NewService(app.ServiceDeps{
		Credentials: deps.Credentials, Authorizer: deps.Authorizer, Auditor: durableAudit,
		Resolver: deps.Resolver, Acquirer: deps.Acquirer, Leases: deps.Leases,
		Recovery: recoveredCoordinator{}, Profile: deps.Profile, Now: deps.Now,
	})
	if err := svc.Startup(ctx); err != nil {
		return errors.New("serve recovery unavailable")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	r, err := deps.ServerFactory(&service{app: svc, profile: deps.Profile})
	if err != nil {
		return fmt.Errorf("build mcp server: %w", err)
	}
	shutdownReady = true
	if err := r.Run(ctx); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return errServeMCPUnavailable
	}
	return nil
}

type recoveredCoordinator struct{}

func (recoveredCoordinator) Recover(context.Context) error { return nil }

// newMCPServer is the production ServerFactory.
func newMCPServer(s *service) (runner, error) {
	server, err := mcp.New(mcp.Config{
		Info:    mcp.Info{Name: "bac-nexus", Version: "v0.0.0"},
		Service: s.app,
	})
	if err != nil {
		return nil, err
	}
	s.server = server
	return server, nil
}

// defaultDeps returns the canonical production dependency set. The
// serve subcommand supplies the profile from flags; the resolver
// and acquirer are intentionally nil in v1 MCP.
func defaultDeps() mainDeps {
	keyring := credential.NewNativeCredentialStore()
	root, _ := profile.DefaultRoot()
	profiles := profile.Store{Root: root}
	eligibilities := profile.EligibilityStore{Root: root}
	return mainDeps{
		Credentials:      keyring,
		Authorizer:       security.NewPolicy(),
		Auditor:          audit.NewRecorder(),
		LoadProfile:      profiles.Load,
		CheckEligibility: eligibilities.Check,
		KeyringAvailable: func() bool { return keyring.Capability() == credential.CapabilitySupported },
		Admission:        operatorRetentionAdmission,
		OpenAudit:        openDurableAudit,
		OpenOwnership:    openDurableOwnership,
		BuildResolver:    func(loaded profile.Profile) app.CatalogResolver { return newProductionCatalogResolver(loaded.Name) },
		BuildAcquirer: func(loaded profile.Profile, ledger source.OwnershipLedger, recovery app.RecoveryCoordinator) app.SnapshotAcquirer {
			return newProductionSourceAcquirer(loaded, ledger, recovery)
		},
		NewLeases:     func() app.LeaseStore { return source.NewLeaseStore(time.Now, rand.Reader) },
		ServerFactory: newMCPServer,
		Now:           time.Now,
	}
}

func admitServeEligibility(deps mainDeps) (profile.Profile, error) {
	if deps.LoadProfile == nil || deps.CheckEligibility == nil || deps.KeyringAvailable == nil {
		return profile.Profile{}, errors.New("serve eligibility unavailable")
	}
	loaded, err := deps.LoadProfile(deps.Profile)
	if err != nil {
		return profile.Profile{}, errors.New("serve profile unavailable")
	}
	expected, err := profile.DeriveEligibilityBinding(loaded)
	if err != nil {
		return profile.Profile{}, errors.New("serve eligibility rejected")
	}
	if deps.CheckEligibility(loaded, expected, deps.KeyringAvailable(), deps.Now()) != profile.EligibilityApproved {
		return profile.Profile{}, errors.New("serve eligibility rejected")
	}
	return loaded, nil
}

func openDurableAudit(ctx context.Context, _ profile.Profile) (audit.Auditor, io.Closer, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	root, err := os.UserConfigDir()
	if err != nil {
		return nil, nil, audit.ErrAuditUnavailable
	}
	retention, err := audit.LoadOperatorRetention(root, localstate.NewPlatform(os.UserConfigDir))
	if err != nil {
		return nil, nil, audit.ErrAuditUnavailable
	}
	store, err := audit.OpenFile(audit.FileConfig{Root: root, RetentionDays: strconv.Itoa(retention), Components: []string{"BAC Nexus", "audit"}, Platform: localstate.NewPlatform(os.UserConfigDir)})
	if err != nil {
		return nil, nil, audit.ErrAuditUnavailable
	}
	return store, store, nil
}

func openDurableOwnership(ctx context.Context, _ profile.Profile) (ownershipState, error) {
	if err := ctx.Err(); err != nil {
		return ownershipState{}, err
	}
	root, err := os.UserConfigDir()
	if err != nil {
		return ownershipState{}, errors.New("ownership unavailable")
	}
	opened, err := openDurableOwnershipLedger(filepath.Join(root, "BAC Nexus", "ownership"))
	if err != nil {
		return ownershipState{}, errors.New("ownership unavailable")
	}
	if opened.Ledger == nil || opened.RecoveryLedger == nil || opened.Closer == nil {
		return ownershipState{}, errors.New("ownership unavailable")
	}
	credentials := newRecoveryCredentialStore()
	if credentials == nil {
		_ = opened.Closer.Close()
		return ownershipState{}, errors.New("ownership unavailable")
	}
	recovery := source.RecoveryCoordinator{
		Ledger:         opened.RecoveryLedger,
		ResolveProfile: loadRecoveryProfile,
		GetCredential: func(ctx context.Context, name string) ([]byte, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			secret, err := credentials.Get(name)
			if err != nil {
				return nil, err
			}
			if err := ctx.Err(); err != nil {
				credential.Zero(secret)
				return nil, err
			}
			return secret, nil
		},
		OpenCleanup: openRecoveryCleanup,
	}
	return ownershipState{Ledger: opened.Ledger, Recovery: recovery, Closer: opened.Closer}, nil
}

func newProductionSourceAcquirer(loaded profile.Profile, ledger source.OwnershipLedger, recovery app.RecoveryCoordinator) source.Acquirer {
	targetDigest := sourceTargetDigest(loaded)
	return source.Acquirer{
		Ownership: ledger, Profile: loaded.Name, TargetDigest: targetDigest[:], Recover: recovery.Recover, Random: rand.Reader, Now: time.Now,
		Open: func(ctx context.Context) (source.AcquisitionRemote, io.Closer, error) {
			fresh, err := loadSourceProfile(ctx, loaded.Name)
			if err != nil || fresh.SchemaVersion != profile.SchemaVersionV3 || fresh.CredentialMode != profile.CredentialModeKeyring || fresh.Name != loaded.Name {
				return nil, nil, errors.New("source profile unavailable")
			}
			credentials := newSourceCredentialStore()
			if credentials == nil {
				return nil, nil, errors.New("source credential unavailable")
			}
			secret, err := credentials.Get(fresh.Name)
			if err != nil {
				return nil, nil, errors.New("source credential unavailable")
			}
			defer credential.Zero(secret)
			if err := ctx.Err(); err != nil {
				return nil, nil, err
			}
			return openSourceAcquisition(ctx, fresh, secret)
		},
	}
}

func sourceTargetDigest(loaded profile.Profile) [sha256.Size]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte("BAC Nexus/recovery-target-binding/v1\x00"))
	writeField := func(value string) {
		var length [4]byte
		binary.BigEndian.PutUint32(length[:], uint32(len(value)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(value))
	}
	writeField(loaded.Host)
	var port [2]byte
	binary.BigEndian.PutUint16(port[:], uint16(loaded.Port))
	_, _ = hash.Write(port[:])
	writeField(loaded.Username)
	writeField(loaded.HostKeyFingerprint)
	writeField(string(loaded.HostKeyTrust))
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	return digest
}

func operatorRetentionAdmission(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root, err := os.UserConfigDir()
	if err != nil {
		return errors.New("operator configuration unavailable")
	}
	if _, err := audit.LoadOperatorRetention(root, localstate.NewPlatform(os.UserConfigDir)); err != nil {
		return errors.New("operator configuration unavailable")
	}
	return nil
}

// runCommand is the CLI entry point. It accepts "serve" and "help"
// subcommands; anything else is rejected.
func runCommand(args []string, out io.Writer) error {
	if out == nil {
		out = io.Discard
	}
	if len(args) > 0 {
		switch args[0] {
		case "-h", "--help", "help":
			return printHelp(args[1:], out)
		}
		if strings.HasPrefix(args[0], "-") {
			fmt.Fprintln(out, "root flags are not accepted; use a subcommand")
			return errors.New("root flag is invalid")
		}
	}
	if len(args) == 0 {
		fmt.Fprintln(out, "usage: nexus <subcommand> [flags]")
		fmt.Fprintln(out, "subcommands: serve, configure, version, help")
		return errors.New("explicit subcommand is required")
	}
	switch args[0] {
	case "serve":
		return runServe(args[1:], out)
	case "configure":
		return runConfigure(args[1:])
	case "version":
		return printVersion(args[1:], out)
	default:
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func printHelp(args []string, out io.Writer) error {
	if len(args) > 0 && args[0] == "serve" {
		return printServeHelp(out)
	}
	fmt.Fprintln(out, "nexus — BAC Nexus v1 MCP stdio server")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "usage: nexus <subcommand> [flags]")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "subcommands:")
	fmt.Fprintln(out, "  serve    Run the typed stdio MCP server with the two allowed tools.")
	fmt.Fprintln(out, "  configure Open the local profile configuration terminal UI.")
	fmt.Fprintln(out, "  help     Show this help text or the help for a subcommand.")
	fmt.Fprintln(out, "  version  Show the release version and VCS revision.")
	return flag.ErrHelp
}

func printVersion(args []string, out io.Writer) error {
	if len(args) > 0 && args[0] == "--json" {
		data, err := release.VersionJSON(release.Identity{Version: releaseVersion, Revision: vcsRevision})
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, string(data))
		return err
	}
	if len(args) != 0 {
		return errors.New("version accepts only --json")
	}
	_, err := fmt.Fprintf(out, "nexus version=%s revision=%s\n", releaseVersion, vcsRevision)
	return err
}

func runConfigure(args []string) error {
	if len(args) != 0 {
		return errors.New("configure accepts no arguments")
	}
	root, err := profile.DefaultRoot()
	if err != nil {
		return err
	}
	profileStore := profile.Store{Root: root}
	var store configuration.ProfilesStore = profileStore
	prompt := remote.SecretPrompt{Input: os.Stdin, Output: os.Stderr, IsTerminal: term.IsTerminal, Read: term.ReadPassword}
	return runConfigureTUI(context.Background(), store, tui.BuildInfo{Version: releaseVersion, Revision: vcsRevision}, newDirectOnboardingService(profileStore), prompt)
}

type capturedCredential struct{ secret []byte }

func (c capturedCredential) Get(ctx context.Context, _ string, _ profile.CredentialMode) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return append([]byte(nil), c.secret...), nil
}

func newDirectOnboardingService(store profile.Store) *configuration.OnboardingService {
	keyring := credential.NewNativeCredentialStore()
	onboardingAudit := profile.OnboardingAuditStore{Profiles: store}
	return configuration.NewOnboardingService(configuration.OnboardingDeps{
		Inspect: remote.InspectHostKey,
		Existing: func(_ context.Context, name string) (*profile.Profile, error) {
			existing, err := store.Load(name)
			if os.IsNotExist(err) {
				return nil, nil
			}
			if err != nil {
				return nil, err
			}
			return &existing, nil
		},
		Proof: func(ctx context.Context, p profile.Profile, secret []byte) error {
			service := newStep8ProductionRunnerWithCredentials(store, capturedCredential{secret: secret})
			request := configuration.Step8Request{RequestID: "direct-" + p.Name, Generation: 1, Profile: p, WSSConsent: true}
			result := service.Run(ctx, request)
			if result.Decision == configuration.DecisionSSHEligible {
				grant, allowed := (configuration.PolicyFallbackAuthorizer{}).Authorize(request.RequestID, request.Generation, result.FallbackClass)
				sshRequest, consented := (configuration.PolicySSHConsent{}).From(grant, request.RequestID, request.Generation)
				if !allowed || !consented {
					return errors.New("authenticated proof fallback was not authorized")
				}
				sshRequest.Profile, sshRequest.FallbackTicket = p, result.FallbackTicket
				result = service.RunSSH(ctx, sshRequest)
			}
			if result.Class != configuration.ResultProofSuccess || result.ProofRevision != mapepire.FixedProofRevision || !result.Cleanup || (result.Decision != configuration.DecisionWSSSelected && result.Decision != configuration.DecisionSSHEligible) {
				return errors.New("authenticated proof did not complete")
			}
			return nil
		},
		Save: func(p profile.Profile) error { _, err := store.Save(p); return err },
		Delete: func(name string) error {
			_, err := store.Delete(name, profile.DeleteConfirmation("delete "+name))
			return err
		},
		Audit: func(ctx context.Context, event configuration.OnboardingAuditEvent) error {
			return onboardingAudit.Record(ctx, event.Profile, event.Code)
		},
		Capability: keyring.Capability,
		Keyring:    keyring,
		Commit: func(ctx context.Context, p profile.Profile, secret []byte, auditCommitted func(context.Context) error) profile.OnboardingCommitResult {
			transactionID := "onboarding-" + p.Name
			var result profile.OnboardingCommitResult
			eligibilities := profile.EligibilityStore{Root: store.Root}
			var prior *profile.Eligibility
			err := store.WithPreparedCreateLock(ctx, p.Name, func() error {
				wasExisting := false
				result = (profile.OnboardingCommit{
					Prepare: func(context.Context) error {
						exists, err := store.Exists(p.Name)
						if err != nil {
							return err
						}
						wasExisting = exists
						return store.WritePreparedCreate(profile.PreparedCreateJournal{Profile: p.Name, TransactionID: transactionID, Phase: profile.PreparedCreateSaving})
					},
					StoreKeyring: func() error {
						if p.CredentialMode != profile.CredentialModeKeyring {
							return nil
						}
						return keyring.Set(p.Name, secret)
					},
					SaveProfile: func() error {
						if wasExisting {
							_, err := store.Update(p, p.Name)
							return err
						}
						_, err := store.Save(p)
						return err
					},
					CommitPin: func() error { return nil },
					RevokePriorEligibility: func() error {
						existing, err := eligibilities.Load(p.Name)
						if errors.Is(err, profile.ErrEligibilityMissing) {
							return nil
						}
						if err != nil {
							return err
						}
						prior = &existing
						return eligibilities.Revoke(p.Name)
					},
					SaveEligibility: func() error {
						eligibility, err := profile.NewEligibility(p, time.Now())
						if err != nil {
							return err
						}
						return eligibilities.Save(eligibility)
					},
					RollbackEligibility: func() error { return eligibilities.Revoke(p.Name) },
					RestorePriorEligibility: func() error {
						if prior == nil {
							return nil
						}
						return eligibilities.Save(*prior)
					},
					AuditCommitted: auditCommitted, RollbackPin: func() error { return nil },
					RollbackProfile: func() error {
						if wasExisting {
							return store.Restore(p.Name)
						}
						_, err := store.Delete(p.Name, profile.DeleteConfirmation("delete "+p.Name))
						return err
					},
					RollbackKeyring: func() error {
						if p.CredentialMode != profile.CredentialModeKeyring {
							return nil
						}
						return keyring.Delete(p.Name)
					},
					ClearJournal: func() error { return store.ClearPreparedCreate(p.Name) },
				}).Commit(ctx)
				return result.Err
			})
			if err != nil && result.Err == nil {
				result.Err = err
			}
			return result
		},
	})
}

type sshFingerprintObserver struct{}

func (sshFingerprintObserver) ObserveSSHFingerprint(ctx context.Context, host string, port int) (string, error) {
	observation, err := remote.InspectHostKey(ctx, host, port)
	if err != nil {
		return "", err
	}
	return observation.Fingerprint, nil
}

func newStep8ProductionRunnerWithCredentials(store profile.Store, credentials configuration.CredentialProvider) configuration.Step8Service {
	trust := security.NewStep8SSHTrustAdapter(sshFingerprintObserver{})
	return configuration.NewStep8Production(configuration.Step8ProductionDependencies{
		Observe:        configuration.NewManagedStep8PreAuth(),
		Credentials:    credentials,
		WSS:            configuration.NewManagedStep8WSS(),
		SSHPolicy:      security.NewStep8SSHPolicy(),
		SSHTrust:       trust,
		SSHCredentials: credentials,
		SSH:            configuration.NewSSHRuntimeFactory(),
		Markers:        newStep8MarkerAdapter(profile.Step8MarkerStore{Profiles: store}),
		Audit:          audit.NewStep8ConfigurationAdapter(audit.NewStep8Auditor(audit.NewRecorder())),
		NowUnixMs:      func() int64 { return time.Now().UnixMilli() },
	})
}

func printServeHelp(out io.Writer) error {
	fmt.Fprintln(out, "nexus serve — run the typed stdio MCP server")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "usage: nexus serve -profile <name> [flags]")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "flags:")
	fmt.Fprintln(out, "  -profile string   Approved Nexus profile name. Required.")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "tools exposed:")
	fmt.Fprintln(out, "  resolve_catalog_candidates   Resolve up to 50 catalog candidates for a bounded query.")
	fmt.Fprintln(out, "  read_selected_source         Read a single source page for the exact selection bound to a cursor.")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Only the two tools above are exposed. No generic surface is offered.")
	return flag.ErrHelp
}

func runServe(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(out)
	registerServeFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	profile := fs.Lookup("profile").Value.String()
	if strings.TrimSpace(profile) == "" {
		fmt.Fprintln(out, "nexus serve requires -profile <name>")
		return errors.New("serve requires a non-empty profile")
	}
	deps := defaultDeps()
	deps.Profile = profile
	return runWithDeps(context.Background(), deps)
}

func registerServeFlags(fs *flag.FlagSet) {
	fs.String("profile", "", "approved Nexus profile name (required)")
}
