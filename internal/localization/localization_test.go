package localization

import (
	"strings"
	"testing"
	"testing/fstest"

	"golang.org/x/text/language"
)

func TestLocalesAreCompleteAndDeterministic(t *testing.T) {
	for _, tt := range []struct {
		name string
		make func() Localizer
		want string
	}{{"spanish default", Spanish, "Crear un perfil"}, {"explicit english", English, "Create a profile"}} {
		t.Run(tt.name, func(t *testing.T) {
			l := tt.make()
			if got := l.Text("home.menu.create", nil); got != tt.want {
				t.Fatalf("translation = %q, want %q", got, tt.want)
			}
			if got := l.Text("home.readiness.available", map[string]any{"Count": 2}); got == "" {
				t.Fatal("interpolation was empty")
			}
			if one, many := l.Text("localization.profile_count", map[string]any{"Count": 1}), l.Text("localization.profile_count", map[string]any{"Count": 2}); one == many {
				t.Fatal("plural forms did not differ")
			}
			for _, id := range MessageIDs() {
				if got := l.Text(id, map[string]any{"Count": 2}); got == "" {
					t.Fatalf("%s is empty", id)
				}
			}
		})
	}
}

func TestFourStepOnboardingCatalogsRemainPairedWithoutLanguageLeakage(t *testing.T) {
	for _, tt := range []struct {
		name, required, forbidden string
		localizer                 Localizer
	}{
		{"spanish", "Paso", "Step", Spanish()},
		{"english", "Step", "Paso", English()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for step := 1; step <= 4; step++ {
				id := "onboarding.step." + string(rune('0'+step))
				text := tt.localizer.Text(id, nil)
				if !strings.Contains(text, tt.required) || strings.Contains(text, tt.forbidden) || !strings.Contains(text, "4") {
					t.Fatalf("%s = %q, want localized four-step label", id, text)
				}
			}
			for _, id := range []string{"onboarding.name.description", "onboarding.connection.description", "onboarding.credentials.description", "onboarding.review.description"} {
				if text := tt.localizer.Text(id, nil); text == "" || strings.Contains(text, tt.forbidden) {
					t.Fatalf("%s = %q, want paired %s copy without language leakage", id, text, tt.name)
				}
			}
		})
	}
}

func TestCatalogValidationRejectsIncompleteEnglishWithoutFallback(t *testing.T) {
	files := fstest.MapFS{
		"catalogs/es.json": {Data: []byte(`[{"id":"home.help","translation":"Ayuda"}]`)},
		"catalogs/en.json": {Data: []byte(`[{"id":"home.help","translation":"Help"}]`)},
	}
	if err := validateCatalogFiles(files); err == nil {
		t.Fatal("incomplete English catalog passed direct validation")
	}
}

func TestCatalogValidationRejectsEachMissingPluralForm(t *testing.T) {
	for _, message := range []catalogMessage{{ID: "localization.profile_count", One: "one"}, {ID: "localization.profile_count", Other: "other"}} {
		if err := validateCatalogMessage("localization.profile_count", message); err == nil {
			t.Fatal("missing plural form passed validation")
		}
	}
}

func TestUnsupportedLocaleAndMissingIDFailClosed(t *testing.T) {
	if _, err := New(language.French); err == nil {
		t.Fatal("unsupported locale succeeded")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("missing message ID did not fail closed")
		}
	}()
	Spanish().Text("missing.id", nil)
}
