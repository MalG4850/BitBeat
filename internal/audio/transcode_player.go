package audio

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

type pcmDecoder struct {
	io.ReadCloser
	mu          sync.Mutex
	pos         int64
	totalLength int64
	engine      *Engine
}

func (p *pcmDecoder) Read(buf []byte) (int, error) {
	p.mu.Lock()
	n, err := p.ReadCloser.Read(buf)
	p.pos += int64(n)
	p.mu.Unlock()
	if err == io.EOF {
		p.engine.mu.Lock()
		p.engine.eofReached = true
		p.engine.mu.Unlock()
	}
	return n, err
}

func (p *pcmDecoder) Seek(offset int64, whence int) (int64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if whence == io.SeekCurrent {
		return p.pos, nil
	}
	if whence == io.SeekStart && offset == p.pos {
		return p.pos, nil
	}
	return 0, fmt.Errorf("seeking is not supported on live transcoded PCM streams")
}

func (p *pcmDecoder) Length() int64 {
	return p.totalLength
}

func hasFFmpeg() bool {
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		return true
	}
	for _, p := range []string{"/usr/bin/ffmpeg", "/usr/local/bin/ffmpeg", "/opt/homebrew/bin/ffmpeg"} {
		if _, err := exec.Command(p, "-version").Output(); err == nil {
			return true
		}
	}
	return false
}

func getFFmpegPath() string {
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p
	}
	for _, p := range []string{"/usr/bin/ffmpeg", "/usr/local/bin/ffmpeg", "/opt/homebrew/bin/ffmpeg"} {
		if _, err := exec.Command(p, "-version").Output(); err == nil {
			return p
		}
	}
	return "ffmpeg"
}

// loadTranscoded transcodes the input stream to raw PCM using FFmpeg on the fly.
func (e *Engine) loadTranscoded(stream io.Reader, closer io.Closer) error {
	ffmpegPath := getFFmpegPath()

	// Transcode input stream to raw s16le PCM at 44100 Hz, stereo
	cmd := exec.Command(ffmpegPath, "-i", "pipe:0", "-f", "s16le", "-acodec", "pcm_s16le", "-ar", "44100", "-ac", "2", "pipe:1")
	cmd.Stdin = stream
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create ffmpeg stdout pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %v", err)
	}

	// Verify that the transcoding starts successfully by reading the first 4 bytes of PCM
	var firstBytes [4]byte
	n, errRead := io.ReadFull(stdout, firstBytes[:])
	if errRead != nil {
		stdout.Close()
		cmd.Process.Kill()
		cmd.Wait()
		return fmt.Errorf("failed to transcode audio stream: %v (read %d bytes)", errRead, n)
	}

	// Wrap stdout to close it and kill process on Close
	pcmStream := &ffmpegCloser{
		ReadCloser: &combinedReadCloser{
			Reader: io.MultiReader(bytes.NewReader(firstBytes[:]), stdout),
			Closer: stdout,
		},
		cmd: cmd,
	}

	// Set e.decoder to our pcmDecoder
	totalLength := int64(e.totalSeconds * 44100 * 4) // 44100 samples/sec * 2 channels * 2 bytes/sample = 4 bytes/sample
	e.decoder = &pcmDecoder{
		ReadCloser:  pcmStream,
		totalLength: totalLength,
		engine:      e,
	}

	e.currentStream = &combinedCloser{closer1: pcmStream, closer2: closer}
	return nil
}

type combinedReadCloser struct {
	io.Reader
	io.Closer
}

type combinedCloser struct {
	closer1 io.Closer
	closer2 io.Closer
}

func (cc *combinedCloser) Close() error {
	err1 := cc.closer1.Close()
	err2 := cc.closer2.Close()
	if err1 != nil {
		return err1
	}
	return err2
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
