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
	// Let's use a popular SoundCloud track URL
	soundcloudURL := "https://soundcloud.com/octobersveryown/drake-back-to-back"
	client.BaseURL = soundcloudURL

	fmt.Println("Resolving SoundCloud track...")
	entries, err := client.FetchEntries("")
	if err != nil {
		fmt.Printf("Error fetching SoundCloud entries: %v\n", err)
		return
	}
	if len(entries) == 0 {
		fmt.Println("No entries found for SoundCloud URL")
		return
	}

	entry := entries[0]
	fmt.Printf("Playing SoundCloud entry: %s (URL: %s)\n", entry.Name, entry.URL)

	streamURL, err := client.GetSoundCloudStream(entry.URL)
	if err != nil {
		fmt.Printf("Error getting SoundCloud stream URL: %v\n", err)
		return
	}

	fmt.Printf("Resolved Stream URL: %s\n", streamURL)

	resp, err := client.GetStream(streamURL)
	if err != nil {
		fmt.Printf("Error getting stream: %v\n", err)
		return
	}

	fmt.Println("Loading stream...")
	err = engine.LoadStream(resp.Body, entry.Name, streamURL)
	if err != nil {
		fmt.Printf("Error loading stream: %v\n", err)
		return
	}

	fmt.Println("Playing...")
	engine.Play()

	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		curr, tot := engine.GetProgress()
		status := engine.GetStatus()
		fmt.Printf("Tick %d: currSec=%f, totSec=%f, status=%v\n", i, curr, tot, status)
	}
}
