package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"bitbeat/internal/audio"
	"bitbeat/internal/config"
	"bitbeat/internal/network"
	"bitbeat/internal/saved"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tickMsg time.Time
type playSuccessMsg struct{}

type sessionState int

const (
	stateBrowsing sessionState = iota
	stateInputting
	stateMainMenu
	stateSavedEntries
	stateAddEntry
)

const (
	scrollBuffer = 2
)

type Model struct {
	config          *config.Config
	client          *network.Client
	engine          *audio.Engine
	entries         []network.Entry
	playingEntries  []network.Entry
	playingIndex    int
	playingPath     string
	cursor          int
	scrollOffset    int
	err             error
	ready           bool
	width           int
	height          int
	currSec         float64
	totSec          float64
	status          audio.PlaybackStatus
	state           sessionState
	textInput       textinput.Model
	currPath        string
	loading         bool
	savedEntries    []saved.Entry
	menuCursor      int
	entryTitleInput textinput.Model
	entryURLInput   textinput.Model
}

func NewModel(cfg *config.Config, client *network.Client, engine *audio.Engine) Model {
	ti := textinput.New()
	ti.Placeholder = "Enter repository link (e.g. Codeberg)..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 50

	titleInput := textinput.New()
	titleInput.Placeholder = "Enter entry title..."
	titleInput.CharLimit = 100
	titleInput.Width = 50

	urlInput := textinput.New()
	urlInput.Placeholder = "Enter URL (Codeberg or SoundCloud)..."
	urlInput.CharLimit = 156
	urlInput.Width = 50

	return Model{
		config:          cfg,
		client:          client,
		engine:          engine,
		status:          audio.StatusStopped,
		state:           stateMainMenu,
		textInput:       ti,
		entryTitleInput: titleInput,
		entryURLInput:   urlInput,
		playingIndex:    -1,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tick())
}

func (m Model) fetchEntries(path string) tea.Cmd {
	return func() tea.Msg {
		entries, err := m.client.FetchEntries(path)
		if err != nil {
			return err
		}
		return entries
	}
}

func tick() tea.Cmd {
	return tea.Tick(time.Millisecond*500, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		// Don't return, let other handlers process it if needed

	case tickMsg:
		m.currSec, m.totSec = m.engine.GetProgress()
		m.status = m.engine.GetStatus()

		// Write debug ticks log
		f, _ := os.OpenFile("debug_ticks.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f != nil {
			_, _ = f.WriteString(fmt.Sprintf("[%s] currSec: %f, totSec: %f, status: %v, state: %v\n", time.Now().Format("15:04:05"), m.currSec, m.totSec, m.status, m.state))
			f.Close()
		}

		if m.err == nil && !m.loading && m.engine.IsFinished() {
			// Check for premature EOF/interruption (e.g. connection cut off)
			if m.totSec > 0 && m.currSec < m.totSec-3.0 {
				m.err = fmt.Errorf("playback interrupted: connection lost or stream ended prematurely")
				m.loading = false
				m.engine.Stop()
				return m, tick()
			}
			m.loading = true
			m.currSec = 0
			m.totSec = 0
			nextModel, nextCmd := m.nextSong()
			return nextModel, tea.Batch(nextCmd, tick())
		}
		return m, tick()

	case playSuccessMsg:
		m.loading = false
		m.err = nil
		return m, nil

	case []network.Entry:
		m.entries = msg
		m.err = nil
		m.scrollOffset = 0
		return m, nil

	case error:
		m.err = msg
		m.loading = false
		return m, nil
	}

	if m.err != nil {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				m.err = nil
				return m, nil
			case m.config.Keybindings.Quit, "ctrl+c", "q":
				return m, tea.Quit
			case "l":
				m.err = nil
				m.state = stateInputting
				m.textInput.Focus()
				return m, nil
			}
		}
		return m, nil
	}

	switch m.state {
	case stateMainMenu:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case m.config.Keybindings.Quit, "ctrl+c", "q":
				return m, tea.Quit
			case "up", "k":
				if m.menuCursor > 0 {
					m.menuCursor--
				}
			case "down", "j":
				if m.menuCursor < 2 {
					m.menuCursor++
				}
			case "enter":
				switch m.menuCursor {
				case 0: // Saved Entries
					entries, err := saved.LoadEntries()
					if err != nil {
						m.err = err
					} else {
						m.savedEntries = entries
						m.menuCursor = 0
						m.state = stateSavedEntries
					}
				case 1: // Link
					m.state = stateInputting
					m.textInput.Focus()
				case 2: // Exit
					return m, tea.Quit
				}
			}
		}
		return m, nil

	case stateSavedEntries:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case m.config.Keybindings.Quit, "ctrl+c", "q":
				return m, tea.Quit
			case "up", "k":
				if m.menuCursor > 0 {
					m.menuCursor--
				}
			case "down", "j":
				if m.menuCursor < len(m.savedEntries) {
					m.menuCursor++
				}
			case "enter":
				if m.menuCursor < len(m.savedEntries) {
					// Load selected saved entry
					selected := m.savedEntries[m.menuCursor]
					m.client.BaseURL = selected.URL
					m.currPath = ""
					m.state = stateBrowsing
					m.cursor = 0
					m.scrollOffset = 0
					m.err = nil
					return m, m.fetchEntries("")
				} else {
					// "Add an Entry" option selected
					m.state = stateAddEntry
					m.entryTitleInput.Reset()
					m.entryURLInput.Reset()
					m.entryTitleInput.Focus()
				}
			case "backspace", "esc":
				m.state = stateMainMenu
				m.menuCursor = 0
			}
		}
		return m, nil

	case stateAddEntry:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.state = stateSavedEntries
				m.menuCursor = len(m.savedEntries) // place cursor on "Add an Entry"
				return m, nil
			case "tab", "down":
				if m.entryTitleInput.Focused() {
					m.entryTitleInput.Blur()
					m.entryURLInput.Focus()
				} else {
					m.entryURLInput.Blur()
					m.entryTitleInput.Focus()
				}
			case "up":
				if m.entryTitleInput.Focused() {
					m.entryTitleInput.Blur()
					m.entryURLInput.Focus()
				} else {
					m.entryURLInput.Blur()
					m.entryTitleInput.Focus()
				}
			case "enter":
				if m.entryTitleInput.Focused() {
					title := strings.TrimSpace(m.entryTitleInput.Value())
					if title != "" {
						m.entryTitleInput.Blur()
						m.entryURLInput.Focus()
					}
				} else {
					title := strings.TrimSpace(m.entryTitleInput.Value())
					url := strings.TrimSpace(m.entryURLInput.Value())
					if title != "" && url != "" {
						err := saved.SaveEntry(title, url)
						if err != nil {
							m.err = err
						} else {
							// Reload entries and go back
							entries, err := saved.LoadEntries()
							if err != nil {
								m.err = err
							} else {
								m.savedEntries = entries
								m.menuCursor = len(entries) - 1 // highlight the newly added entry
								m.state = stateSavedEntries
							}
						}
					}
				}
			}
		}

		if m.entryTitleInput.Focused() {
			m.entryTitleInput, cmd = m.entryTitleInput.Update(msg)
		} else if m.entryURLInput.Focused() {
			m.entryURLInput, cmd = m.entryURLInput.Update(msg)
		}
		return m, cmd

	case stateInputting:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				url := m.textInput.Value()
				if url != "" {
					m.client.BaseURL = url
					m.currPath = ""
					m.state = stateBrowsing
					return m, m.fetchEntries("")
				}
			case "esc":
				m.state = stateMainMenu
				m.menuCursor = 1 // focus on "Link" in main menu
				return m, nil
			}
		}
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd

	case stateBrowsing:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case m.config.Keybindings.Quit, "ctrl+c", "q":
				return m, tea.Quit
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
					m.adjustScrollOffset()
				}
			case "down", "j":
				if m.cursor < len(m.entries)-1 {
					m.cursor++
					m.adjustScrollOffset()
				}
			case "enter":
				if len(m.entries) > 0 {
					entry := m.entries[m.cursor]
					if entry.IsFolder {
						m.currPath = entry.Path
						m.cursor = 0
						m.scrollOffset = 0
						m.err = nil
						return m, m.fetchEntries(m.currPath)
					}
					m.playingEntries = m.entries
					m.playingIndex = m.cursor
					m.playingPath = m.currPath
					m.loading = true
					m.currSec = 0
					m.totSec = 0
					m.err = nil
					return m, m.playEntry(entry)
				}
			case "backspace":
				if m.currPath != "" {
					parts := strings.Split(strings.TrimSuffix(m.currPath, "/"), "/")
					if len(parts) > 0 {
						m.currPath = strings.Join(parts[:len(parts)-1], "/")
						m.cursor = 0
						m.scrollOffset = 0
						m.err = nil
						return m, m.fetchEntries(m.currPath)
					}
				} else {
					// At root, go back to main menu
					m.state = stateMainMenu
					m.menuCursor = 0
					return m, nil
				}
			case "esc":
				if m.err != nil {
					m.err = nil
					return m, nil
				}
				m.state = stateMainMenu
				m.menuCursor = 0
				return m, nil
			case "l":
				m.state = stateInputting
				m.textInput.Focus()
				return m, nil
			case m.config.Keybindings.PlayPause:
				if m.status == audio.StatusPlaying {
					m.engine.Pause()
					m.status = audio.StatusPaused
				} else if m.status == audio.StatusPaused {
					m.engine.Play()
					m.status = audio.StatusPlaying
				}
			case m.config.Keybindings.NextTrack:
				if !m.loading {
					m.engine.Stop()
					m.loading = true
					m.currSec = 0
					m.totSec = 0
					m.err = nil
					nextModel, nextCmd := m.nextSong()
					return nextModel, nextCmd
				}
			case m.config.Keybindings.PrevTrack:
				if !m.loading {
					m.engine.Stop()
					m.loading = true
					m.currSec = 0
					m.totSec = 0
					m.err = nil
					prevModel, prevCmd := m.prevSong()
					return prevModel, prevCmd
				}
			}
		}
	}

	return m, nil
}

func (m *Model) adjustScrollOffset() {
	ps := m.getPageSize()
	if len(m.entries) <= ps {
		m.scrollOffset = 0
		return
	}

	sb := scrollBuffer
	if ps <= 2*scrollBuffer {
		sb = 0
	}

	// Adjust scrollOffset if cursor is too close to the top of the viewport
	if m.cursor < m.scrollOffset+sb {
		m.scrollOffset = m.cursor - sb
	}

	// Adjust scrollOffset if cursor is too close to the bottom of the viewport
	if m.cursor >= m.scrollOffset+ps-sb {
		m.scrollOffset = m.cursor - ps + 1 + sb
	}

	// Clamp scrollOffset
	maxScroll := len(m.entries) - ps
	if m.scrollOffset > maxScroll {
		m.scrollOffset = maxScroll
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

func (m Model) nextSong() (tea.Model, tea.Cmd) {
	if len(m.playingEntries) == 0 {
		m.loading = false
		m.engine.Stop()
		return m, nil
	}

	startIdx := m.playingIndex
	nextIdx := (startIdx + 1) % len(m.playingEntries)

	// Find next non-folder entry
	for nextIdx != startIdx {
		if !m.playingEntries[nextIdx].IsFolder {
			m.playingIndex = nextIdx
			if m.currPath == m.playingPath {
				m.cursor = nextIdx
				m.adjustScrollOffset()
			}
			m.currSec = 0
			m.totSec = 0
			return m, m.playEntry(m.playingEntries[nextIdx])
		}
		nextIdx = (nextIdx + 1) % len(m.playingEntries)
	}

	// If we looped back and it's not a folder, play it anyway
	if !m.playingEntries[startIdx].IsFolder {
		m.currSec = 0
		m.totSec = 0
		return m, m.playEntry(m.playingEntries[startIdx])
	}

	m.loading = false
	m.engine.Stop()
	return m, nil
}

func (m Model) prevSong() (tea.Model, tea.Cmd) {
	if len(m.playingEntries) == 0 {
		m.loading = false
		m.engine.Stop()
		return m, nil
	}

	startIdx := m.playingIndex
	prevIdx := (startIdx - 1 + len(m.playingEntries)) % len(m.playingEntries)

	// Find previous non-folder entry
	for prevIdx != startIdx {
		if !m.playingEntries[prevIdx].IsFolder {
			m.playingIndex = prevIdx
			if m.currPath == m.playingPath {
				m.cursor = prevIdx
				m.adjustScrollOffset()
			}
			m.currSec = 0
			m.totSec = 0
			return m, m.playEntry(m.playingEntries[prevIdx])
		}
		prevIdx = (prevIdx - 1 + len(m.playingEntries)) % len(m.playingEntries)
	}

	// If we looped back and it's not a folder, play it anyway
	if !m.playingEntries[startIdx].IsFolder {
		m.currSec = 0
		m.totSec = 0
		return m, m.playEntry(m.playingEntries[startIdx])
	}

	m.loading = false
	m.engine.Stop()
	return m, nil
}

func (m Model) playEntry(entry network.Entry) tea.Cmd {
	return func() tea.Msg {
		if entry.URL == "" {
			return fmt.Errorf("no download URL for this entry")
		}

		streamURL := entry.URL
		var err error

		// Resolve SoundCloud URL if needed
		if strings.Contains(entry.URL, "soundcloud.com") {
			streamURL, err = m.client.GetSoundCloudStream(entry.URL)
			if err != nil {
				return err
			}
		}

		resp, err := m.client.GetStream(streamURL)
		if err != nil {
			return err
		}
		if err := m.engine.LoadStream(resp.Body, entry.Name, streamURL); err != nil {
			return err
		}
		m.engine.Play()
		return playSuccessMsg{}
	}
}

func (m Model) renderPlayerStatus() string {
	if m.status == audio.StatusStopped {
		return ""
	}

	statusStr := "[STOPPED]"
	if m.status == audio.StatusPlaying {
		statusStr = "[PLAYING]"
	} else if m.status == audio.StatusPaused {
		statusStr = "[PAUSED]"
	}

	progressBarWidth := m.width - 30
	if progressBarWidth < 10 {
		progressBarWidth = 10
	}
	if progressBarWidth > 60 {
		progressBarWidth = 60
	}

	progress := 0.0
	if m.totSec > 0 {
		progress = m.currSec / m.totSec
	}

	filled := int(progress * float64(progressBarWidth))
	if filled > progressBarWidth {
		filled = progressBarWidth
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", progressBarWidth-filled)

	return fmt.Sprintf("%s [%s] %02d:%02d / %02d:%02d\n",
		statusStr,
		bar,
		int(m.currSec)/60, int(m.currSec)%60,
		int(m.totSec)/60, int(m.totSec)%60)
}

func (m Model) View() string {
	var content string

	if m.err != nil {
		errMsg := m.err.Error()
		if strings.Contains(errMsg, "free bitrate format is not supported") {
			errMsg = "MP3 Format Error: This file uses a 'free bitrate' or VBR format not supported by the current engine.\nSuggestion: Re-encode the file using FFmpeg (constant bitrate)."
		} else if strings.Contains(errMsg, "lookup") || strings.Contains(errMsg, "dial tcp") {
			errMsg = "Network Error: Could not reach the repository host. Please check your internet connection or DNS settings.\nSuggestion: Ensure the URL is correct and you are online."
		}
		content = fmt.Sprintf("Error: %s\n\nPress 'esc' to clear error, 'l' to try a different link or %s to quit", errMsg, m.config.Keybindings.Quit)
	} else if !m.ready {
		content = "Initializing..."
	} else {
		switch m.state {
		case stateMainMenu:
			var s strings.Builder
			headerStyle := lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("5")).
				Padding(0, 1)
			s.WriteString(headerStyle.Render("BitBeat - Terminal Audio Engine"))
			s.WriteString("\n\n")

			options := []string{"Saved Entries", "Link", "Exit"}
			for i, opt := range options {
				cursor := " "
				if m.menuCursor == i {
					cursor = ">"
				}
				style := lipgloss.NewStyle()
				if m.menuCursor == i {
					style = style.Foreground(lipgloss.Color("2")).Bold(true)
				}
				s.WriteString(fmt.Sprintf("%s %s\n", cursor, style.Render(opt)))
			}

			// Show player status if a song is playing in the background
			if m.status != audio.StatusStopped {
				s.WriteString("\n")
				s.WriteString(m.renderPlayerStatus())
			}

			s.WriteString("\nPress 'q' to exit.\n")
			content = s.String()

		case stateSavedEntries:
			var s strings.Builder
			headerStyle := lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("5")).
				Padding(0, 1)
			s.WriteString(headerStyle.Render("BitBeat - Saved Entries"))
			s.WriteString("\n\n")

			if len(m.savedEntries) == 0 {
				s.WriteString("  (no saved entries)\n")
			}

			for i, entry := range m.savedEntries {
				cursor := " "
				if m.menuCursor == i {
					cursor = ">"
				}
				style := lipgloss.NewStyle()
				if m.menuCursor == i {
					style = style.Foreground(lipgloss.Color("2")).Bold(true)
				}
				s.WriteString(fmt.Sprintf("%s %s\n", cursor, style.Render(entry.Title)))
			}

			// Add an Entry option
			addCursor := " "
			if m.menuCursor == len(m.savedEntries) {
				addCursor = ">"
			}
			addStyle := lipgloss.NewStyle()
			if m.menuCursor == len(m.savedEntries) {
				addStyle = addStyle.Foreground(lipgloss.Color("2")).Bold(true)
			}
			s.WriteString(fmt.Sprintf("%s %s\n", addCursor, addStyle.Render("+ Add an Entry")))

			// Show player status if playing
			if m.status != audio.StatusStopped {
				s.WriteString("\n")
				s.WriteString(m.renderPlayerStatus())
			}

			s.WriteString("\nPress 'backspace' or 'esc' to go back, 'q' to quit.\n")
			content = s.String()

		case stateAddEntry:
			var s strings.Builder
			headerStyle := lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("5")).
				Padding(0, 1)
			s.WriteString(headerStyle.Render("BitBeat - Add Saved Entry"))
			s.WriteString("\n\n")

			s.WriteString("Title:\n")
			s.WriteString(m.entryTitleInput.View())
			s.WriteString("\n\nURL:\n")
			s.WriteString(m.entryURLInput.View())
			s.WriteString("\n\n")

			if m.entryTitleInput.Focused() {
				s.WriteString("(Press Enter or Tab to go to URL input)")
			} else {
				s.WriteString("(Press Enter to save, Tab/Up to go back to Title)")
			}
			s.WriteString("\n\n(esc to cancel)")
			content = s.String()

		case stateInputting:
			content = fmt.Sprintf(
				"Enter Repository URL:\n\n%s\n\n(esc to cancel)",
				m.textInput.View(),
			) + "\n"

		case stateBrowsing:
			var s strings.Builder

			// Header
			headerStyle := lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("5")).
				Padding(0, 1)
			s.WriteString(headerStyle.Render("BitBeat - Terminal Audio Engine"))
			s.WriteString("\n")
			s.WriteString(fmt.Sprintf("Path: /%s\n\n", m.currPath))

			// Entry List
			if len(m.entries) == 0 {
				s.WriteString("  (empty folder)\n")
			}

			ps := m.getPageSize()
			end := m.scrollOffset + ps
			if end > len(m.entries) {
				end = len(m.entries)
			}

			for i := m.scrollOffset; i < end; i++ {
				entry := m.entries[i]
				cursor := " "
				if m.cursor == i {
					cursor = ">"
				}

				style := lipgloss.NewStyle()
				if m.cursor == i {
					style = style.Foreground(lipgloss.Color("2")).Bold(true)
				}

				icon := "󰉋" // folder
				if !entry.IsFolder {
					icon = "󰎆" // file/music
				}

				displayName := entry.Name
				maxNameLen := 45
				if m.width > 12 {
					maxNameLen = m.width - 12
				}
				if maxNameLen < 20 {
					maxNameLen = 20
				}
				displayName = truncateString(displayName, maxNameLen)

				s.WriteString(fmt.Sprintf("%s %s %s\n", cursor, icon, style.Render(displayName)))
			}

			s.WriteString("\n")

			// Player Status
			if m.status != audio.StatusStopped {
				s.WriteString(m.renderPlayerStatus())
			}

			s.WriteString("\nPress 'l' to input link, 'backspace' to go up, 'q' to quit.\n")
			content = s.String()
		}
	}

	if m.width > 0 && m.height > 0 {
		contentHeight := lipgloss.Height(content)
		contentWidth := lipgloss.Width(content)

		vAlign := lipgloss.Center
		if m.height < contentHeight {
			vAlign = lipgloss.Top
		}

		hAlign := lipgloss.Center
		if m.width < contentWidth {
			hAlign = lipgloss.Left
		}

		placed := lipgloss.Place(m.width, m.height, hAlign, vAlign, content)
		maxLines := m.height - 1
		if maxLines < 1 {
			maxLines = 1
		}
		return limitLines(placed, maxLines)
	}
	return content
}

func limitLines(s string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[:maxLines], "\n")
}

func (m Model) getPageSize() int {
	return 10
}

func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen < 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}
