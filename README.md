# BitBeat - Terminal Audio Engine

BitBeat is a high-performance, terminal-based audio player designed for seamless streaming and playback from remote repositories and streaming services. Built with Go and powered by the Ebitengine audio stack, BitBeat combines a modern TUI (Terminal User Interface) with a robust background transcoding engine.

---

## 🚀 Features

- **🏛️ Archive.org Integration:** Stream public domain music collections, albums, and items directly via Archive.org metadata endpoints.
- **🌐 Dynamic Repository Streaming:** Parse and browse file structures directly from Codeberg or other Git-based repository hosts.
- **🎵 SoundCloud Support:** Play individual tracks or entire playlists by simply pasting a SoundCloud URL.
- **💾 Saved Entries Management:** Save favorite URLs with custom titles locally into `saved_entries.json` and delete deprecated entries on the fly.
- **🏗️ Smart Architecture:** Automatically detects file types and utilizes FFmpeg for real-time transcoding of complex formats (M4A, AAC, VBR MP3).
- **📂 Hierarchical Navigation:** Full directory support with recursive browsing, making it easy to navigate large music collections.
- **📊 Precise Progress Tracking:** Real-time progress bars and duration probing using `ffprobe` for accurate `xx:xx / xx:xx` timing.
- **🎨 Polished TUI:** Built with the Charmbracelet `bubbletea` and `lipgloss` ecosystem for a beautiful, responsive terminal experience.

---
## 🏗️ Architecture

BitBeat is organized into several internal modules that work together to provide a smooth audio experience:

### 1. **UI Layer (`internal/ui`)**
- **Framework:** Uses `bubbletea` for state management and `lipgloss` for styling.
- **State Machine:** Manages application states including `stateMainMenu`, `stateSavedEntries`, `stateAddEntry`, `stateInputting` (for link entry), and `stateBrowsing` (for track selection).
- **Responsiveness:** Automatically handles terminal window resizing to ensure the UI remains centered and clear.

### 2. **Network Client (`internal/network`)**
- **Multi-Protocol Parsing:** Detects the host (Codeberg, Archive.org, SoundCloud, etc.) and uses the appropriate API or parser.
- **Archive.org Scanner:** Parses Archive.org item IDs and metadata endpoints to locate `.mp3` and `.m4a` files for direct byte streaming.
- **Codeberg API:** Uses the Codeberg API to recursively fetch repository contents, distinguishing between folders and audio files.
- **SoundCloud Integration:** Utilizes a dedicated SoundCloud client to resolve permalinks into streamable progressive MP3 or HLS URLs.

### 3. **Saved Entries Persistence (`internal/saved`)**
- **Local Storage:** Stores saved titles and URLs in a clean `saved_entries.json` structure.
- **Entry Management:** Supports loading, saving, and deleting entries natively via TUI keybindings.

### 4. **Audio Engine (`internal/audio`)**
- **Core:** Built on `oto v3` for low-level audio device access.
- **MP3 Decoding:** Uses `go-mp3` for high-fidelity, pure-Go decoding of MP3 streams.
- **FFmpeg Bridge:** For formats like M4A or AAC, the engine spawns a background FFmpeg process to transcode the stream into a standard format in real-time.
- **Duration Probing:** Uses `ffprobe` to scan remote file metadata upfront, providing accurate total duration even for streamed pipes.

---

## 🛠️ Requirements

- **Go:** 1.26 or higher.
- **FFmpeg:** Required for M4A/AAC playback and duration probing. Ensure `ffmpeg` and `ffprobe` are in your system's PATH.
- **Terminal:** A terminal that supports ANSI escape codes (most modern terminals).

---

## 📥 Installation

1. **Clone the repository:**
   ```bash
   git clone [https://github.com/your-username/BitBeat.git](https://github.com/your-username/BitBeat.git)
   cd BitBeat
   
2. **Build the application (Offline / Vendored):**
   The project dependencies are vendored directly in the `vendor/` folder, meaning you do not need an active internet connection to download dependencies on a fresh machine.
   
   Build the binary into your `builds/` directory:
   ```bash
   go build -mod=vendor -o builds/bitbeat main.go
