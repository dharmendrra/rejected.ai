// Package signals computes measurable communication signals. It deliberately
// avoids pseudoscience: only directly countable quantities are used.
package signals

import "strings"

// CompressionRatio measures information density: the number of distinct evidence
// items a candidate conveyed per word of answer. High-signal short answers score
// higher than long, low-signal ones — the platform rewards density rather than
// length. Returns 0 for empty answers.
func CompressionRatio(answer string, evidenceCount int) float64 {
	words := len(strings.Fields(answer))
	if words == 0 {
		return 0
	}
	return float64(evidenceCount) / float64(words)
}
