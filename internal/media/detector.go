package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/dharmendra/rejected.ai/internal/domain"
)

// VideoDetector turns a video file into raw per-frame metrics. It is pluggable
// so different engines can be swapped in. We deliberately do NOT bundle a
// computer-vision stack: the detector only counts observable frames (face
// present, gaze on screen, etc.) and never infers traits.
type VideoDetector interface {
	Detect(ctx context.Context, videoPath string) (domain.FrameMetrics, error)
	Name() string
}

// ExternalDetector shells out to a user-supplied CLI that inspects a video and
// prints FrameMetrics as JSON to stdout. This mirrors the whisper.cpp audio path:
// the heavy/optional dependency lives outside the Go binary, and is only used
// when configured. Invocation: `<bin> -i <videoPath> [-m <model>]`.
type ExternalDetector struct {
	Bin   string // path/name of the detector CLI
	Model string // optional model path passed as -m
}

// NewExternalDetector returns a detector, or nil if no binary is configured.
func NewExternalDetector(bin, model string) *ExternalDetector {
	if strings.TrimSpace(bin) == "" {
		return nil
	}
	return &ExternalDetector{Bin: bin, Model: model}
}

func (d *ExternalDetector) Name() string { return "external" }

// Detect runs the detector CLI and parses its JSON FrameMetrics from stdout.
func (d *ExternalDetector) Detect(ctx context.Context, videoPath string) (domain.FrameMetrics, error) {
	args := []string{"-i", videoPath}
	if strings.TrimSpace(d.Model) != "" {
		args = append(args, "-m", d.Model)
	}
	cmd := exec.CommandContext(ctx, d.Bin, args...)
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if ee, ok := err.(*exec.ExitError); ok {
			msg = strings.TrimSpace(string(ee.Stderr))
		}
		return domain.FrameMetrics{}, fmt.Errorf("video detector failed: %v: %s", err, msg)
	}
	var m domain.FrameMetrics
	if err := json.Unmarshal(out, &m); err != nil {
		return domain.FrameMetrics{}, fmt.Errorf("parse detector output as FrameMetrics JSON: %w", err)
	}
	return m, nil
}
