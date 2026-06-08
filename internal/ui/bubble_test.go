package ui

import (
	"testing"

	"bitbeat/internal/config"
	"bitbeat/internal/network"

	tea "github.com/charmbracelet/bubbletea"
)

func TestScrollingBehavior(t *testing.T) {
	// Create a dummy model
	m := Model{
		config: &config.Config{
			Keybindings: config.KeybindingConfig{
				Quit:      "q",
				PlayPause: " ",
			},
		},
		state: stateBrowsing,
		entries: []network.Entry{
			{Name: "Song 1"},
			{Name: "Song 2"},
			{Name: "Song 3"},
			{Name: "Song 4"},
			{Name: "Song 5"},
			{Name: "Song 6"},
			{Name: "Song 7"},
			{Name: "Song 8"},
			{Name: "Song 9"},
			{Name: "Song 10"},
			{Name: "Song 11"},
			{Name: "Song 12"},
			{Name: "Song 13"},
			{Name: "Song 14"},
			{Name: "Song 15"},
		},
		cursor:       0,
		scrollOffset: 0,
	}

	// Helper to send key press
	pressKey := func(model Model, key string) Model {
		newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		return newModel.(Model)
	}

	// Initially: cursor should be 0, scrollOffset should be 0
	if m.cursor != 0 || m.scrollOffset != 0 {
		t.Fatalf("expected initial cursor=0, scrollOffset=0, got cursor=%d, scrollOffset=%d", m.cursor, m.scrollOffset)
	}

	// Press down 7 times to reach the 8th song (index 7)
	for i := 0; i < 7; i++ {
		m = pressKey(m, "j")
	}

	// At 8th position song (index 7)
	if m.cursor != 7 {
		t.Errorf("expected cursor at index 7, got %d", m.cursor)
	}
	if m.scrollOffset != 0 {
		t.Errorf("expected scrollOffset to be 0 when cursor is at index 7, got %d", m.scrollOffset)
	}

	// Press down 1 more time (8th time) -> index 8 (9th song)
	// With pageSize=10 and scrollBuffer=2, scrollOffset should now be 1
	// because index 8 is >= scrollOffset + pageSize - scrollBuffer (0 + 10 - 2 = 8)
	// scrollOffset = 8 - 10 + 1 + 2 = 1
	m = pressKey(m, "j")
	if m.cursor != 8 {
		t.Errorf("expected cursor at index 8, got %d", m.cursor)
	}
	if m.scrollOffset != 1 {
		t.Errorf("expected scrollOffset to be 1, got %d", m.scrollOffset)
	}

	// Press down 1 more time (9th time) -> index 9 (10th song)
	// scrollOffset should now be 2
	m = pressKey(m, "j")
	if m.cursor != 9 {
		t.Errorf("expected cursor at index 9, got %d", m.cursor)
	}
	if m.scrollOffset != 2 {
		t.Errorf("expected scrollOffset to be 2, got %d", m.scrollOffset)
	}

	// Verify top visible song (scrollOffset) is index 2 (3rd position)
	// and last visible song (scrollOffset + 9) is index 11 (12th position)
	lastVisibleIndex := m.scrollOffset + 9
	if lastVisibleIndex != 11 {
		t.Errorf("expected last visible song to be index 11 (12th position), got index %d", lastVisibleIndex)
	}

	// Press down 5 more times to reach the end (index 14)
	for i := 0; i < 5; i++ {
		m = pressKey(m, "j")
	}
	if m.cursor != 14 {
		t.Errorf("expected cursor to be at index 14, got %d", m.cursor)
	}
	// scrollOffset should be clamped to 5 (len(entries) - pageSize = 15 - 10 = 5)
	if m.scrollOffset != 5 {
		t.Errorf("expected scrollOffset at the end to be 5, got %d", m.scrollOffset)
	}

	// Press up 7 times -> cursor moves from 14 to 7
	// cursor 7 >= scrollOffset + scrollBuffer (5 + 2 = 7), so scrollOffset remains 5
	for i := 0; i < 7; i++ {
		m = pressKey(m, "k")
	}
	if m.cursor != 7 {
		t.Errorf("expected cursor at index 7, got %d", m.cursor)
	}
	if m.scrollOffset != 5 {
		t.Errorf("expected scrollOffset to remain 5, got %d", m.scrollOffset)
	}

	// Press up 1 more time -> cursor 6 < 5 + 2 = 7, scrollOffset becomes 6 - 2 = 4
	m = pressKey(m, "k")
	if m.cursor != 6 {
		t.Errorf("expected cursor at index 6, got %d", m.cursor)
	}
	if m.scrollOffset != 4 {
		t.Errorf("expected scrollOffset to be 4, got %d", m.scrollOffset)
	}

	// Press up 6 more times to return to top (cursor 0)
	for i := 0; i < 6; i++ {
		m = pressKey(m, "k")
	}
	if m.cursor != 0 {
		t.Errorf("expected cursor to return to 0, got %d", m.cursor)
	}
	if m.scrollOffset != 0 {
		t.Errorf("expected scrollOffset to return to 0, got %d", m.scrollOffset)
	}
}
