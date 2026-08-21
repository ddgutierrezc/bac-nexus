package source

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"reflect"
	"testing"

	"bac-nexus/internal/profile"
)

func TestRecoveryCoordinatorRemovesConfirmedExactRowsAndDeletesThem(t *testing.T) {
	fresh := recoveryProfile()
	first := recoveryRecordForProfile(fresh)
	second := recoveryRecordForProfile(fresh)
	second.Token = bytes.Repeat([]byte{0x18}, 16)
	second.RemotePath = "/home/nexus/.bac-nexus/tmp/recovery-018.utf8"
	ledger := &recoveryLedgerFake{records: []OwnershipRecord{first, second}}
	remote := &recoveryRemoteFake{removeErrors: []error{nil, ErrRemoteNotFound}, statErrors: []error{ErrRemoteNotFound, ErrRemoteNotFound}}
	coordinator := recoveryCoordinatorFor(fresh, ledger, remote)

	if err := coordinator.Recover(context.Background()); err != nil {
		t.Fatalf("recover exact rows: %v", err)
	}
	if ledger.listCalls != 1 {
		t.Fatalf("list calls = %d, want 1 bounded list", ledger.listCalls)
	}
	if !reflect.DeepEqual(ledger.deleted, []OwnershipRecord{first, second}) {
		t.Fatalf("deleted records = %#v, want exact listed rows %#v", ledger.deleted, []OwnershipRecord{first, second})
	}
	if want := []string{"remove:" + first.RemotePath, "stat:" + first.RemotePath, "close", "remove:" + second.RemotePath, "stat:" + second.RemotePath, "close"}; !reflect.DeepEqual(remote.events, want) {
		t.Fatalf("remote events = %v, want %v", remote.events, want)
	}
	if err := coordinator.Recover(context.Background()); err != nil {
		t.Fatalf("repeat recovery: %v", err)
	}
	if ledger.listCalls != 2 || len(ledger.deleted) != 2 || len(remote.events) != 6 {
		t.Fatalf("repeat recovery reused rows: lists=%d deleted=%d remote=%v", ledger.listCalls, len(ledger.deleted), remote.events)
	}
}

func TestRecoveryCoordinatorRetainsRowsOnUncertainOrInvalidRecovery(t *testing.T) {
	fresh := recoveryProfile()
	record := recoveryRecordForProfile(fresh)
	tests := []struct {
		name       string
		configure  func(*recoveryCoordinator, *recoveryLedgerFake, *recoveryRemoteFake)
		wantEvents []string
	}{
		{
			name: "list failure makes no remote call",
			configure: func(_ *recoveryCoordinator, ledger *recoveryLedgerFake, _ *recoveryRemoteFake) {
				ledger.listErr = errRecoveryGuard
			},
		},
		{
			name: "corrupt row makes no remote call",
			configure: func(_ *recoveryCoordinator, ledger *recoveryLedgerFake, _ *recoveryRemoteFake) {
				bad := record
				bad.RemotePath = "/tmp/bac-nexus-catalog-historical.utf8"
				ledger.records = []OwnershipRecord{bad}
			},
		},
		{
			name: "retargeted profile makes no remote call",
			configure: func(coordinator *recoveryCoordinator, _ *recoveryLedgerFake, _ *recoveryRemoteFake) {
				coordinator.resolveProfile = func(context.Context, string) (profile.Profile, error) {
					retargeted := fresh
					retargeted.Host = "other.example.test"
					return retargeted, nil
				}
			},
		},
		{
			name: "remove uncertainty retains row",
			configure: func(_ *recoveryCoordinator, _ *recoveryLedgerFake, remote *recoveryRemoteFake) {
				remote.removeErrors = []error{errRecoveryGuard}
				remote.statErrors = []error{ErrRemoteNotFound}
			},
			wantEvents: []string{"remove:" + record.RemotePath, "stat:" + record.RemotePath, "close"},
		},
		{
			name: "present path retains row",
			configure: func(_ *recoveryCoordinator, _ *recoveryLedgerFake, remote *recoveryRemoteFake) {
				remote.statErrors = []error{nil}
			},
			wantEvents: []string{"remove:" + record.RemotePath, "stat:" + record.RemotePath, "close"},
		},
		{
			name: "ledger delete failure retains row after confirmed absence",
			configure: func(_ *recoveryCoordinator, ledger *recoveryLedgerFake, remote *recoveryRemoteFake) {
				ledger.deleteErr = errRecoveryGuard
				remote.statErrors = []error{ErrRemoteNotFound}
			},
			wantEvents: []string{"remove:" + record.RemotePath, "stat:" + record.RemotePath, "close"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ledger := &recoveryLedgerFake{records: []OwnershipRecord{record}}
			remote := &recoveryRemoteFake{statErrors: []error{ErrRemoteNotFound}}
			coordinator := recoveryCoordinatorFor(fresh, ledger, remote)
			tt.configure(&coordinator, ledger, remote)

			if err := coordinator.Recover(context.Background()); err == nil {
				t.Fatal("recovery unexpectedly deleted an unsafe row")
			}
			if len(ledger.deleted) != 0 || len(ledger.records) != 1 {
				t.Fatalf("unsafe recovery deleted or lost its row: deleted=%v records=%v", ledger.deleted, ledger.records)
			}
			if !reflect.DeepEqual(remote.events, tt.wantEvents) {
				t.Fatalf("remote events = %v, want %v", remote.events, tt.wantEvents)
			}
		})
	}
}

type recoveryLedgerFake struct {
	records   []OwnershipRecord
	listErr   error
	deleteErr error
	listCalls int
	deleted   []OwnershipRecord
}

func (l *recoveryLedgerFake) ListRecovery(context.Context) ([]OwnershipRecord, error) {
	l.listCalls++
	if l.listErr != nil {
		return nil, l.listErr
	}
	return append([]OwnershipRecord(nil), l.records...), nil
}

func (l *recoveryLedgerFake) Delete(_ context.Context, record OwnershipRecord) error {
	if l.deleteErr != nil {
		return l.deleteErr
	}
	for index, candidate := range l.records {
		if reflect.DeepEqual(candidate, record) {
			l.records = append(l.records[:index], l.records[index+1:]...)
			l.deleted = append(l.deleted, record)
			return nil
		}
	}
	return ErrOwnershipInvalid
}

type recoveryRemoteFake struct {
	removeErrors []error
	statErrors   []error
	events       []string
}

func (r *recoveryRemoteFake) Remove(_ context.Context, path string) error {
	r.events = append(r.events, "remove:"+path)
	return r.next(&r.removeErrors)
}

func (r *recoveryRemoteFake) Stat(_ context.Context, path string) (fs.FileInfo, error) {
	r.events = append(r.events, "stat:"+path)
	return nil, r.next(&r.statErrors)
}

func (r *recoveryRemoteFake) Close() error {
	r.events = append(r.events, "close")
	return nil
}

func (r *recoveryRemoteFake) next(errors *[]error) error {
	if len(*errors) == 0 {
		return nil
	}
	err := (*errors)[0]
	*errors = (*errors)[1:]
	return err
}

func recoveryCoordinatorFor(fresh profile.Profile, ledger RecoveryLedger, remote RecoveryRemote) recoveryCoordinator {
	return recoveryCoordinator{
		ledger: ledger,
		resolveProfile: func(context.Context, string) (profile.Profile, error) {
			return fresh, nil
		},
		getCredential: func(context.Context, string) ([]byte, error) {
			return []byte("recovery-secret"), nil
		},
		openCleanup: func(context.Context, profile.Profile, []byte) (RecoveryRemote, error) {
			return remote, nil
		},
	}
}

var _ RecoveryLedger = (*recoveryLedgerFake)(nil)
var _ RecoveryRemote = (*recoveryRemoteFake)(nil)

var _ = errors.Is
