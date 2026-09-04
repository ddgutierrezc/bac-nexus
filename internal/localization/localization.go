// Package localization provides the immutable UI translation boundary.
package localization

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed catalogs/*.json
var catalogs embed.FS

// Localizer translates semantic UI message IDs; secret-bearing values are never inputs.
type Localizer interface {
	Text(id string, data map[string]any) string
	Locale() language.Tag
}

type localizer struct {
	localizer *i18n.Localizer
	locale    language.Tag
}

// New creates a complete, immutable localizer for a supported locale.
func New(locale language.Tag) (Localizer, error) {
	if locale != language.Spanish && locale != language.English {
		return nil, fmt.Errorf("unsupported locale: %s", locale)
	}
	bundle := i18n.NewBundle(language.Spanish)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	if err := loadCatalogs(bundle, catalogs); err != nil {
		return nil, err
	}
	if err := validateCatalogFiles(catalogs); err != nil {
		return nil, err
	}
	return localizer{localizer: i18n.NewLocalizer(bundle, locale.String()), locale: locale}, nil
}

// Spanish is the production-default UI locale.
func Spanish() Localizer { return mustNew(language.Spanish) }

// English provides the explicit composition seam for tests and future settings.
func English() Localizer { return mustNew(language.English) }

func mustNew(locale language.Tag) Localizer {
	l, err := New(locale)
	if err != nil {
		panic(err)
	}
	return l
}

func (l localizer) Text(id string, data map[string]any) string {
	config := &i18n.LocalizeConfig{MessageID: id, TemplateData: data}
	if id == "localization.profile_count" {
		config.PluralCount = pluralCount(data)
	}
	text, err := l.localizer.Localize(config)
	if err != nil {
		panic(fmt.Sprintf("localize %q: %v", id, err))
	}
	if id == "wizard.step.profile" || id == "wizard.step.connection" || id == "wizard.step.identity" {
		text = strings.ReplaceAll(strings.ReplaceAll(text, "de 9", "de 8"), "of 9", "of 8")
	}
	return text
}

func pluralCount(data map[string]any) any {
	if data == nil {
		return nil
	}
	return data["Count"]
}

func (l localizer) Locale() language.Tag { return l.locale }

func loadCatalogs(bundle *i18n.Bundle, files fs.FS) error {
	entries, err := fs.ReadDir(files, "catalogs")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if _, err := bundle.LoadMessageFileFS(files, "catalogs/"+entry.Name()); err != nil {
			return err
		}
	}
	return nil
}

type catalogMessage struct{ ID, Translation, One, Other string }

func validateCatalogFiles(files fs.FS) error {
	want := make(map[string]struct{}, len(catalogIDs()))
	for _, id := range catalogIDs() {
		want[id] = struct{}{}
	}
	for _, locale := range []string{"es", "en"} {
		got := map[string]catalogMessage{}
		entries, err := fs.ReadDir(files, "catalogs")
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() || len(entry.Name()) < len(locale) || entry.Name()[:len(locale)] != locale {
				continue
			}
			data, err := fs.ReadFile(files, "catalogs/"+entry.Name())
			if err != nil {
				return err
			}
			var messages []catalogMessage
			if err := json.Unmarshal(data, &messages); err != nil {
				return err
			}
			for _, message := range messages {
				if _, exists := got[message.ID]; exists {
					return fmt.Errorf("duplicate %s catalog ID %q", locale, message.ID)
				}
				got[message.ID] = message
			}
		}
		if len(got) != len(want) {
			return fmt.Errorf("incomplete %s catalog: got %d IDs, want %d", locale, len(got), len(want))
		}
		for id := range want {
			message, ok := got[id]
			if !ok {
				return fmt.Errorf("incomplete %s catalog for %q", locale, id)
			}
			if err := validateCatalogMessage(id, message); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateCatalogMessage(id string, message catalogMessage) error {
	if id == "localization.profile_count" {
		if message.One == "" || message.Other == "" {
			return fmt.Errorf("incomplete plural forms for %s", id)
		}
		return nil
	}
	if message.Translation == "" {
		return fmt.Errorf("missing translation for %s", id)
	}
	return nil
}

// MessageIDs returns the expected complete catalog surface for tests.
func MessageIDs() []string {
	ids := catalogIDs()
	sort.Strings(ids)
	return ids
}

func catalogIDs() []string {
	return []string{
		"wizard.identity.key_type",
		"action.back", "action.cancel", "action.continue", "common.confirmation", "common.error", "common.status", "home.help", "wizard.connection.port", "wizard.identity.footer", "localization.profile_count",
		"credential.absent", "credential.present", "credential.unavailable", "error.port_number", "domain.trust.tofu", "domain.trust.verified", "domain.credential.vault", "domain.credential.prompt", "wizard.profile.new", "wizard.profile.example", "overflow.above", "overflow.below",
		"form.fields", "form.footer", "form.action.save", "form.action.cancel", "form.label.credential_mode", "form.label.fingerprint", "form.label.host", "form.label.java_home", "form.label.mapepire_jar", "form.label.name", "form.label.port", "form.label.trust", "form.label.username",
		"profile.validation.name", "profile.validation.endpoint", "profile.validation.username", "profile.validation.host_key", "profile.validation.java_home", "profile.validation.mapepire_jar", "profile.validation.credential_mode", "profile.validation.profile",
		"home.footer", "home.header.profile.empty", "home.header.profile.none", "home.header.profile.unassessed", "home.header.status.configuration_required", "home.header.status.pending_verification", "home.header.status.unassessed", "home.menu.configuration", "home.menu.create", "home.menu.diagnostics", "home.menu.exit", "home.menu.integrations", "home.menu.manage", "home.menu.readiness", "home.profile_heading", "home.readiness.local", "home.readiness.none", "home.readiness.summary", "home.readiness.unassessed", "home.readiness.available", "home.tagline", "home.unavailable", "legacy.confirm.cancel", "legacy.confirm.delete", "legacy.confirm.retains_backup", "legacy.confirm.type_delete", "legacy.detail.footer", "legacy.detail.host", "legacy.detail.profile", "legacy.detail.trust", "legacy.detail.username", "legacy.list.empty", "legacy.list.footer", "legacy.list.heading", "legacy.title",
		"operation.cancelled", "operation.credential_deleted", "operation.credential_rotated", "operation.credential_stored", "operation.host_key_enrolled", "operation.profile_created", "operation.profile_deleted", "operation.profile_saved", "operation.profile_updated", "operation.unavailable", "operation.load_failed", "operation.refresh_failed",
		"security.confirm_credential", "security.confirm_migration", "security.confirm_tofu", "security.error_prefix", "security.footer_back", "security.header", "security.manual", "security.menu", "security.progress", "security.status", "security.tofu_warning", "security.trust.confirmation", "security.trust.fingerprint", "security.trust.provenance",
		"wizard.connection.default_port", "wizard.connection.description", "wizard.connection.help", "wizard.connection.host", "wizard.connection.section", "wizard.connection.title", "wizard.connection.username", "wizard.footer.connection", "wizard.footer.identity", "wizard.footer.profile", "wizard.header.configuring", "wizard.header.profile", "wizard.identity.choice_known.description", "wizard.identity.choice_known.label", "wizard.identity.choice_observed.description", "wizard.identity.choice_observed.label", "wizard.identity.choice_observed.note", "wizard.identity.description", "wizard.identity.help", "wizard.identity.question", "wizard.identity.section", "wizard.identity.title", "wizard.identity.warning", "wizard.identity.footer_authorize", "wizard.identity.footer_review", "wizard.identity.authorize", "wizard.identity.notice_1", "wizard.identity.notice_2", "wizard.identity.inspect", "wizard.identity.retry", "wizard.identity.loading", "wizard.identity.error", "wizard.identity.observed_title", "wizard.identity.warning_observed", "wizard.identity.observed_description_1", "wizard.identity.observed_description_2", "wizard.identity.trust", "wizard.identity.trusted", "wizard.identity.completed", "wizard.profile.available", "wizard.profile.duplicate", "wizard.profile.guidance_1", "wizard.profile.guidance_2", "wizard.profile.guidance_3", "wizard.profile.help", "wizard.profile.invalid", "wizard.profile.label", "wizard.profile.loading", "wizard.profile.name_label", "wizard.profile.required", "wizard.profile.title", "wizard.step.connection", "wizard.step.identity", "wizard.step.profile", "wizard.validation.host_invalid", "wizard.validation.host_required", "wizard.validation.port_invalid", "wizard.validation.port_number", "wizard.validation.username_invalid", "wizard.validation.username_required",
		"wizard.footer.mapepire",
		"onboarding.title", "onboarding.description", "onboarding.host", "onboarding.username", "onboarding.connect", "onboarding.footer", "onboarding.validation", "onboarding.validation.host", "onboarding.validation.username", "onboarding.unavailable", "onboarding.prompt_failed", "onboarding.running", "onboarding.cancel", "onboarding.failed", "onboarding.failed_cleanup", "onboarding.saved", "onboarding.finalize", "onboarding.name", "onboarding.port", "onboarding.next", "onboarding.capture", "onboarding.password_prompt", "onboarding.capture_failed", "onboarding.capture_required", "onboarding.validation.name_loading", "onboarding.validation.name_duplicate", "onboarding.validation.port", "onboarding.name.description", "onboarding.connection.description", "onboarding.credentials.description", "onboarding.review.description", "onboarding.step.1", "onboarding.step.2", "onboarding.step.3", "onboarding.step.4", "onboarding.completion_saved", "onboarding.completion_failed", "onboarding.diagnostic.details", "onboarding.diagnostic.reference", "onboarding.diagnostic.written_guidance", "onboarding.diagnostic.unavailable_guidance", "onboarding.diagnostic.phase.host_key_inspection", "onboarding.diagnostic.phase.existing_identity", "onboarding.diagnostic.phase.authenticated_proof", "onboarding.diagnostic.phase.bootstrap_audit", "onboarding.diagnostic.phase.keyring_precondition", "onboarding.diagnostic.phase.commit", "onboarding.diagnostic.phase.save", "onboarding.diagnostic.phase.unavailable",
	}
}
