package ui

import (
	"fmt"
	"strings"
	"time"

	"bitbeat/internal/audio"
	"bitbeat/internal/config"
	"bitbeat/internal/network"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tickMsg time.Time

type sessionState int

const (
	stateBrowsing sessionState = iota
	stateInputting
)

type Model struct {
	config    *config.Config
	client    *network.Client
	engine    *audio.Engine
	entries   []network.Entry
	cursor    int
	err       error
	ready     bool
	width     int
	height    int
	currSec   float64
	totSec    float64
	status    audio.PlaybackStatus
	state     sessionState
	textInput textinput.Model
	currPath  string
}

func NewModel(cfg *config.Config, client *network.Client, engine *audio.Engine) Model {
	ti := textinput.New()
	ti.Placeholder = "Enter repository link (e.g. Codeberg)..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 50

	return Model{
		config:    cfg,
		client:    client,
		engine:    engine,
		status:    audio.StatusStopped,
		state:     stateInputting,
		textInput: ti,
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
		return m, tick()

	case []network.Entry:
		m.entries = msg
		m.err = nil
		return m, nil

	case error:
		m.err = msg
		return m, nil
	}

	switch m.state {
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
				m.state = stateBrowsing
				return m, nil
			}
		}
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd

	case stateBrowsing:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case m.config.Keybindings.Quit, "ctrl+c":
				return m, tea.Quit
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.entries)-1 {
					m.cursor++
				}
			case "enter":
				if len(m.entries) > 0 {
					entry := m.entries[m.cursor]
					if entry.IsFolder {
						m.currPath = entry.Path
						m.cursor = 0
						return m, m.fetchEntries(m.currPath)
					}
					return m, m.playEntry(entry)
				}
			case "backspace":
				if m.currPath != "" {
					parts := strings.Split(strings.TrimSuffix(m.currPath, "/"), "/")
					if len(parts) > 0 {
						m.currPath = strings.Join(parts[:len(parts)-1], "/")
						m.cursor = 0
						return m, m.fetchEntries(m.currPath)
					}
				}
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
			}
		}
	}

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
		return nil
	}
}

func (m Model) View() string {
	if m.state == stateInputting {
		return fmt.Sprintf(
			"Enter Repository URL:\n\n%s\n\n(esc to cancel)",
			m.textInput.View(),
		) + "\n"
	}

	if m.err != nil {
		errMsg := m.err.Error()
		if strings.Contains(errMsg, "free bitrate format is not supported") {
			errMsg = "MP3 Format Error: This file uses a 'free bitrate' or VBR format not supported by the current engine.\nSuggestion: Re-encode the file using FFmpeg (constant bitrate)."
		} else if strings.Contains(errMsg, "lookup") || strings.Contains(errMsg, "dial tcp") {
			errMsg = "Network Error: Could not reach the repository host. Please check your internet connection or DNS settings.\nSuggestion: Ensure the URL is correct and you are online."
		}
		return fmt.Sprintf("Error: %s\n\nPress 'l' to try a different link or %s to quit", errMsg, m.config.Keybindings.Quit)
	}

	if !m.ready {
		return "Initializing..."
	}

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
	for i, entry := range m.entries {
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

		s.WriteString(fmt.Sprintf("%s %s %s\n", cursor, icon, style.Render(entry.Name)))
	}

	s.WriteString("\n")

	// Player Status
	if m.status != audio.StatusStopped {
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

		s.WriteString(fmt.Sprintf("%s [%s] %02d:%02d / %02d:%02d\n", 
			statusStr, 
			bar, 
			int(m.currSec)/60, int(m.currSec)%60, 
			int(m.totSec)/60, int(m.totSec)%60))
	}

	s.WriteString("\nPress 'l' to input link, 'backspace' to go up, 'q' to quit.\n")

	return s.String()
}
