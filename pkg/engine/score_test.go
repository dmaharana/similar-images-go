package engine

import (
	"math"
	"testing"
)

func TestSimilarityScoreClamping(t *testing.T) {
	tests := []struct {
		name     string
		input    float32
		expected float32
	}{
		{"negative", -5.0, 0.0},
		{"greater than 100", 105.0, 100.0},
		{"nan", float32(math.NaN()), 0.0},
		{"valid mid", 82.5, 82.5},
		{"zero", 0.0, 0.0},
		{"hundred", 100.0, 100.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			score := NewSimilarityScore(tc.input)
			if score.AsPercent() != tc.expected {
				t.Fatalf("expected %f, got %f", tc.expected, score.AsPercent())
			}
		})
	}
}

func TestSimilarityScoreThreshold(t *testing.T) {
	score := NewSimilarityScore(85.0)
	if !score.MeetsThreshold(80.0) {
		t.Errorf("expected 85.0 to meet threshold 80.0")
	}
	if !score.MeetsThreshold(85.0) {
		t.Errorf("expected 85.0 to meet threshold 85.0")
	}
	if score.MeetsThreshold(90.0) {
		t.Errorf("expected 85.0 NOT to meet threshold 90.0")
	}
}

func TestSimilarityScoreString(t *testing.T) {
	score := NewSimilarityScore(94.5)
	if score.String() != "94.5%" {
		t.Errorf("expected '94.5%%', got %q", score.String())
	}
}
