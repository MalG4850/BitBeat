package audio

import (
	"bytes"
	"fmt"
	"io"
	"github.com/hajimehoshi/go-mp3"
)

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

type peekReader struct {
	underlying io.Reader
	buf        bytes.Buffer
	buffering  bool
}

func (pr *peekReader) Read(p []byte) (n int, err error) {
	n, err = pr.underlying.Read(p)
	if n > 0 && pr.buffering {
		pr.buf.Write(p[:n])
	}
	return n, err
}

// loadRawMP3 attempts to decode the stream directly using go-mp3.
// If it succeeds and is 44100 Hz, it sets the engine decoder and returns nil.
// If it fails, it returns the error and the buffered peek reader so the caller can reuse it.
func (e *Engine) loadRawMP3(stream io.ReadCloser) (*peekReader, error) {
	pr := &peekReader{
		underlying: stream,
		buffering:  true,
	}
	mp3Dec, err := mp3.NewDecoder(pr)
	if err != nil {
		return pr, err
	}

	if mp3Dec.SampleRate() != 44100 {
		return pr, fmt.Errorf("unsupported sample rate %d Hz (want 44100)", mp3Dec.SampleRate())
	}

	// It's a valid 44100 Hz MP3.
	// We re-create the decoder on a fresh combined reader to ensure that no buffered bytes
	// are lost or skipped during playback.
	combined := io.MultiReader(&pr.buf, pr.underlying)
	freshDec, errFresh := mp3.NewDecoder(combined)
	if errFresh != nil {
		return pr, errFresh
	}

	e.decoder = &mp3DecoderWrap{Decoder: freshDec, closer: stream, engine: e}
	return nil, nil
}
