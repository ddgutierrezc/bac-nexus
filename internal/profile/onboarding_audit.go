package profile

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

var ErrOnboardingAuditRejected = errors.New("onboarding audit rejected")

// OnboardingAuditStore durably records the two automatic-TOFU lifecycle
// events. Its fixed schema intentionally has no endpoint, username, secret,
// or remote error field.
type OnboardingAuditStore struct{ Profiles Store }

type onboardingAuditRecord struct {
	Code      string `json:"code"`
	Profile   string `json:"profile"`
	Timestamp int64  `json:"timestamp"`
}

func (s OnboardingAuditStore) Record(ctx context.Context, name, code string) error {
	if ctx == nil || ctx.Err() != nil || ValidateName(name) != nil || !validOnboardingAuditCode(code) {
		return ErrOnboardingAuditRejected
	}
	if s.Profiles.Root == "" {
		return ErrOnboardingAuditRejected
	}
	if err := os.MkdirAll(s.Profiles.Root, 0o700); err != nil || s.Profiles.verifyRoot() != nil {
		return ErrOnboardingAuditRejected
	}
	data, err := json.Marshal(onboardingAuditRecord{Code: code, Profile: name, Timestamp: time.Now().UnixMilli()})
	if err != nil {
		return ErrOnboardingAuditRejected
	}
	file, err := os.OpenFile(filepath.Join(s.Profiles.Root, ".onboarding-audit.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return ErrOnboardingAuditRejected
	}
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil || file.Sync() != nil {
		return ErrOnboardingAuditRejected
	}
	return nil
}

func validOnboardingAuditCode(code string) bool {
	return code == "identity_bootstrap_allowed" || code == "identity_pin_committed"
}
