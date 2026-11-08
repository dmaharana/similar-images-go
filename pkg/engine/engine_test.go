package engine

import (
	"image"
	"image/color"
	"testing"
)

func createTestImage(w, h int, pattern string) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			switch pattern {
			case "gradient":
				c := uint8((x + y) * 255 / (w + h))
				img.Set(x, y, color.RGBA{R: c, G: c, B: c, A: 255})
			case "inv_gradient":
				c := uint8(255 - ((x + y) * 255 / (w + h)))
				img.Set(x, y, color.RGBA{R: c, G: c, B: c, A: 255})
			case "red":
				img.Set(x, y, color.RGBA{R: 240, G: 20, B: 20, A: 255})
			case "blue":
				img.Set(x, y, color.RGBA{R: 20, G: 20, B: 240, A: 255})
			case "checkerboard":
				if (x/8+y/8)%2 == 0 {
					img.Set(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
				} else {
					img.Set(x, y, color.RGBA{R: 0, G: 0, B: 0, A: 255})
				}
			}
		}
	}
	return img
}

func TestEngineSelfSimilarityAndSerialization(t *testing.T) {
	metrics := SupportedMetrics()
	testImg := createTestImage(100, 100, "gradient")

	for _, m := range metrics {
		t.Run(string(m), func(t *testing.T) {
			eng, err := GetEngine(m)
			if err != nil {
				t.Fatalf("failed to get engine for %s: %v", m, err)
			}

			fp := eng.Compute(testImg)
			if fp == nil {
				t.Fatalf("nil fingerprint returned for %s", m)
			}

			// Self similarity must be 100%
			score := eng.Similarity(fp, fp)
			if score.AsPercent() < 99.9 {
				t.Fatalf("%s self similarity expected 100%%, got %f", m, score.AsPercent())
			}

			// Serialization roundtrip
			b := fp.Bytes()
			decoded, err := eng.DecodeFingerprint(b)
			if err != nil {
				t.Fatalf("%s failed to decode bytes: %v", m, err)
			}

			scoreRoundtrip := eng.Similarity(fp, decoded)
			if scoreRoundtrip.AsPercent() < 99.9 {
				t.Fatalf("%s roundtrip decoded similarity expected 100%%, got %f", m, scoreRoundtrip.AsPercent())
			}
		})
	}
}

func TestMetricParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected Metric
	}{
		{"phash", MetricPHash},
		{"PHASH", MetricPHash},
		{"phash-dct", MetricPHash},
		{"pdq", MetricPDQ},
		{"wavelet", MetricWavelet},
		{"whash", MetricWavelet},
		{"dhash", MetricDHash},
		{"perceptual", MetricDHash},
		{"palette", MetricPalette},
		{"color", MetricPalette},
	}

	for _, tc := range tests {
		m, err := ParseMetric(tc.input)
		if err != nil {
			t.Errorf("failed to parse %q: %v", tc.input, err)
		}
		if m != tc.expected {
			t.Errorf("parse %q: expected %s, got %s", tc.input, tc.expected, m)
		}
	}

	_, err := ParseMetric("unknown-metric")
	if err == nil {
		t.Errorf("expected error parsing unknown metric, got nil")
	}
}

func TestImageDiscrimination(t *testing.T) {
	img1 := createTestImage(128, 128, "red")
	img2 := createTestImage(128, 128, "blue")

	palEng := NewPaletteEngine()
	fp1 := palEng.Compute(img1)
	fp2 := palEng.Compute(img2)
	score := palEng.Similarity(fp1, fp2)
	if score.AsPercent() > 20.0 {
		t.Fatalf("red vs blue palette similarity should be very low, got %f", score.AsPercent())
	}
}
