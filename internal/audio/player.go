package audio

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/ebitengine/oto/v3"
)

type PlaybackStatus int

const (
	StatusStopped PlaybackStatus = iota
	StatusPlaying
	StatusPaused
)

type Decoder interface {
	io.ReadSeeker
	Length() int64
}

type AudioEngine interface {
	LoadStream(stream io.ReadCloser, filename string, url string) error
	Play()
	Pause()
	Stop()
	SetVolume(volume float64)
	GetProgress() (currentSeconds float64, totalSeconds float64)
	GetStatus() PlaybackStatus
	Close() error
}

type Engine struct {
	ctx           *oto.Context
	player        *oto.Player
	decoder       Decoder
	currentStream io.Closer
	status        PlaybackStatus
	volume        float64
	mu            sync.Mutex
	
	totalSeconds   float64
	currentSeconds float64
	
	eofReached bool
	done       chan struct{}
	currentURL string
}

func NewEngine() (*Engine, error) {
	op := &oto.NewContextOptions{
		SampleRate:   44100,
		ChannelCount: 2,
		Format:       oto.FormatSignedInt16LE,
	}
	ctx, ready, err := oto.NewContext(op)
	if err != nil {
		return nil, err
	}
	<-ready

	return &Engine{
		ctx:    ctx,
		volume: 0.5,
		status: StatusStopped,
		done:   make(chan struct{}),
	}, nil
}

func (e *Engine) IsFinished() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.eofReached && e.status == StatusPlaying
}

func (e *Engine) LoadStream(stream io.ReadCloser, filename string, url string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.decoder = nil
	if e.player != nil {
		e.player.Close()
	}
	if e.currentStream != nil {
		e.currentStream.Close()
	}

	e.eofReached = false

	ext := strings.ToLower(filename)
	isMP3 := strings.HasSuffix(ext, ".mp3")
	isM4A := strings.HasSuffix(ext, ".m4a") || strings.HasSuffix(ext, ".mp4")
	isAAC := strings.HasSuffix(ext, ".aac")
	isOpus := strings.HasSuffix(ext, ".opus")
	_ = isMP3 // Kept for explicit design reference as requested

	isTranscodedFormat := isM4A || isAAC || isOpus

	e.totalSeconds = 0
	e.currentURL = url
	go func(targetURL string) {
		f, _ := os.OpenFile("probe.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f != nil {
			_, _ = f.WriteString(fmt.Sprintf("Goroutine started for URL: %s\n", targetURL))
			f.Close()
		}
		duration, _ := probeDuration(targetURL)
		if duration > 0 {
			e.mu.Lock()
			if e.currentURL == targetURL {
				e.totalSeconds = duration
			}
			e.mu.Unlock()
		}
	}(url)

	if isTranscodedFormat {
		if !hasFFmpeg() {
			return fmt.Errorf("ffmpeg is not installed, cannot play %s", ext)
		}
		if err := e.loadTranscoded(stream, stream); err != nil {
			return err
		}
	} else if isMP3 {
		// Attempt to load raw MP3
		pr, err := e.loadRawMP3(stream)
		if err != nil {
			// Fallback to transcoding if raw MP3 decoding failed and we have FFmpeg
			if hasFFmpeg() {
				combined := io.MultiReader(&pr.buf, pr.underlying)
				if errTrans := e.loadTranscoded(combined, stream); errTrans != nil {
					return fmt.Errorf("MP3 decoding failed, fallback transcoding failed: %v", errTrans)
				}
			} else {
				return fmt.Errorf("MP3 decoding failed: %v. This file might be corrupt or use an unsupported format (like MPEG 2.5).", err)
			}
		} else {
			e.currentStream = stream
		}
	} else {
		// Unknown format, try to transcode
		if hasFFmpeg() {
			if err := e.loadTranscoded(stream, stream); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("unsupported format %s and ffmpeg is not installed", ext)
		}
	}

	e.player = e.ctx.NewPlayer(e.decoder)
	
	// Only override duration if we couldn't probe it and Length() is valid
	if e.totalSeconds <= 0 {
		e.totalSeconds = float64(e.decoder.Length()) / float64(44100*4)
	}
	
	e.currentSeconds = 0
	e.status = StatusStopped
	e.player.SetVolume(e.volume)

	return nil
}

func probeDuration(url string) (float64, error) {
	if url == "" {
		return 0, fmt.Errorf("empty URL")
	}

	// 1. Try fetching first 200KB via HTTP Range request to parse duration quickly
	req, err := http.NewRequest("GET", url, nil)
	if err == nil {
		req.Header.Set("Range", "bytes=0-200000")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent {
				cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", "-")
				cmd.Stdin = resp.Body
				out, err := cmd.Output()
				if err == nil {
					var duration float64
					fmt.Sscanf(string(out), "%f", &duration)
					if duration > 0 {
						// Write debug info to probe.log
						f, _ := os.OpenFile("probe.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
						if f != nil {
							_, _ = f.WriteString(fmt.Sprintf("URL: %s\nRange-probed Duration: %f\n\n", url, duration))
							f.Close()
						}
						return duration, nil
					}
				}
			}
		}
	}

	// 2. Fallback to full URL probing
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", url)
	out, err := cmd.Output()

	// Logging code
	f, _ := os.OpenFile("probe.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if f != nil {
		if err != nil {
			stderrMsg := ""
			if exitErr, ok := err.(*exec.ExitError); ok {
				stderrMsg = string(exitErr.Stderr)
			}
			_, _ = f.WriteString(fmt.Sprintf("URL: %s\nFallback Error: %v\nStderr: %s\n\n", url, err, stderrMsg))
		} else {
			_, _ = f.WriteString(fmt.Sprintf("URL: %s\nFallback Output: %s\n\n", url, strings.TrimSpace(string(out))))
		}
		f.Close()
	}

	if err != nil {
		return 0, err
	}
	var duration float64
	fmt.Sscanf(string(out), "%f", &duration)
	return duration, nil
}

func (e *Engine) Play() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.player != nil {
		e.player.Play()
		e.status = StatusPlaying
	}
}

func (e *Engine) Pause() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.player != nil {
		e.player.Pause()
		e.status = StatusPaused
	}
}

func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.player != nil {
		e.player.Pause()
	}
	e.status = StatusStopped
	e.eofReached = false
}

func (e *Engine) SetVolume(volume float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.volume = volume
	if e.player != nil {
		e.player.SetVolume(volume)
	}
}

func (e *Engine) GetProgress() (float64, float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.decoder == nil {
		return 0, 0
	}
	
	pos, _ := e.decoder.Seek(0, 1) // 1 = io.SeekCurrent
	curr := float64(pos) / (44100.0 * 4.0)
	return curr, e.totalSeconds
}

func (e *Engine) GetStatus() PlaybackStatus {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.status
}

func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.player != nil {
		e.player.Close()
	}
	if e.currentStream != nil {
		e.currentStream.Close()
	}
	return nil
}
