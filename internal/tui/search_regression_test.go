package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/KabosuNeko/Futon/internal/api"
)

func TestTypingUpdatesInput(t *testing.T) {
	m := testSearchModel()

	for _, r := range "naruto" {
		newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = newM.(SearchModel)
	}

	if m.input.Value() != "naruto" {
		t.Errorf("expected input 'naruto', got %q", m.input.Value())
	}
}

func TestTabCyclesProviders(t *testing.T) {
	providers := []api.MangaProvider{api.NewOTruyenProvider(), api.NewMangaDexProvider()}
	m := NewSearchModel(providers)

	// Default is "All" mode, CurrentProvider returns nil
	if m.CurrentProvider() != nil {
		t.Fatalf("expected nil (All mode), got %v", m.CurrentProvider())
	}

	// Tab 1: All -> OTruyen
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")})
	rm := newM.(SearchModel)
	if rm.CurrentProvider().Name() != "OTruyen" {
		t.Errorf("expected provider OTruyen after tab, got %s", rm.CurrentProvider().Name())
	}

	// Tab 2: OTruyen -> MangaDex
	newM, _ = rm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")})
	rm = newM.(SearchModel)
	if rm.CurrentProvider().Name() != "MangaDex" {
		t.Errorf("expected provider MangaDex after second tab, got %s", rm.CurrentProvider().Name())
	}

	// Tab 3: MangaDex -> All
	newM, _ = rm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")})
	rm = newM.(SearchModel)
	if rm.CurrentProvider() != nil {
		t.Errorf("expected All mode after third tab, got %v", rm.CurrentProvider().Name())
	}
}
