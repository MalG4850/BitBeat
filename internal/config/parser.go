package config

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	RepositoryURL string           `toml:"repository_url"`
	DefaultFolder string           `toml:"default_folder"`
	Keybindings   KeybindingConfig `toml:"keybindings"`
	Audio         AudioConfig      `toml:"audio"`
}

type KeybindingConfig struct {
	Quit        string `toml:"quit"`
	PlayPause   string `toml:"play_pause"`
	NextTrack   string `toml:"next_track"`
	PrevTrack   string `toml:"prev_track"`
	FastForward string `toml:"fast_forward"`
	Rewind      string `toml:"rewind"`
}

type AudioConfig struct {
	SkipIntervalSec int `toml:"skip_interval_seconds"`
	DefaultVolume   int `toml:"default_volume"`
}

func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()

	f, err := os.Open("config.toml")
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	defer f.Close()

	if err := toml.NewDecoder(f).Decode(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func DefaultConfig() *Config {
	return &Config{
		RepositoryURL: "https://raw.githubusercontent.com/learn-music-theory/music-assets/main/catalog.json",
		DefaultFolder: "music",
		Keybindings: KeybindingConfig{
			Quit:        "q",
			PlayPause:   " ",
			NextTrack:   "n",
			PrevTrack:   "p",
			FastForward: "l",
			Rewind:      "h",
		},
		Audio: AudioConfig{
			SkipIntervalSec: 10,
			DefaultVolume:   50,
		},
	}
}
