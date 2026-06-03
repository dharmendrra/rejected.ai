// Package media handles audio (and later video) processing for interviews. It
// computes only MEASURABLE signals from a transcript and audio duration —
// speaking pace, filler-word statistics, word count, and (when provided)
// response latency. It deliberately does NOT infer honesty, intelligence,
// personality, or any other unmeasurable trait.
package media

import (
	"regexp"
	"sort"
	"strings"

	"github.com/dharmendra/rejected.ai/internal/domain"
)

// fillerPatterns are common verbal fillers. Multi-word phrases are matched as
// phrases. All matching is case-insensitive and word-boundary aware.
var fillerPatterns = []string{
	"um", "uh", "er", "erm", "ah", "hmm", "mm",
	"like", "basically", "actually", "literally", "honestly",
	"right", "okay", "so",
	"you know", "i mean", "sort of", "kind of", "kinda", "i guess",
}

var wordRe = regexp.MustCompile(`[A-Za-z']+`)

// Analyze computes measurable speech signals from a transcript and the audio
// duration (seconds). durationSec <= 0 yields wpm = 0.
func Analyze(text string, durationSec float64) (wordCount int, wpm float64, fillerTotal int, fillerRate float64, fillers []domain.FillerStat) {
	words := wordRe.FindAllString(text, -1)
	wordCount = len(words)

	if durationSec > 0 {
		wpm = float64(wordCount) / durationSec * 60.0
	}

	lower := strings.ToLower(text)
	for _, f := range fillerPatterns {
		n := countOccurrences(lower, f)
		if n > 0 {
			fillers = append(fillers, domain.FillerStat{Word: f, Count: n})
			fillerTotal += n
		}
	}
	// Stable, count-desc ordering.
	sort.SliceStable(fillers, func(i, j int) bool { return fillers[i].Count > fillers[j].Count })

	if wordCount > 0 {
		fillerRate = float64(fillerTotal) / float64(wordCount) * 100.0
	}
	return
}

// countOccurrences counts word-boundary-aware, case-insensitive occurrences of
// phrase in lowered text.
func countOccurrences(loweredText, phrase string) int {
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(phrase) + `\b`)
	return len(re.FindAllStringIndex(loweredText, -1))
}
