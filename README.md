Here is a comprehensive, production-grade `README.md` written explicitly for an **Agentic Coder / LLM Software Engineer**. It uses precise architecture blueprints, clear interface definitions, state machine boundaries, and technical constraints to ensure an LLM agent can ingest it and generate the entire Go codebase with minimal friction.

---

# README.md

# BitBeat — Concurrent Streaming Terminal Audio Engine

BitBeat is a high-performance, single-binary Terminal User Interface (TUI) music streamer written in Go. The application streams audio data directly from a remote Git repository/HTTP file server into a local, concurrent, double-buffered audio engine without downloading the full asset to disk.

This document serves as the absolute technical specification and implementation blueprint for an **Agentic Software Engineer**. Follow the architectural guidelines, data structures, and interface definitions exactly.

---

## 1. System Architecture & Component Topology

BitBeat operates using three entirely decoupled layers that communicate purely via channels (`chan`) and Go standard interfaces (`io.Reader`, `io.Writer`).

```
 +-----------------------------------------------------------------------+
 |                             TUI Layer (ui)                            |
 |           Bubble Tea Event Loop (Model-View-Update Engine)            |
 +-----------------------------------+-----------------------------------+
                                     |
                Tracks State Sync    |    Control Signals (Play/Pause)
                via Go Channels      v    via Go Channels
 +-----------------------------------+-----------------------------------+
 |                        Network Layer (network)                        |
 |             Chunked HTTP Stream Downloader (io.Reader)                |
 +-----------------------------------+-----------------------------------+
                                     |
                                     |  Slices of Raw Audio Bytes
                                     v  (chan []byte)
 +-----------------------------------+-----------------------------------+
 |                         Audio Layer (audio)                           |
 |               Oto v3 Engine & Linear Streaming Buffer                 |
 +-----------------------------------------------------------------------+

```

### Component Design Requirements

1. **Concurrency Isolation:** The TUI, Network Downloader, and Audio Playback loops must execute on separate Goroutines. No layer may block another layer's primary loop.
2. **Zero Disk Dependency:** Audio bytes must flow directly from the HTTP socket into memory buffers, passing through the decoder directly into the speaker stream.

---

## 2. Technical Stack & Dependencies

The agent must configure `go.mod` to use exclusively the following native Go modules. Do not introduce raw C bindings or external FFI modules.

* **TUI Framework:** `github.com/charmbracelet/bubbletea` (Elm Architecture Engine)
* **TUI Styling:** `github.com/charmbracelet/lipgloss` (ANSI Layout & Term Color)
* **Audio Core:** `github.com/ebitengine/oto/v3` (Low-level native OS PCM output)
* **Audio Decoder:** `github.com/hajimehoshi/go-mp3` (`io.Reader` wrapper for raw PCM extraction)
* **Configuration:** `github.com/pelletier/go-toml/v2` (Vim/NVChad configuration file mapping)

---

## 3. Data Structures & Interface Specifications

### 3.1 Domain Models (`internal/config/parser.go`)

```go
package config

type Config struct {
	RepositoryURL string            `toml:"repository_url"`
	DefaultFolder string            `toml:"default_folder"`
	Keybindings   KeybindingConfig  `toml:"keybindings"`
	Audio         AudioConfig       `toml:"audio"`
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
	SkipIntervalSec int `toml:"skip_interval_seconds"` // Default: 5 or 10s
	DefaultVolume   int `toml:"default_volume"`           // 0-100 scale
}

```

### 3.2 Track Metadata (`internal/network/client.go`)

```go
package network

type Track struct {
	Title    string  `json:"title"`
	Artist   string  `json:"artist"`
	Filename string  `json:"filename"`
	StreamURL string `json:"stream_url"`
	Duration float64 `json:"duration_seconds"`
}

// Client handles remote layout scanning and chunked HTTP requests
type Client struct {
	BaseURL string
}

```

### 3.3 Audio Engine Core Control (`internal/audio/player.go`)

```go
package audio

import "io"

type PlaybackStatus int

const (
	StatusStopped PlaybackStatus = iota
	StatusPlaying
	StatusPaused
)

type AudioEngine interface {
	LoadStream(stream io.Reader) error
	Play()
	Pause()
	Stop()
	SetVolume(volume float64) // Scale: 0.0 to 1.0
	Seek(seconds float64)
	GetProgress() (currentSeconds float64, totalSeconds float64)
	GetStatus() PlaybackStatus
}

```

---

## 4. Layer Implementation Instructions

### 4.1 Configuration Layer (`internal/config`)

* Implement a parser using `go-toml/v2` that attempts to read `config.toml` from the current working directory.
* If the file does not exist, initialize a default layout with hardcoded fallback bindings (`q` to quit, `space` to pause/play, `j`/`k` for navigation, `l` to fast-forward, `h` to rewind).

### 4.2 Network Streaming Layer (`internal/network`)

* The network scanner must request the directory layout from the target repository/remote host using standard HTTP `GET` operations.
* **Chunked Stream Downloader:** When a track is selected, the stream handler must make a targeted HTTP request. The resulting `response.Body` (which acts as an `io.ReadCloser`) must **not** be read fully via `io.ReadAll`.
* Instead, pipe the `response.Body` chunk-by-chunk directly into the `go-mp3` decoder.

### 4.3 Concurrent Audio Core Layer (`internal/audio`)

* Initialize Oto v3 Context cleanly using standard sample rates (typically 44100Hz, Stereo, 2 channels).
* **The Streaming Pipeline:** Connect the components using Go interfaces:
```
HTTP Socket Body (io.Reader) -> go-mp3.Decoder (io.Reader) -> Oto Player Buffer

```


* Implement a safe state mutex (`sync.Mutex`) or internal channel controller around the Play/Pause logic to prevent the audio hardware state machine from crashing when data delivery encounters network latency.
* To implement the **Fast-Forward / Rewind** function without storing the file on disk, track the number of bytes consumed by the decoder (`current_bytes / bytes_per_second`). When a seek token is issued, calculation must skip the corresponding byte count offset forward, or discard the current decoder context and reissue a HTTP `Range` request to reset the stream point if rewinding.

### 4.4 UI Display & State Loop (`internal/ui`)

* Implement the Bubble Tea `tea.Model` pattern cleanly.
* **The State (Model):** Maintain active choices list, current selection pointer (`cursor`), active song playback statistics, volume levels, and network connectivity state.
* **The Interaction Engine (Update):** Translate incoming terminal key bindings into explicit functional commands (`tea.Cmd`). Maps cursor adjustments directly to visual components.
* **Double-Buffering UI Rendering (View):** Use `lipgloss` to compute screen boundaries. The screen layout must render safely into a clean text frame string block:
* *Top Section:* Header title and configuration path state.
* *Middle Section:* Scrollable NVChad-like track selection file grid displaying available remote titles.
* *Bottom Section:* Persistent Audio Dash containing a smooth progress bar slider (`[████░░░░░░░░░░]`), timestamps (`01:24 / 04:05`), playback statuses (`[PLAYING]`/`[PAUSED]`), and active volume controls.



---

## 5. Execution Steps for the Agent Engine

To construct this tool cleanly, execute the creation sequence across these modular targets:

1. **Phase 1 - Environment Base Setup:** Parse `config.toml` variables, instantiate structural packages, and handle cross-layer directory topologies.
2. **Phase 2 - Network Verification:** Implement the network client routines to scrape file structures from remote URLs and unpack metadata objects successfully.
3. **Phase 3 - Pipeline Assembly:** Implement the concurrent data processing pipeline. Link `HTTP Body Reader` into `go-mp3.NewDecoder` and pipe chunks directly into an active `Oto` runtime context running on a separate Goroutine loop.
4. **Phase 4 - TUI State Bindings:** Enwrap the playback control loops within a responsive Bubble Tea event matrix. Block screen flicker entirely using standardized structural string maps.

---

**Agent Target Directive:** You are fully authorized to proceed with creating this system configuration. Initialize the development directory and execute `go build -o bitbeat main.go` upon feature completion to ensure absolute binary integrity.
