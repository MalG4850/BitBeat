package network

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	soundcloudapi "github.com/zackradisic/soundcloud-api"
)

type Entry struct {
	Name     string
	IsFolder bool
	Path     string
	URL      string
}

type Client struct {
	BaseURL  string
	scClient *soundcloudapi.API
}

func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL:  baseURL,
		scClient: nil,
	}
}

func (c *Client) FetchEntries(path string) ([]Entry, error) {
	// Simple detection for SoundCloud links
	if strings.Contains(c.BaseURL, "soundcloud.com") {
		return c.fetchSoundCloudEntries(path)
	}

	// Simple detection for Codeberg links
	if strings.Contains(c.BaseURL, "codeberg.org") {
		return c.fetchCodebergEntries(path)
	}

	// Fallback to original behavior or other parsers
	resp, err := http.Get(c.BaseURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch: status %d", resp.StatusCode)
	}

	var tracks []struct {
		Title    string `json:"title"`
		Artist   string `json:"artist"`
		Filename string `json:"filename"`
		URL      string `json:"stream_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tracks); err != nil {
		return nil, err
	}

	var entries []Entry
	for _, t := range tracks {
		name := t.Title
		if name == "" {
			name = t.Filename
		}
		entries = append(entries, Entry{
			Name:     name,
			IsFolder: false,
			URL:      t.URL,
		})
	}

	return entries, nil
}

func (c *Client) fetchCodebergEntries(path string) ([]Entry, error) {
	parts := strings.Split(strings.TrimPrefix(strings.TrimSuffix(c.BaseURL, "/"), "https://"), "/")
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid Codeberg URL")
	}
	owner := parts[1]
	repo := parts[2]

	apiURL := fmt.Sprintf("https://codeberg.org/api/v1/repos/%s/%s/contents/%s", owner, repo, path)
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Codeberg API error: status %d", resp.StatusCode)
	}

	var contents []struct {
		Name        string `json:"name"`
		Type        string `json:"type"`
		Path        string `json:"path"`
		DownloadURL string `json:"download_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&contents); err != nil {
		return nil, err
	}

	var entries []Entry
	for _, item := range contents {
		entries = append(entries, Entry{
			Name:     item.Name,
			IsFolder: item.Type == "dir",
			Path:     item.Path,
			URL:      item.DownloadURL,
		})
	}

	return entries, nil
}

func (c *Client) fetchSoundCloudEntries(path string) ([]Entry, error) {
	if c.scClient == nil {
		sc, err := soundcloudapi.New(soundcloudapi.APIOptions{})
		if err != nil {
			return nil, err
		}
		c.scClient = sc
	}

	tracks, err := c.scClient.GetTrackInfo(soundcloudapi.GetTrackInfoOptions{
		URL: c.BaseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("SoundCloud error: %v", err)
	}

	var entries []Entry
	for _, track := range tracks {
		entries = append(entries, Entry{
			Name:     fmt.Sprintf("%s - %s", track.User.Username, track.Title),
			IsFolder: false,
			URL:      track.PermalinkURL,
		})
	}

	return entries, nil
}

func (c *Client) GetSoundCloudStream(url string) (string, error) {
	if c.scClient == nil {
		sc, err := soundcloudapi.New(soundcloudapi.APIOptions{})
		if err != nil {
			return "", err
		}
		c.scClient = sc
	}
	tracks, err := c.scClient.GetTrackInfo(soundcloudapi.GetTrackInfoOptions{
		URL: url,
	})
	if err != nil || len(tracks) == 0 {
		return "", fmt.Errorf("failed to get SoundCloud track info: %v", err)
	}

	streamURL, err := c.scClient.GetDownloadURL(url, "progressive")
	if err != nil {
		streamURL, err = c.scClient.GetDownloadURL(url, "hls")
		if err != nil {
			return "", fmt.Errorf("failed to get SoundCloud stream URL: %v", err)
		}
	}
	return streamURL, nil
}

func (c *Client) GetStream(url string) (http.Response, error) {
	resp, err := http.Get(url)
	if err != nil {
		return http.Response{}, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return http.Response{}, fmt.Errorf("failed to get stream: status %d", resp.StatusCode)
	}
	return *resp, nil
}
