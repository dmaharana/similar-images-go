package engine

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"math/bits"
)

// DHashFingerprint holds 128-bit dual-gradient difference fingerprint (horizontal 64-bit + vertical 64-bit).
type DHashFingerprint struct {
	H uint64
	V uint64
}

func (f DHashFingerprint) Bytes() []byte {
	buf := make([]byte, 16)
	binary.BigEndian.PutUint64(buf[0:8], f.H)
	binary.BigEndian.PutUint64(buf[8:16], f.V)
	return buf
}

func (f DHashFingerprint) String() string {
	return hex.EncodeToString(f.Bytes())
}

// DHashEngine computes 128-bit 2D dual-gradient difference hashing.
type DHashEngine struct{}

func NewDHashEngine() *DHashEngine {
	return &DHashEngine{}
}

func (e *DHashEngine) Metric() Metric {
	return MetricDHash
}

func (e *DHashEngine) Compute(img image.Image) Fingerprint {
	// 1. Horizontal gradient: 9 cols x 8 rows
	hGray := ResizeGrayscale(img, 9, 8)
	var hBits uint64
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			p1 := hGray[y*9+x]
			p2 := hGray[y*9+x+1]
			if p1 >= p2 {
				hBits |= 1 << (y*8 + x)
			}
		}
	}

	// 2. Vertical gradient: 8 cols x 9 rows
	vGray := ResizeGrayscale(img, 8, 9)
	var vBits uint64
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			p1 := vGray[y*8+x]
			p2 := vGray[(y+1)*8+x]
			if p1 >= p2 {
				vBits |= 1 << (y*8 + x)
			}
		}
	}

	return DHashFingerprint{H: hBits, V: vBits}
}

func (e *DHashEngine) DecodeFingerprint(data []byte) (Fingerprint, error) {
	if len(data) != 16 {
		return nil, errors.New("invalid dhash fingerprint length: expected 16 bytes")
	}
	return DHashFingerprint{
		H: binary.BigEndian.Uint64(data[0:8]),
		V: binary.BigEndian.Uint64(data[8:16]),
	}, nil
}

func (e *DHashEngine) Similarity(f1, f2 Fingerprint) SimilarityScore {
	df1, ok1 := f1.(DHashFingerprint)
	df2, ok2 := f2.(DHashFingerprint)
	if !ok1 || !ok2 {
		return NewSimilarityScore(0)
	}

	dist := bits.OnesCount64(df1.H^df2.H) + bits.OnesCount64(df1.V^df2.V)
	score := (1.0 - (float32(dist) / 128.0)) * 100.0
	return NewSimilarityScore(score)
}

func (e *DHashEngine) FormatDistance(f1, f2 Fingerprint) string {
	df1, ok1 := f1.(DHashFingerprint)
	df2, ok2 := f2.(DHashFingerprint)
	if !ok1 || !ok2 {
		return "N/A"
	}
	dist := bits.OnesCount64(df1.H^df2.H) + bits.OnesCount64(df1.V^df2.V)
	return fmt.Sprintf("%d/128 bits", dist)
}
