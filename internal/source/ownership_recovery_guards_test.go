package source

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io/fs"
	"reflect"
	"testing"
	"time"

	"bac-nexus/internal/profile"
)

var errRecoveryGuard = errors.New("recovery guard failed")

var validRecoveryTargetDigest = mustRecoveryDigest("56fd2e82c3d814386b40fb61ba3d9001c330f9cf61562ae1b47810842647b78d")

func TestRecoverOwnershipRecordRunsFreshGuardsBeforeExactPathCallback(t *testing.T) {
	fresh := recoveryProfile()
	secret := []byte("recovery-secret")
	record := recoveryRecordForProfile(fresh)
	var events []string
	var receivedSecret []byte

	err := recoverOwnershipRecord(context.Background(), record, recoveryGuards{
		resolveProfile: func(_ context.Context, name string) (profile.Profile, error) {
			events = append(events, "profile")
			if name != record.Profile {
				t.Fatalf("profile name = %q, want %q", name, record.Profile)
			}
			return fresh, nil
		},
		getCredential: func(_ context.Context, name string) ([]byte, error) {
			events = append(events, "credential")
			if name != record.Profile {
				t.Fatalf("credential profile = %q, want %q", name, record.Profile)
			}
			return secret, nil
		},
		openCleanup: func(_ context.Context, got profile.Profile, password []byte) (RecoveryRemote, error) {
			events = append(events, "open")
			if got != fresh {
				t.Fatalf("fresh profile = %#v, want %#v", got, fresh)
			}
			receivedSecret = password
			return recoveryCloser{onClose: func() { events = append(events, "close") }}, nil
		},
		cleanupReady: func(_ context.Context, _ RecoveryRemote, path string) error {
			events = append(events, "ready")
			if path != record.RemotePath {
				t.Fatalf("cleanup-ready path = %q, want exact recorded path %q", path, record.RemotePath)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("recover ownership record: %v", err)
	}
	if want := []string{"profile", "credential", "open", "ready", "close"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("guard events = %v, want %v", events, want)
	}
	if !bytes.Equal(receivedSecret, make([]byte, len(secret))) {
		t.Fatalf("temporary credential was not cleared: %x", receivedSecret)
	}
}

func TestRecoverOwnershipRecordRetainsOwnershipBeforeCleanupReady(t *testing.T) {
	valid := recoveryProfile()
	validRecord := recoveryRecordForProfile(valid)
	tests := []struct {
		name   string
		ctx    context.Context
		record OwnershipRecord
		guards func(*[]string) recoveryGuards
		want   []string
	}{
		{
			name:   "cancelled context",
			ctx:    cancelledRecoveryContext(),
			record: validRecord,
			guards: recoveryWorkingGuards(valid),
			want:   nil,
		},
		{
			name:   "profile resolution fails",
			ctx:    context.Background(),
			record: validRecord,
			guards: func(events *[]string) recoveryGuards {
				guards := recoveryWorkingGuards(valid)(events)
				guards.resolveProfile = func(context.Context, string) (profile.Profile, error) {
					*events = append(*events, "profile")
					return profile.Profile{}, errRecoveryGuard
				}
				return guards
			},
			want: []string{"profile"},
		},
		{
			name:   "credential retrieval fails",
			ctx:    context.Background(),
			record: validRecord,
			guards: func(events *[]string) recoveryGuards {
				guards := recoveryWorkingGuards(valid)(events)
				guards.getCredential = func(context.Context, string) ([]byte, error) {
					*events = append(*events, "credential")
					return nil, errRecoveryGuard
				}
				return guards
			},
			want: []string{"profile", "credential"},
		},
		{
			name: "target binding differs",
			ctx:  context.Background(),
			record: func() OwnershipRecord {
				record := validRecord
				record.TargetDigest = bytes.Repeat([]byte{0xff}, 32)
				return record
			}(),
			guards: recoveryWorkingGuards(valid),
			want:   []string{"profile", "credential"},
		},
		{
			name:   "fresh profile pin is invalid",
			ctx:    context.Background(),
			record: validRecord,
			guards: func(events *[]string) recoveryGuards {
				invalid := valid
				invalid.HostKeyFingerprint = ""
				return recoveryWorkingGuards(invalid)(events)
			},
			want: []string{"profile", "credential"},
		},
		{
			name:   "constrained cleanup opener fails",
			ctx:    context.Background(),
			record: validRecord,
			guards: func(events *[]string) recoveryGuards {
				guards := recoveryWorkingGuards(valid)(events)
				guards.openCleanup = func(context.Context, profile.Profile, []byte) (RecoveryRemote, error) {
					*events = append(*events, "open")
					return nil, errRecoveryGuard
				}
				return guards
			},
			want: []string{"profile", "credential", "open"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var events []string
			err := recoverOwnershipRecord(tt.ctx, tt.record, tt.guards(&events))
			if err == nil {
				t.Fatal("recovery guard failure unexpectedly continued")
			}
			if !reflect.DeepEqual(events, tt.want) {
				t.Fatalf("guard events = %v, want %v; cleanup-ready must not run", events, tt.want)
			}
		})
	}
}

func recoveryWorkingGuards(fresh profile.Profile) func(*[]string) recoveryGuards {
	return func(events *[]string) recoveryGuards {
		return recoveryGuards{
			resolveProfile: func(context.Context, string) (profile.Profile, error) {
				*events = append(*events, "profile")
				return fresh, nil
			},
			getCredential: func(context.Context, string) ([]byte, error) {
				*events = append(*events, "credential")
				return []byte("recovery-secret"), nil
			},
			openCleanup: func(context.Context, profile.Profile, []byte) (RecoveryRemote, error) {
				*events = append(*events, "open")
				return recoveryCloser{}, nil
			},
			cleanupReady: func(context.Context, RecoveryRemote, string) error {
				*events = append(*events, "ready")
				return nil
			},
		}
	}
}

func recoveryRecordForProfile(fresh profile.Profile) OwnershipRecord {
	return OwnershipRecord{
		Token:        bytes.Repeat([]byte{0x17}, 16),
		RemotePath:   "/home/nexus/.bac-nexus/tmp/recovery-017.utf8",
		Profile:      fresh.Name,
		TargetDigest: append([]byte(nil), validRecoveryTargetDigest...),
		CreatedAt:    time.Date(2026, 8, 20, 0, 0, 17, 0, time.UTC),
	}
}

func recoveryProfile() profile.Profile {
	return profile.Profile{
		Name:               "approved",
		Host:               "ibmi.example.test",
		Port:               22,
		Username:           "NEXUS$USER",
		HostKeyFingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		HostKeyTrust:       profile.HostKeyTrustVerified,
		CredentialMode:     profile.CredentialModeVault,
	}
}

func cancelledRecoveryContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func mustRecoveryDigest(encoded string) []byte {
	digest, err := hex.DecodeString(encoded)
	if err != nil {
		panic(err)
	}
	return digest
}

type recoveryCloser struct{ onClose func() }

func (c recoveryCloser) Close() error {
	if c.onClose != nil {
		c.onClose()
	}
	return nil
}

func (recoveryCloser) Remove(context.Context, string) error { return ErrRemoteNotFound }

func (recoveryCloser) Stat(context.Context, string) (fs.FileInfo, error) {
	return nil, ErrRemoteNotFound
}

var _ RecoveryRemote = recoveryCloser{}
