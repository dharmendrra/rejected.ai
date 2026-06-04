package media

import "github.com/dharmendra/rejected.ai/internal/domain"

// AnalyzeVideo turns raw per-frame counts into measurable video signals. Every
// returned value is a direct share of analyzed frames (or of clip duration) —
// no trait is inferred. Percentages are clamped to [0,100]; when no frames were
// analyzed (or duration is 0) the corresponding percentage is 0.
func AnalyzeVideo(m domain.FrameMetrics) (facePresentPct, gazeOnScreenPct, onCameraPct, multiFacePct float64) {
	if m.FramesAnalyzed > 0 {
		f := float64(m.FramesAnalyzed)
		facePresentPct = pct(float64(m.FramesFacePresent), f)
		gazeOnScreenPct = pct(float64(m.FramesGazeOnScreen), f)
		multiFacePct = pct(float64(m.FramesMultiFace), f)
	}
	if m.DurationSec > 0 {
		onCameraPct = pct(m.OnCameraSec, m.DurationSec)
	}
	return
}

// pct returns part/whole as a percentage, clamped to [0,100]. whole is assumed
// > 0 by callers; values above the whole (a malformed count) clamp to 100.
func pct(part, whole float64) float64 {
	v := part / whole * 100.0
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
