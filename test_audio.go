package main

import (
	"fmt"
	"time"

	"bitbeat/internal/audio"
	"bitbeat/internal/network"
)

func main() {
	engine, err := audio.NewEngine()
	if err != nil {
		fmt.Printf("Error creating engine: %v\n", err)
		return
	}
	defer engine.Close()

	client := network.NewClient("")
	streamURL := "https://codeberg.org/MalG4850/MyMusic/raw/branch/main/English/Metal/Seek%20And%20Destroy%20%E2%80%94%20Metallica.mp3"

	resp, err := client.GetStream(streamURL)
	if err != nil {
		fmt.Printf("Error getting stream: %v\n", err)
		return
	}

	fmt.Println("Loading stream...")
	err = engine.LoadStream(resp.Body, "Seek And Destroy — Metallica.mp3", streamURL)
	if err != nil {
		fmt.Printf("Error loading stream: %v\n", err)
		return
	}

	fmt.Println("Playing...")
	engine.Play()

	for i := 0; i < 24; i++ {
		time.Sleep(500 * time.Millisecond)
		curr, tot := engine.GetProgress()
		status := engine.GetStatus()
		fmt.Printf("Tick %d: currSec=%f, totSec=%f, status=%v\n", i, curr, tot, status)
	}
}
