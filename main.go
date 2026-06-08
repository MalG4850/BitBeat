package main

import (
	"fmt"
	"os"

	"bitbeat/internal/audio"
	"bitbeat/internal/config"
	"bitbeat/internal/network"
	"bitbeat/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	client := network.NewClient(cfg.RepositoryURL)

	engine, err := audio.NewEngine()
	if err != nil {
		fmt.Printf("Error initializing audio engine: %v\n", err)
		os.Exit(1)
	}
	defer engine.Close()

	p := tea.NewProgram(ui.NewModel(cfg, client, engine), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
