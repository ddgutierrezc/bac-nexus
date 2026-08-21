// Package configuration owns the reusable application services for
// the Nexus configuration lifecycle. The package depends only on
// internal/profile, internal/credential, internal/remote, and
// internal/mapepire. It MUST NOT import internal/mcp, the cmd/* flag
// surface, or the stdin/stdout transport owned by the entry points.
// Future slices (TUI shell, profile CRUD, credential/trust flows,
// readiness diagnostics) consume this package as a
// service-complete adapter; the Charm v1 family is admitted out of
// band and is NOT imported in this slice.
package configuration

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"bac-nexus/internal/credential"
	"bac-nexus/internal/mapepire"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/remote"
)

// ProfilesStore is the consumer-owned profile persistence seam.
// Future slices extend this interface with List, Read, Update, and
// Delete.
type ProfilesStore interface {
	Save(p profile.Profile) (string, error)
}

// VaultsStore is the consumer-owned vault persistence seam. Future
// slices extend it with Status, Get, Rotate, and Migrate.
type VaultsStore interface {
	Set(profile string, password, master []byte, replace bool) (credential.SetResult, error)
	Delete(profile string) (bool, error)
}

// SecretReader prompts the operator for a transient secret value.
// LineReader prompts for a non-secret line of input. DiscoverJarFn
// returns the Mapepire discovery scan. VerifyJarFn verifies a
// candidate Mapepire JAR path. InspectKeyFn captures a host-key
// observation under an externally-supplied deadline.
type (
	SecretReader  func(label string) ([]byte, error)
	LineReader    func(label string) (string, error)
	DiscoverJarFn func() mapepire.DiscoveryResult
	VerifyJarFn   func(path string) error
	InspectKeyFn  func(ctx context.Context, host string, port int) (remote.HostKeyObservation, error)
)

// Dependencies wires the Service to its consumer-owned seams.
// Every field is required; an omitted field causes Configure to
// fail closed. The Service never imports the cmd/* flag surface or
// internal/mcp.
type Dependencies struct {
	Profiles    ProfilesStore
	Vaults      VaultsStore
	ReadLine    LineReader
	ReadExact   LineReader
	ReadSecret  SecretReader
	DiscoverJAR DiscoverJarFn
	VerifyJAR   VerifyJarFn
	InspectKey  InspectKeyFn
	Output      io.Writer
	Notices     io.Writer
}

func (d Dependencies) validate() error {
	// Each field is checked explicitly because Go's nil-interface
	// semantics make a generic map-iteration check incorrect for
	// function-typed fields: a nil function stored in an `any` is
	// not equal to a nil interface.
	if d.Profiles == nil {
		return errors.New("configuration dependencies are incomplete: Profiles is required")
	}
	if d.Vaults == nil {
		return errors.New("configuration dependencies are incomplete: Vaults is required")
	}
	if d.ReadLine == nil {
		return errors.New("configuration dependencies are incomplete: ReadLine is required")
	}
	if d.ReadSecret == nil {
		return errors.New("configuration dependencies are incomplete: ReadSecret is required")
	}
	if d.DiscoverJAR == nil {
		return errors.New("configuration dependencies are incomplete: DiscoverJAR is required")
	}
	if d.VerifyJAR == nil {
		return errors.New("configuration dependencies are incomplete: VerifyJAR is required")
	}
	if d.InspectKey == nil {
		return errors.New("configuration dependencies are incomplete: InspectKey is required")
	}
	if d.Output == nil {
		return errors.New("configuration dependencies are incomplete: Output is required")
	}
	if d.Notices == nil {
		return errors.New("configuration dependencies are incomplete: Notices is required")
	}
	return nil
}

// Service is the entry point of the configuration application
// layer. A Service is safe for sequential use only.
//
// Fallback: if the Charm v1 dependency admission workflow in
// .github/workflows/charm-dependency-gate.yml is denied in a
// future slice, the Service remains usable for the
// cmd/catalogspike CLI adapter without the TUI screens. The
// TUI import is the only thing that requires Charm admission.
//
// Stdio isolation: the Service has no implicit dependency on
// process stdio. Every I/O surface (Output, Notices, ReadLine,
// ReadSecret) is supplied through the Dependencies struct. The
// Service never calls os.Stdin, os.Stdout, or os.Stderr.
type Service struct{ deps Dependencies }

// NewService returns a Service bound to the supplied dependencies.
// A nil dependency produces a Service that fails closed.
func NewService(deps Dependencies) *Service { return &Service{deps: deps} }

// Configure runs the canonical setup orchestration. The body is
// identical to the previous cmd/catalogspike.executeSetup behavior;
// the cmd adapter is now a thin forwarder.
func (s *Service) Configure(ctx context.Context) error {
	if err := s.deps.validate(); err != nil {
		return err
	}
	return s.deps.runSetup(ctx)
}

// runSetup is the extracted setup orchestration. Future slices
// (CRUD, credentials, trust) reuse the same Dependencies shape.
func (d Dependencies) runSetup(ctx context.Context) error {
	readExact := d.ReadExact
	if readExact == nil {
		readExact = d.ReadLine
	}
	if _, err := fmt.Fprintln(d.Notices, "Host-key inspection is optional first-contact discovery, not independent server identity verification."); err != nil {
		return err
	}
	name, err := d.ReadLine("Connection name")
	if err != nil {
		return err
	}
	host, err := d.ReadLine("Host")
	if err != nil {
		return err
	}
	portText, err := d.ReadLine("Port [22]")
	if err != nil {
		return err
	}
	port := 22
	if portText != "" {
		port, err = strconv.Atoi(portText)
		if err != nil {
			return errors.New("port must be a number")
		}
	}
	if err := profile.ValidateEndpoint(host, port); err != nil {
		return err
	}
	hostKeyPath, err := d.ReadLine("Host-key enrollment [manual/inspect] (manual recommended; inspect is spike-only TOFU fallback)")
	if err != nil {
		return err
	}
	var fingerprint string
	var hostKeyTrust profile.HostKeyTrust
	switch hostKeyPath {
	case "inspect":
		probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		observation, inspectErr := d.InspectKey(probeCtx, host, port)
		cancel()
		if inspectErr != nil {
			return inspectErr
		}
		if observation.Verified || observation.TrustCandidate != profile.HostKeyTrustTOFU {
			return errors.New("host-key inspection returned an invalid trust observation")
		}
		if _, err := fmt.Fprintf(d.Notices, "Observed SSH host key algorithm %s with fingerprint %s\n", observation.Algorithm, observation.Fingerprint); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(d.Notices, "WARNING: this key came from the current connection and is not independently verified. Production enrollment requires an approved independent channel."); err != nil {
			return err
		}
		confirm, err := readExact("Trust this observed key for this spike? Type exact yes")
		if err != nil {
			return err
		}
		if confirm != "yes" {
			return errors.New("host-key trust was not confirmed; exact yes is required")
		}
		fingerprint = observation.Fingerprint
		hostKeyTrust = profile.HostKeyTrustTOFU
	case "manual":
		fingerprint, err = d.ReadLine("Independently verified SHA256 host-key fingerprint")
		if err != nil {
			return err
		}
		hostKeyTrust = profile.HostKeyTrustVerified
	default:
		return errors.New("host-key enrollment must be inspect or manual")
	}
	if err := profile.ValidateHostKey(fingerprint, hostKeyTrust); err != nil {
		return err
	}
	username, err := d.ReadLine("Username")
	if err != nil {
		return err
	}
	javaHome, err := d.ReadLine("Optional Java home")
	if err != nil {
		return err
	}
	discovery := d.DiscoverJAR()
	jar := ""
	automaticallyDiscovered := false
	if discovery.Status == mapepire.DiscoveryFound && discovery.VerifiedCandidateCount == 1 && discovery.Path != "" {
		jar = discovery.Path
		automaticallyDiscovered = true
	} else {
		switch {
		case discovery.Status == mapepire.DiscoveryAmbiguous:
			_, err = fmt.Fprintf(d.Notices, "Mapepire Server 2.3.5 auto-discovery found %d verified candidates; a unique candidate is required. Enter the absolute path manually.\n", discovery.VerifiedCandidateCount)
		case discovery.RejectedCandidateCount > 0:
			_, err = fmt.Fprintf(d.Notices, "Mapepire Server 2.3.5 auto-discovery found no verified candidate; %d exact-location candidate(s) failed verification. Enter the absolute path manually.\n", discovery.RejectedCandidateCount)
		case discovery.InspectionFailed:
			_, err = fmt.Fprintln(d.Notices, "Mapepire Server 2.3.5 auto-discovery could not inspect the VS Code extensions directory. Enter the absolute path manually.")
		default:
			_, err = fmt.Fprintln(d.Notices, "Mapepire Server 2.3.5 auto-discovery did not find a unique verified candidate. Enter the absolute path manually.")
		}
		if err != nil {
			return err
		}
		jar, err = d.ReadLine("Local Mapepire Server 2.3.5 JAR path")
		if err != nil {
			return err
		}
		if !filepath.IsAbs(jar) {
			return errors.New("Mapepire JAR path must be absolute")
		}
	}
	if err := d.VerifyJAR(jar); err != nil {
		return errors.New("Mapepire Server JAR verification failed")
	}
	if automaticallyDiscovered {
		if _, err := fmt.Fprintln(d.Notices, "Mapepire Server 2.3.5 was automatically found and verified."); err != nil {
			return err
		}
	}
	p := profile.Profile{
		Name: name, Host: host, Port: port, Username: username,
		HostKeyFingerprint: fingerprint, HostKeyTrust: hostKeyTrust,
		JavaHome: javaHome, MapepireJAR: jar, CredentialMode: profile.CredentialModeVault,
	}
	if err := p.Validate(); err != nil {
		return err
	}
	if jar == "" {
		return errors.New("Mapepire JAR path is required")
	}
	password, err := d.ReadSecret("IBM i password for " + name)
	if err != nil {
		return err
	}
	defer credential.Zero(password)
	master, err := d.ReadSecret("Vault master passphrase for " + name)
	if err != nil {
		return err
	}
	defer credential.Zero(master)
	confirmation, err := d.ReadSecret("Confirm vault master passphrase for " + name)
	if err != nil {
		return err
	}
	defer credential.Zero(confirmation)
	if subtle.ConstantTimeCompare(master, confirmation) != 1 {
		return errors.New("vault master passphrase confirmation does not match")
	}
	confirm, err := d.ReadLine("Create this profile and encrypted vault? [yes/no]")
	if err != nil {
		return err
	}
	if !strings.EqualFold(confirm, "yes") {
		return errors.New("setup cancelled")
	}
	output, err := json.Marshal(struct {
		Status  string          `json:"status"`
		Profile profile.Profile `json:"profile"`
	}{"configured", p})
	if err != nil {
		return errors.New("render setup result")
	}
	output = append(output, '\n')
	if _, err := d.Vaults.Set(name, password, master, false); err != nil {
		return err
	}
	if _, err := d.Profiles.Save(p); err != nil {
		if _, cleanupErr := d.Vaults.Delete(name); cleanupErr != nil {
			return errors.Join(fmt.Errorf("publish setup profile: %w", err), &OrphanVaultError{Profile: name, Err: cleanupErr})
		}
		return err
	}
	if _, err := d.Output.Write(output); err != nil {
		return &CommittedOutputError{Operation: "setup", Output: "stdout result", Err: err}
	}
	return nil
}

// VaultStatusReader is the read-only seam used by AcquireLivePassword.
type VaultStatusReader interface {
	Status(profile string) (bool, error)
	Get(profile string, master []byte) ([]byte, error)
}

// SecretPromptFunc prompts the operator for a transient secret.
type SecretPromptFunc func(label string) ([]byte, error)

// AcquireLivePassword is the canonical password acquisition path.
// It honors CredentialModeVault, fails closed when the vault is
// missing, and uses the supplied prompt for the IBM i password in
// CredentialModePrompt mode.
func AcquireLivePassword(vault VaultStatusReader, profileName string, mode profile.CredentialMode, prompt SecretPromptFunc) ([]byte, error) {
	if mode == profile.CredentialModePrompt {
		return prompt("IBM i password for " + profileName)
	}
	if mode != profile.CredentialModeVault {
		return nil, errors.New("profile credential mode is invalid")
	}
	exists, err := vault.Status(profileName)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("vault-mode profile has no credential vault; run credentials set explicitly")
	}
	master, err := prompt("Vault master passphrase for " + profileName)
	if err != nil {
		return nil, err
	}
	defer credential.Zero(master)
	return vault.Get(profileName, master)
}

// OrphanVaultError is returned when both the profile save and the
// vault cleanup fail. The error is recoverable through the
// canonical credentials status/delete command and the message never
// echoes secret material.
type OrphanVaultError struct {
	Profile string
	Err     error
}

func (e *OrphanVaultError) Error() string {
	return fmt.Sprintf("setup profile %q was not published and encrypted vault cleanup failed; recover with credentials status/delete -profile %q", e.Profile, e.Profile)
}

func (e *OrphanVaultError) Unwrap() error { return e.Err }

func (e *OrphanVaultError) Recoverable() bool { return true }

// CommittedOutputError is returned when a configuration mutation
// has already been committed (vault set, profile saved) but the
// downstream Output writer failed. The caller MUST NOT retry.
type CommittedOutputError struct {
	Operation string
	Output    string
	Err       error
}

func (e *CommittedOutputError) Error() string {
	return e.Operation + " committed but " + e.Output + " delivery failed; do not retry the mutation; query current status"
}

func (e *CommittedOutputError) Unwrap() error   { return e.Err }
func (e *CommittedOutputError) Committed() bool { return true }
