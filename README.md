# BitBeat - Terminal Audio Engine

BitBeat is a high-performance, terminal-based audio player designed for seamless streaming and playback from remote repositories and streaming services. Built with Go and powered by the Ebitengine audio stack, BitBeat combines a modern TUI (Terminal User Interface) with a robust background transcoding engine.

---

## 🚀 Features

- **🌐 Dynamic Repository Streaming:** Parse and browse file structures directly from Codeberg or other Git-based repository hosts.
- **🎵 SoundCloud Support:** Play individual tracks or entire playlists by simply pasting a SoundCloud URL.
- **🏗️ Smart Architecture:** Automatically detects file types and utilizes FFmpeg for real-time transcoding of complex formats (M4A, AAC, VBR MP3).
- **📂 Hierarchical Navigation:** Full directory support with recursive browsing, making it easy to navigate large music collections.
- **📊 Precise Progress Tracking:** Real-time progress bars and duration probing using `ffprobe` for accurate `xx:xx / xx:xx` timing.
- **🎨 Polished TUI:** Built with the Charmbracelet `bubbletea` and `lipgloss` ecosystem for a beautiful, responsive terminal experience.

---

## 🏗️ Architecture

BitBeat is organized into several internal modules that work together to provide a smooth audio experience:

### 1. **UI Layer (`internal/ui`)**
- **Framework:** Uses `bubbletea` for state management and `lipgloss` for styling.
- **State Machine:** Manages different application states such as `stateInputting` (for link entry) and `stateBrowsing` (for track selection).
- **Responsiveness:** Automatically handles terminal window resizing to ensure the UI remains centered and clear.

### 2. **Network Client (`internal/network`)**
- **Multi-Protocol Parsing:** Detects the host (Codeberg, SoundCloud, etc.) and uses the appropriate API or parser.
- **Codeberg API:** Uses the Codeberg API to recursively fetch repository contents, distinguishing between folders and audio files.
- **SoundCloud Integration:** Utilizes a dedicated SoundCloud client to resolve permalinks into streamable progressive MP3 or HLS URLs.

### 3. **Audio Engine (`internal/audio`)**
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
   git clone https://github.com/your-username/BitBeat.git
   cd BitBeat
   ```

2. **Install dependencies:**
   ```bash
   go mod tidy
   ```

3. **Build the application:**
   ```bash
   go build -o bitbeat main.go
   ```

---

## 🎮 Usage

### Starting the Application
Run the binary:
```bash
./bitbeat
```

### Link Input
Upon startup, the application will ask for a repository or SoundCloud link:
- **Codeberg:** `https://codeberg.org/User/Repo`
- **SoundCloud Track:** `https://soundcloud.com/artist/track-name`
- **SoundCloud Playlist:** `https://soundcloud.com/artist/sets/playlist-name`

### Keybindings
| Key | Action |
| :--- | :--- |
| `Enter` | Open folder / Play selected track |
| `Backspace` | Go up one directory |
| `l` | Input a new link |
| `q` / `Ctrl+C` | Quit the application |
| `Space` | Play / Pause |
| `Up` / `k` | Move cursor up |
| `Down` / `j` | Move cursor down |

---

## 🔧 Technical Details: How it Works

### The FFmpeg Pipe
When an M4A file is selected, BitBeat executes:
```bash
ffmpeg -i pipe:0 -f mp3 pipe:1
```
It streams the remote file into `stdin` and reads the transcoded MP3 output from `stdout`. This allows for "instant-on" playback without needing to store large temporary files.

### The Duration Probe
To avoid the `00:00 / 00:00` problem, BitBeat runs:
```bash
ffprobe -v error -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 <URL>
```
This is done asynchronously before playback begins, ensuring the user always sees the correct track length.

---

## 🤝 Contributing
Contributions are welcome! Please feel free to submit a Pull Request.

## 📄 License
This project is licensed under the GPLv3 License - see the LICENSE file for details.
