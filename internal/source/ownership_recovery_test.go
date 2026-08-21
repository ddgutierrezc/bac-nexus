package source

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

func TestGuardRecoveryRecordAcceptsExactCanonicalRecord(t *testing.T) {
	record := OwnershipRecord{
		Token:        bytes.Repeat([]byte{0x17}, 16),
		RemotePath:   "/home/nexus/.bac-nexus/tmp/recovery-017.utf8",
		Profile:      "approved",
		TargetDigest: bytes.Repeat([]byte{0x42}, 32),
		CreatedAt:    time.Date(2026, 8, 20, 0, 0, 17, 0, time.UTC),
	}

	if err := guardRecoveryRecord(record); err != nil {
		t.Fatalf("guardRecoveryRecord() error = %v", err)
	}
}

func TestGuardRecoveryRecordRejectsMalformedOrUnavailableRecord(t *testing.T) {
	valid := OwnershipRecord{
		Token:        bytes.Repeat([]byte{0x17}, 16),
		RemotePath:   "/home/nexus/.bac-nexus/tmp/recovery-017.utf8",
		Profile:      "approved",
		TargetDigest: bytes.Repeat([]byte{0x42}, 32),
		CreatedAt:    time.Date(2026, 8, 20, 0, 0, 17, 0, time.UTC),
	}

	tests := []struct {
		name   string
		mutate func(*OwnershipRecord)
	}{
		{
			name: "token is unavailable",
			mutate: func(record *OwnershipRecord) {
				record.Token = nil
			},
		},
		{
			name: "token has the wrong length",
			mutate: func(record *OwnershipRecord) {
				record.Token = record.Token[:15]
			},
		},
		{
			name: "path is relative",
			mutate: func(record *OwnershipRecord) {
				record.RemotePath = ".bac-nexus/tmp/recovery-017.utf8"
			},
		},
		{
			name: "path traverses the namespace",
			mutate: func(record *OwnershipRecord) {
				record.RemotePath = "/home/nexus/.bac-nexus/tmp/../recovery-017.utf8"
			},
		},
		{
			name: "path is historical shared temporary",
			mutate: func(record *OwnershipRecord) {
				record.RemotePath = "/tmp/bac-nexus-catalog-recovery-017.utf8"
			},
		},
		{
			name: "profile is malformed",
			mutate: func(record *OwnershipRecord) {
				record.Profile = "not/approved"
			},
		},
		{
			name: "target digest is unavailable",
			mutate: func(record *OwnershipRecord) {
				record.TargetDigest = nil
			},
		},
		{
			name: "creation time is unavailable",
			mutate: func(record *OwnershipRecord) {
				record.CreatedAt = time.Time{}
			},
		},
		{
			name: "creation time has subsecond precision",
			mutate: func(record *OwnershipRecord) {
				record.CreatedAt = record.CreatedAt.Add(time.Nanosecond)
			},
		},
		{
			name: "creation time is not canonical UTC",
			mutate: func(record *OwnershipRecord) {
				record.CreatedAt = time.Date(2026, 8, 20, 0, 0, 17, 0, time.FixedZone("UTC", 0))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := valid
			tt.mutate(&record)

			if err := guardRecoveryRecord(record); !errors.Is(err, ErrOwnershipInvalid) {
				t.Fatalf("guardRecoveryRecord() error = %v, want ErrOwnershipInvalid", err)
			}
		})
	}
}
