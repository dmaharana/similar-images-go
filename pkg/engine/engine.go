package engine

import (
	"fmt"
	"image"
	"strings"
)

// Metric identifies an image similarity algorithm.
type Metric string

const (
	MetricPHash    Metric = "phash"
	MetricPDQ      Metric = "pdq"
	MetricWavelet  Metric = "wavelet"
	MetricDHash    Metric = "dhash"
	MetricPalette  Metric = "palette"

	// MetricPerceptual is an alias for dhash (backward compatibility with Rust tool)
	MetricPerceptual Metric = "perceptual"
)

// Canonical returns the canonical name for this metric.
func (m Metric) Canonical() Metric {
	if m == MetricPerceptual {
		return MetricDHash
	}
	return m
}

// String returns the string representation.
func (m Metric) String() string {
	return string(m.Canonical())
}

// ParseMetric parses a string into a known Metric.
func ParseMetric(s string) (Metric, error) {
	norm := strings.ToLower(strings.TrimSpace(s))
	switch Metric(norm) {
	case MetricPHash, "phash-dct", "dct":
		return MetricPHash, nil
	case MetricPDQ, "pdq-hash":
		return MetricPDQ, nil
	case MetricWavelet, "whash", "haar":
		return MetricWavelet, nil
	case MetricDHash, "dhash-2d":
		return MetricDHash, nil
	case MetricPerceptual:
		return MetricDHash, nil
	case MetricPalette, "color", "histogram":
		return MetricPalette, nil
	default:
		return "", fmt.Errorf("unknown similarity metric: %q (supported: phash, pdq, wavelet, dhash/perceptual, palette)", s)
	}
}

// SupportedMetrics returns all canonical supported metrics.
func SupportedMetrics() []Metric {
	return []Metric{
		MetricPHash,
		MetricPDQ,
		MetricWavelet,
		MetricDHash,
		MetricPalette,
	}
}

// Fingerprint is an extracted numerical representation of an image.
type Fingerprint interface {
	Bytes() []byte
	String() string
}

// Engine calculates visual fingerprints and similarity scores.
type Engine interface {
	Metric() Metric
	Compute(img image.Image) Fingerprint
	DecodeFingerprint(data []byte) (Fingerprint, error)
	Similarity(f1, f2 Fingerprint) SimilarityScore
}

// GetEngine returns the engine implementation for the requested metric.
func GetEngine(m Metric) (Engine, error) {
	canonical := m.Canonical()
	switch canonical {
	case MetricPHash:
		return NewPHashEngine(), nil
	case MetricPDQ:
		return NewPDQEngine(), nil
	case MetricWavelet:
		return NewWaveletEngine(), nil
	case MetricDHash:
		return NewDHashEngine(), nil
	case MetricPalette:
		return NewPaletteEngine(), nil
	default:
		return nil, fmt.Errorf("unsupported metric engine: %s", m)
	}
}
