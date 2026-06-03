package media

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Transcriber turns an audio file into text. It is pluggable so different
// engines (whisper.cpp now, others later) can be swapped in.
type Transcriber interface {
	Transcribe(ctx context.Context, audioPath string) (string, error)
	Name() string
}

// WhisperCpp shells out to a whisper.cpp CLI (e.g. "whisper-cli" or "main").
// It is only constructed when a binary + model are configured; otherwise audio
// transcription is unavailable and callers must supply a transcript directly.
type WhisperCpp struct {
	Bin   string // path/name of the whisper.cpp CLI
	Model string // path to a ggml model file
}

// NewWhisperCpp returns a transcriber, or nil if not configured.
func NewWhisperCpp(bin, model string) *WhisperCpp {
	if strings.TrimSpace(bin) == "" || strings.TrimSpace(model) == "" {
		return nil
	}
	return &WhisperCpp{Bin: bin, Model: model}
}

func (w *WhisperCpp) Name() string { return "whisper.cpp" }

// Transcribe runs whisper.cpp, writing a .txt next to the audio and returning
// its contents. whisper.cpp expects 16kHz mono WAV; conversion (e.g. via ffmpeg)
// is the caller's responsibility.
func (w *WhisperCpp) Transcribe(ctx context.Context, audioPath string) (string, error) {
	outBase := strings.TrimSuffix(audioPath, filepath.Ext(audioPath))
	// -otxt writes <outBase>.txt; -nt suppresses timestamps in the text output.
	cmd := exec.CommandContext(ctx, w.Bin, "-m", w.Model, "-f", audioPath, "-otxt", "-nt", "-of", outBase)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("whisper.cpp failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	data, err := os.ReadFile(outBase + ".txt")
	if err != nil {
		return "", fmt.Errorf("read whisper output: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}
