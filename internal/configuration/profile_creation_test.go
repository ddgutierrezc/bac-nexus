package configuration_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"bac-nexus/internal/configuration"
	"bac-nexus/internal/credential"
	"bac-nexus/internal/profile"
)

func TestPreparedCreateRejectsExistingCredentialWithoutChangingProfile(t *testing.T) {
	store := profile.Store{Root: t.TempDir()}
	credentials := &fakePreparedCredentials{presence: credential.PresencePresent}
	creator := configuration.NewProfileCreator(store, credentials)
	_, err := creator.Create(context.Background(), createRequest("request-existing", 1, "digest-existing"))
	if !errors.Is(err, configuration.ErrCredentialUnavailable) {
		t.Fatalf("Create() error = %v, want ErrCredentialUnavailable", err)
	}
	if credentials.provisionCalls != 0 {
		t.Fatalf("provision calls = %d, want 0", credentials.provisionCalls)
	}
	if _, err := store.Read("wizard"); !errors.Is(err, profile.ErrProfileNotFound) {
		t.Fatalf("profile changed after rejected create: %v", err)
	}
}

func TestPromptCreateSavesLocalProfileWithoutCredentialProvisioning(t *testing.T) {
	store := profile.Store{Root: t.TempDir()}
	credentials := &fakePreparedCredentials{presence: credential.PresenceUnavailable}
	creator := configuration.NewProfileCreator(store, credentials)
	request := createRequest("request-prompt", 1, "digest-prompt")
	request.Profile.CredentialMode = profile.CredentialModePrompt
	result, err := creator.Create(context.Background(), request)
	if err != nil || result.Profile != request.Profile {
		t.Fatalf("prompt Create() = %#v, %v", result, err)
	}
	if credentials.provisionCalls != 0 {
		t.Fatalf("prompt provision calls = %d, want 0", credentials.provisionCalls)
	}
	if _, err := store.Load(request.Profile.Name); err != nil {
		t.Fatalf("prompt profile was not saved locally: %v", err)
	}
}
func TestPreparedCreateRejectsExistingProfileBeforeCredentialProvisioning(t *testing.T) {
	store := profile.Store{Root: t.TempDir()}
	request := createRequest("request-profile-exists", 1, "digest-profile-exists")
	if _, err := store.Save(request.Profile); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	request.Profile.CredentialMode = profile.CredentialModePrompt
	credentials := &fakePreparedCredentials{presence: credential.PresenceAbsent}
	creator := configuration.NewProfileCreator(store, credentials)
	_, err := creator.Create(context.Background(), request)
	if !errors.Is(err, configuration.ErrProfileAlreadyExists) {
		t.Fatalf("Create() error = %v, want ErrProfileAlreadyExists", err)
	}
	if credentials.provisionCalls != 0 {
		t.Fatalf("provision calls = %d, want 0", credentials.provisionCalls)
	}
}

func TestPreparedCreateProvisionFailureLeavesNoProfileAndRequiresRecovery(t *testing.T) {
	store := profile.Store{Root: t.TempDir()}
	credentials := &fakePreparedCredentials{
		presence:     credential.PresenceAbsent,
		provisionErr: errors.New("native keyring failure"),
		result:       configuration.CredentialProvisionResult{CleanupRequired: true},
	}
	creator := configuration.NewProfileCreator(store, credentials)
	_, err := creator.Create(context.Background(), createRequest("request-cleanup", 1, "digest-cleanup"))
	if !errors.Is(err, configuration.ErrCredentialUnavailable) {
		t.Fatalf("Create() error = %v, want ErrCredentialUnavailable", err)
	}
	if credentials.deleteCalls != 0 {
		t.Fatalf("unsafe credential deletes = %d, want 0", credentials.deleteCalls)
	}
	if _, err := store.Read("wizard"); !errors.Is(err, profile.ErrProfileNotFound) {
		t.Fatalf("profile committed after provision failure: %v", err)
	}
}

func TestCreateProfileJoinsMatchingPendingRequestAndReplaysExactSavedProfile(t *testing.T) {
	store := profile.Store{Root: t.TempDir()}
	credentials := &fakePreparedCredentials{presence: credential.PresenceUnavailable}
	creator := configuration.NewProfileCreator(store, credentials)
	request := createRequest("request-join", 7, "digest-join")
	request.Profile.CredentialMode = profile.CredentialModePrompt
	first, err := creator.Create(context.Background(), request)
	if err != nil {
		t.Fatalf("first Create() error = %v", err)
	}
	second, err := creator.Create(context.Background(), request)
	if err != nil {
		t.Fatalf("second Create() error = %v", err)
	}
	if first.Profile != request.Profile || second.Profile != request.Profile {
		t.Fatalf("saved profiles = %#v / %#v, want exact %#v", first.Profile, second.Profile, request.Profile)
	}
	if first != second {
		t.Fatalf("joined result = %#v, want replay %#v", second, first)
	}
	if credentials.provisionCalls != 0 {
		t.Fatalf("provision calls = %d, want 0", credentials.provisionCalls)
	}
	replayed, err := creator.Create(context.Background(), request)
	if err != nil {
		t.Fatalf("replay Create() error = %v", err)
	}
	if replayed != first {
		t.Fatalf("replayed result = %#v, want %#v", replayed, first)
	}
	if credentials.provisionCalls != 0 {
		t.Fatalf("replay provision calls = %d, want 0", credentials.provisionCalls)
	}
}

func TestCreateProfileRejectsIdentityMismatchAndAllowsNewIdentityRetry(t *testing.T) {
	store := profile.Store{Root: t.TempDir()}
	credentials := &fakePreparedCredentials{presence: credential.PresenceAbsent, provisionErr: errors.New("temporarily unavailable")}
	creator := configuration.NewProfileCreator(store, credentials)
	request := createRequest("request-retry", 3, "digest-first")
	request.Profile.CredentialMode = profile.CredentialModePrompt
	if _, err := creator.Create(context.Background(), request); err != nil {
		t.Fatalf("first Create() error = %v", err)
	}
	mismatch := request
	mismatch.DraftDigest = "digest-second"
	if _, err := creator.Create(context.Background(), mismatch); !errors.Is(err, configuration.ErrCreateIdentityMismatch) {
		t.Fatalf("mismatched Create() error = %v, want ErrCreateIdentityMismatch", err)
	}
	retry := createRequest("request-retry-next", 4, "digest-second")
	retry.Profile.CredentialMode, retry.Profile.Name = profile.CredentialModePrompt, "wizard-next"
	result, err := creator.Create(context.Background(), retry)
	if err != nil {
		t.Fatalf("new-identity retry error = %v", err)
	}
	if result.Profile != retry.Profile || result.RequestID != retry.RequestID || result.Generation != retry.Generation || result.DraftDigest != retry.DraftDigest {
		t.Fatalf("retry result = %#v, want identity and exact profile", result)
	}
}

func createRequest(requestID string, generation uint64, digest string) configuration.CreateProfileRequest {
	return configuration.CreateProfileRequest{
		RequestID:   requestID,
		Generation:  generation,
		DraftDigest: digest,
		Profile: profile.Profile{
			SchemaVersion: profile.SchemaVersionV3,
			Name:          "wizard", Host: "ibmi.example.test", Port: 22, Username: "NEXUS$USER",
			HostKeyFingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", HostKeyTrust: profile.HostKeyTrustVerified,
			CredentialMode: profile.CredentialModeKeyring,
		},
	}
}

type fakePreparedCredentials struct {
	mu             sync.Mutex
	presence       credential.Presence
	provisionErr   error
	result         configuration.CredentialProvisionResult
	provisionCalls int
	deleteCalls    int
	started        chan struct{}
	release        chan struct{}
}

func (f *fakePreparedCredentials) Status(string) (credential.Presence, error) { return f.presence, nil }

func (f *fakePreparedCredentials) Provision(context.Context, string) (configuration.CredentialProvisionResult, error) {
	f.mu.Lock()
	f.provisionCalls++
	started, release, result, err := f.started, f.release, f.result, f.provisionErr
	f.mu.Unlock()
	if started != nil {
		close(started)
	}
	if release != nil {
		<-release
	}
	return result, err
}
