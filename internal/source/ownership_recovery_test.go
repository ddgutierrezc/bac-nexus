package source

import (
	"bytes"
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
