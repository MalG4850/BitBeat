package saved

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Entry represents a saved link with a title
type Entry struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

// File to store saved entries
const savedEntriesFile = "saved_entries.json"

// LoadEntries loads saved entries from file
func LoadEntries() ([]Entry, error) {
	// Try to read the file
	data, err := os.ReadFile(savedEntriesFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty slice if file doesn't exist
			return []Entry{}, nil
		}
		return nil, err
	}

	// Parse JSON data
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}

	return entries, nil
}

// SaveEntry saves a new entry to the file
func SaveEntry(title, url string) error {
	// Load existing entries
	entries, err := LoadEntries()
	if err != nil {
		return err
	}

	// Add new entry
	entries = append(entries, Entry{Title: title, URL: url})

	// Create directory if it doesn't exist
	dir := filepath.Dir(savedEntriesFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Write entries back to file
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(savedEntriesFile, data, 0644)
}