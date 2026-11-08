package engine

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"image"
	"math"
)

const (
	PaletteBinsPerChannel = 4
	PaletteTotalBins      = PaletteBinsPerChannel * PaletteBinsPerChannel * PaletteBinsPerChannel // 64 bins
)

// PaletteFingerprint holds a 64-bin normalized color histogram.
type PaletteFingerprint struct {
	Bins [PaletteTotalBins]float32
}

func (f PaletteFingerprint) Bytes() []byte {
	buf := make([]byte, PaletteTotalBins*4)
	for i, b := range f.Bins {
		binary.LittleEndian.PutUint32(buf[i*4:(i+1)*4], math.Float32bits(b))
	}
	return buf
}

func (f PaletteFingerprint) String() string {
	return hex.EncodeToString(f.Bytes())
}

// PaletteEngine computes color histogram intersection over a 64-bin 3D RGB color space.
type PaletteEngine struct{}

func NewPaletteEngine() *PaletteEngine {
	return &PaletteEngine{}
}

func (e *PaletteEngine) Metric() Metric {
	return MetricPalette
}

func (e *PaletteEngine) Compute(img image.Image) Fingerprint {
	rgb := ResizeRGB(img, 64, 64)
	var counts [PaletteTotalBins]uint32
	totalPixels := len(rgb) / 3

	for i := 0; i < totalPixels; i++ {
		r := rgb[i*3]
		g := rgb[i*3+1]
		b := rgb[i*3+2]

		rBin := (int(r) * PaletteBinsPerChannel) / 256
		gBin := (int(g) * PaletteBinsPerChannel) / 256
		bBin := (int(b) * PaletteBinsPerChannel) / 256

		binIdx := (rBin * PaletteBinsPerChannel * PaletteBinsPerChannel) +
			(gBin * PaletteBinsPerChannel) +
			bBin

		counts[binIdx]++
	}

	var bins [PaletteTotalBins]float32
	if totalPixels > 0 {
		for i := 0; i < PaletteTotalBins; i++ {
			bins[i] = float32(counts[i]) / float32(totalPixels)
		}
	}

	return PaletteFingerprint{Bins: bins}
}

func (e *PaletteEngine) DecodeFingerprint(data []byte) (Fingerprint, error) {
	if len(data) != PaletteTotalBins*4 {
		return nil, errors.New("invalid palette fingerprint length: expected 256 bytes")
	}
	var bins [PaletteTotalBins]float32
	for i := 0; i < PaletteTotalBins; i++ {
		bits := binary.LittleEndian.Uint32(data[i*4 : (i+1)*4])
		bins[i] = math.Float32frombits(bits)
	}
	return PaletteFingerprint{Bins: bins}, nil
}

func (e *PaletteEngine) Similarity(f1, f2 Fingerprint) SimilarityScore {
	pf1, ok1 := f1.(PaletteFingerprint)
	pf2, ok2 := f2.(PaletteFingerprint)
	if !ok1 || !ok2 {
		return NewSimilarityScore(0)
	}

	var intersection float32
	for i := 0; i < PaletteTotalBins; i++ {
		v1 := pf1.Bins[i]
		v2 := pf2.Bins[i]
		if v1 < v2 {
			intersection += v1
		} else {
			intersection += v2
		}
	}

	score := intersection * 100.0
	return NewSimilarityScore(score)
}
