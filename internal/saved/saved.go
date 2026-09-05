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

// SaveAllEntries marshals and writes a full slice of entries back to disk
func SaveAllEntries(entries []Entry) error {
	dir := filepath.Dir(savedEntriesFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(savedEntriesFile, data, 0644)
}

// SaveEntry saves a new entry to the file
func SaveEntry(title, url string) error {
	entries, err := LoadEntries()
	if err != nil {
		return err
	}

	entries = append(entries, Entry{Title: title, URL: url})
	return SaveAllEntries(entries)
}

// DeleteEntry removes an entry at the specified index and writes the updated list back to disk
func DeleteEntry(index int) ([]Entry, error) {
	entries, err := LoadEntries()
	if err != nil {
		return nil, err
	}

	if index < 0 || index >= len(entries) {
		return entries, nil
	}

	// Remove item at index
	entries = append(entries[:index], entries[index+1:]...)

	if err := SaveAllEntries(entries); err != nil {
		return nil, err
	}

	return entries, nil
}
