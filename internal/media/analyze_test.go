package media

import "testing"

func TestAnalyze_Measurable(t *testing.T) {
	// 12 words over 6 seconds -> 120 wpm. Fillers: "um", "you know", "like".
	text := "Um, so I built the service, you know, like a real one."
	wc, wpm, fillerTotal, fillerRate, fillers := Analyze(text, 6.0)

	if wc != 12 {
		t.Errorf("word count = %d, want 12", wc)
	}
	if wpm < 119 || wpm > 121 {
		t.Errorf("wpm = %.1f, want ~120", wpm)
	}
	// "um"(1) + "so"(1) + "you know"(1) + "like"(1) = 4
	if fillerTotal != 4 {
		t.Errorf("filler total = %d, want 4 (got %v)", fillerTotal, fillers)
	}
	if fillerRate < 33.0 || fillerRate > 34.0 {
		t.Errorf("filler rate = %.2f, want ~33.33 per 100 words", fillerRate)
	}
}

func TestAnalyze_NoDurationNoFillers(t *testing.T) {
	wc, wpm, fillerTotal, fillerRate, _ := Analyze("clean concise technical answer", 0)
	if wc != 4 {
		t.Errorf("word count = %d, want 4", wc)
	}
	if wpm != 0 {
		t.Errorf("wpm = %.1f, want 0 when duration is 0", wpm)
	}
	if fillerTotal != 0 || fillerRate != 0 {
		t.Errorf("expected no fillers, got total=%d rate=%.2f", fillerTotal, fillerRate)
	}
}

func TestAnalyze_FillerWordBoundary(t *testing.T) {
	// "summary" contains "um" and "likely" contains "like" — must NOT count.
	_, _, fillerTotal, _, fillers := Analyze("In summary this is likely correct", 0)
	if fillerTotal != 0 {
		t.Errorf("word-boundary leak: total=%d fillers=%v", fillerTotal, fillers)
	}
}
