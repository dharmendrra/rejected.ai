package media

import (
	"testing"

	"github.com/dharmendra/rejected.ai/internal/domain"
)

func TestAnalyzeVideo_Measurable(t *testing.T) {
	// 1000 frames: 950 face, 800 gaze-on-screen, 20 multi-face.
	// 58s on camera over a 60s clip.
	m := domain.FrameMetrics{
		FramesAnalyzed:     1000,
		FramesFacePresent:  950,
		FramesGazeOnScreen: 800,
		FramesMultiFace:    20,
		OnCameraSec:        58.0,
		DurationSec:        60.0,
	}
	face, gaze, onCam, multi := AnalyzeVideo(m)

	if face != 95.0 {
		t.Errorf("face_present_pct = %.2f, want 95", face)
	}
	if gaze != 80.0 {
		t.Errorf("gaze_on_screen_pct = %.2f, want 80", gaze)
	}
	if multi != 2.0 {
		t.Errorf("multi_face_pct = %.2f, want 2", multi)
	}
	if onCam < 96.6 || onCam > 96.7 {
		t.Errorf("on_camera_pct = %.2f, want ~96.67", onCam)
	}
}

func TestAnalyzeVideo_NoFramesNoDuration(t *testing.T) {
	face, gaze, onCam, multi := AnalyzeVideo(domain.FrameMetrics{})
	if face != 0 || gaze != 0 || onCam != 0 || multi != 0 {
		t.Errorf("expected all zero with no frames/duration, got %.2f %.2f %.2f %.2f", face, gaze, onCam, multi)
	}
}

func TestAnalyzeVideo_ClampsMalformedCounts(t *testing.T) {
	// More "present" frames than analyzed, and more on-camera time than duration:
	// both are malformed inputs and must clamp to 100, never exceed it.
	m := domain.FrameMetrics{
		FramesAnalyzed:    100,
		FramesFacePresent: 150,
		OnCameraSec:       120,
		DurationSec:       60,
	}
	face, _, onCam, _ := AnalyzeVideo(m)
	if face != 100 {
		t.Errorf("face_present_pct = %.2f, want clamped to 100", face)
	}
	if onCam != 100 {
		t.Errorf("on_camera_pct = %.2f, want clamped to 100", onCam)
	}
}
