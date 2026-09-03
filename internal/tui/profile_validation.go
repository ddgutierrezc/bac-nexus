package tui

import (
	"strconv"

	"bac-nexus/internal/profile"
)

const (
	profileValidationFieldName           = "name"
	profileValidationFieldEndpoint       = "endpoint"
	profileValidationFieldUsername       = "username"
	profileValidationFieldHostKey        = "host_key"
	profileValidationFieldJavaHome       = "java_home"
	profileValidationFieldMapepireJAR    = "mapepire_jar"
	profileValidationFieldCredentialMode = "credential_mode"
)

// profileValidation carries only semantic presentation data. Cause remains
// available for control flow but its prose is never rendered or classified.
type profileValidation struct {
	FieldID   string
	MessageID string
	Cause     error
}

// validateEditProfilePreflight preserves Profile.Validate's early field order
// while raw form values still need syntactic conversion.
func validateEditProfilePreflight(name, host, portText, username string) (int, *profileValidation) {
	if validation := validateEditProfileName(name); validation != nil {
		return 0, validation
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return 0, invalidProfileField(profileValidationFieldEndpoint, "profile.validation.endpoint", err)
	}
	if err := profile.ValidateEndpoint(host, port); err != nil {
		return 0, invalidProfileField(profileValidationFieldEndpoint, "profile.validation.endpoint", err)
	}
	if err := profile.ValidateUsername(username); err != nil {
		return 0, invalidProfileField(profileValidationFieldUsername, "profile.validation.username", err)
	}
	return port, nil
}

func validateEditProfile(original, candidate profile.Profile) *profileValidation {
	if validation := validateEditProfileName(candidate.Name); validation != nil {
		return validation
	}
	if err := profile.ValidateEndpoint(candidate.Host, candidate.Port); err != nil {
		return invalidProfileField(profileValidationFieldEndpoint, "profile.validation.endpoint", err)
	}
	if err := profile.ValidateUsername(candidate.Username); err != nil {
		return invalidProfileField(profileValidationFieldUsername, "profile.validation.username", err)
	}
	if candidate.SchemaVersion == 0 {
		if err := profile.ValidateHostKey(candidate.HostKeyFingerprint, candidate.HostKeyTrust); err != nil {
			return invalidProfileField(profileValidationFieldHostKey, "profile.validation.host_key", err)
		}
	}
	if validation := validateEditDraftGroup(original, profileValidationFieldJavaHome, "profile.validation.java_home", func(draft *profile.Profile) {
		draft.JavaHome = candidate.JavaHome
	}); validation != nil {
		return validation
	}
	if validation := validateEditDraftGroup(original, profileValidationFieldMapepireJAR, "profile.validation.mapepire_jar", func(draft *profile.Profile) {
		draft.MapepireJAR = candidate.MapepireJAR
	}); validation != nil {
		return validation
	}
	if validation := validateEditDraftGroup(original, profileValidationFieldCredentialMode, "profile.validation.credential_mode", func(draft *profile.Profile) {
		draft.CredentialMode = candidate.CredentialMode
	}); validation != nil {
		return validation
	}
	if err := candidate.Validate(); err != nil {
		return invalidProfileField(profileValidationFieldName, "profile.validation.profile", err)
	}
	return nil
}

func validateEditProfileName(name string) *profileValidation {
	if err := profile.ValidateName(name); err != nil {
		return invalidProfileField(profileValidationFieldName, "profile.validation.name", err)
	}
	return nil
}

func validateEditDraftGroup(original profile.Profile, fieldID, messageID string, apply func(*profile.Profile)) *profileValidation {
	draft := original
	apply(&draft)
	if err := draft.Validate(); err != nil {
		return invalidProfileField(fieldID, messageID, err)
	}
	return nil
}

func invalidProfileField(fieldID, messageID string, cause error) *profileValidation {
	if cause == nil {
		return nil
	}
	return &profileValidation{FieldID: fieldID, MessageID: messageID, Cause: cause}
}
