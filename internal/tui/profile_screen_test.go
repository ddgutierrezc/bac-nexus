package tui

import (
	"strings"
	"testing"
)

func TestProfileScreenPrimitivesRenderFieldsActionsAndSemanticFeedback(t *testing.T) {
	m := NewModel(nil)
	theme := newHomeTheme(true)
	body := strings.Join([]string{
		m.profileField("▸", "Host", "ibmi.example.test"),
		m.profileAction("▸", "CONNECT AND SAVE"),
		m.profileFeedback("[ERR]", "Host is invalid", theme),
	}, "\n")
	for _, want := range []string{"▸ Host: ibmi.example.test", "▸ [ CONNECT AND SAVE ]", "[ERR] Host is invalid"} {
		if !strings.Contains(body, want) {
			t.Fatalf("primitive output missing %q: %q", want, body)
		}
	}
}
