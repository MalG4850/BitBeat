package audio

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/ebitengine/oto/v3"
	"github.com/hajimehoshi/go-mp3"
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

// mp3DecoderWrap wraps mp3.Decoder to match our Decoder interface
type mp3DecoderWrap struct {
	*mp3.Decoder
	closer io.Closer
	engine *Engine
}

func (m *mp3DecoderWrap) Length() int64 {
	return m.Decoder.Length()
}

func (m *mp3DecoderWrap) Close() error {
	if m.closer != nil {
		m.closer.Close()
	}
	return nil
}

func (m *mp3DecoderWrap) Read(p []byte) (n int, err error) {
	n, err = m.Decoder.Read(p)
	if err == io.EOF {
		m.engine.mu.Lock()
		m.engine.eofReached = true
		m.engine.mu.Unlock()
	}
	return n, err
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
	currentStream io.ReadCloser
	status        PlaybackStatus
	volume        float64
	mu            sync.Mutex
	
	totalSeconds   float64
	currentSeconds float64
	
	eofReached bool
	done       chan struct{}
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

	if e.player != nil {
		e.player.Close()
	}
	if e.currentStream != nil {
		e.currentStream.Close()
	}

	e.eofReached = false

	var finalStream io.ReadCloser = stream

	ext := strings.ToLower(filename)
	isM4A := strings.HasSuffix(ext, ".m4a") || strings.HasSuffix(ext, ".aac") || strings.HasSuffix(ext, ".mp4")

	// Try to get duration using ffprobe
	duration, _ := probeDuration(url)
	e.totalSeconds = duration

	if isM4A {
		// Use FFmpeg to transcode M4A to MP3 on the fly
		cmd := exec.Command("ffmpeg", "-i", "pipe:0", "-f", "mp3", "pipe:1")
		cmd.Stdin = stream
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("failed to create ffmpeg pipe: %v", err)
		}
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start ffmpeg: %v. Is ffmpeg installed?", err)
		}
		
		// Wrap stdout to ensure we can close it and wait for cmd
		finalStream = &ffmpegCloser{ReadCloser: stdout, cmd: cmd}
	}

	d, err := mp3.NewDecoder(finalStream)
	if err != nil {
		if !isM4A {
			return fmt.Errorf("MP3 decoding failed: %v. This file might be corrupt or use an unsupported bitrate.", err)
		}
		return err
	}
	
	e.decoder = &mp3DecoderWrap{Decoder: d, closer: finalStream, engine: e}
	e.player = e.ctx.NewPlayer(e.decoder)
	e.currentStream = finalStream
	
	// Only override duration if we couldn't probe it and Length() is valid
	if e.totalSeconds <= 0 {
		e.totalSeconds = float64(d.Length()) / float64(44100*4)
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
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", url)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	var duration float64
	fmt.Sscanf(string(out), "%f", &duration)
	return duration, nil
}

type ffmpegCloser struct {
	io.ReadCloser
	cmd *exec.Cmd
}

func (f *ffmpegCloser) Close() error {
	err := f.ReadCloser.Close()
	if f.cmd.Process != nil {
		f.cmd.Process.Kill()
	}
	f.cmd.Wait()
	return err
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
		e.status = StatusStopped
	}
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
