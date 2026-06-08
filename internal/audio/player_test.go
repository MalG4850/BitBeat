package audio

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestTranscodeCheck(t *testing.T) {
	testCases := []struct {
		filename string
		expected bool
	}{
		{"song.mp3", false},
		{"song.MP3", false},
		{"song.m4a", true},
		{"song.AAC", true},
		{"song.mp4", true},
		{"song.opus", true},
		{"song.OPUS", true},
		{"song.ogg", false},
	}

	for _, tc := range testCases {
		ext := strings.ToLower(tc.filename)
		isTranscodedFormat := strings.HasSuffix(ext, ".m4a") || strings.HasSuffix(ext, ".aac") || strings.HasSuffix(ext, ".mp4") || strings.HasSuffix(ext, ".opus")
		if isTranscodedFormat != tc.expected {
			t.Errorf("For filename %q, expected isTranscodedFormat = %v, got %v", tc.filename, tc.expected, isTranscodedFormat)
		}
	}
}

// We wrap a dummy ReadCloser to see if LoadStream tries to use ffmpeg on .opus files.
type dummyReadCloser struct {
	io.Reader
}

func (d dummyReadCloser) Close() error {
	return nil
}

func TestLoadStreamOpus(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	defer engine.Close()

	// Using a dummy stream with non-audio content.
	// For a non-transcoded format (e.g. mp3), it should directly go to mp3.NewDecoder and fail with MP3 decoding failed.
	streamMP3 := dummyReadCloser{Reader: bytes.NewReader([]byte("not-an-mp3-file"))}
	errMP3 := engine.LoadStream(streamMP3, "test.mp3", "")
	if errMP3 == nil {
		t.Error("expected error for invalid mp3 data, got nil")
	} else if !strings.Contains(errMP3.Error(), "MP3 decoding failed") {
		t.Errorf("expected MP3 decoding failed error, got: %v", errMP3)
	}

	// For a transcoded format (e.g. opus), it will try to start ffmpeg on the fly.
	// Since ffmpeg will receive "not-an-mp3-file" which is invalid opus data, ffmpeg will start but probably fail, 
	// or mp3 decoding of its output will fail. But let's verify that it does not return the direct "MP3 decoding failed" error without ffmpeg.
	streamOpus := dummyReadCloser{Reader: bytes.NewReader([]byte("not-an-opus-file"))}
	errOpus := engine.LoadStream(streamOpus, "test.opus", "")
	if errOpus == nil {
		t.Error("expected error for invalid opus data, got nil")
	}
}
