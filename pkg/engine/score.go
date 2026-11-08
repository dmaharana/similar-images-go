package engine

import (
	"math"
)

// SimilarityScore represents a normalized visual closeness score bounded between 0.0% and 100.0%.
type SimilarityScore struct {
	val float32
}

// NewSimilarityScore creates a new score clamped to [0.0, 100.0].
func NewSimilarityScore(v float32) SimilarityScore {
	if math.IsNaN(float64(v)) {
		return SimilarityScore{val: 0.0}
	}
	if v < 0.0 {
		return SimilarityScore{val: 0.0}
	}
	if v > 100.0 {
		return SimilarityScore{val: 100.0}
	}
	return SimilarityScore{val: v}
}

// AsPercent returns the raw percentage (0.0 to 100.0).
func (s SimilarityScore) AsPercent() float32 {
	return s.val
}

// MeetsThreshold checks if this score satisfies the cutoff threshold percentage.
func (s SimilarityScore) MeetsThreshold(threshold float32) bool {
	return s.val >= threshold
}

// String formats the score as a percentage string (e.g., "94.5%").
func (s SimilarityScore) String() string {
	return formatPercent(s.val)
}

func formatPercent(v float32) string {
	return sprintf1(v)
}

func sprintf1(v float32) string {
	// Simple formatted string without heavy reflection
	intPart := int(v)
	decPart := int(math.Round(float64(v-float32(intPart)) * 10))
	if decPart >= 10 {
		intPart++
		decPart = 0
	}
	return itoa(intPart) + "." + itoa(decPart) + "%"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
