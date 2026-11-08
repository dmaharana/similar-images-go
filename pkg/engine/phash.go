package engine

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"math/bits"
)

// PHashFingerprint represents a 64-bit DCT-based perceptual hash.
type PHashFingerprint struct {
	Hash uint64
}

func (f PHashFingerprint) Bytes() []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, f.Hash)
	return buf
}

func (f PHashFingerprint) String() string {
	return hex.EncodeToString(f.Bytes())
}

// PHashEngine computes 64-bit perceptual hashes using 2D Discrete Cosine Transform (DCT-II).
type PHashEngine struct{}

func NewPHashEngine() *PHashEngine {
	return &PHashEngine{}
}

func (e *PHashEngine) Metric() Metric {
	return MetricPHash
}

func (e *PHashEngine) Compute(img image.Image) Fingerprint {
	const N = 32
	gray := ResizeGrayscale(img, N, N)

	// Convert uint8 to float64 for DCT
	floatBuf := make([]float64, N*N)
	for i, v := range gray {
		floatBuf[i] = float64(v)
	}

	dct := ComputeDCT2D(floatBuf, N)

	// Extract 8x8 low-frequency coefficients (excluding DC component at (0,0) for median calculation)
	acVals := make([]float64, 0, 63)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if x == 0 && y == 0 {
				continue // Skip DC term
			}
			acVals = append(acVals, dct[y*N+x])
		}
	}

	med := Median(acVals)

	var hash uint64
	bitIdx := 0
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			val := dct[y*N+x]
			if val > med {
				hash |= 1 << bitIdx
			}
			bitIdx++
		}
	}

	return PHashFingerprint{Hash: hash}
}

func (e *PHashEngine) DecodeFingerprint(data []byte) (Fingerprint, error) {
	if len(data) != 8 {
		return nil, errors.New("invalid phash fingerprint length: expected 8 bytes")
	}
	return PHashFingerprint{
		Hash: binary.BigEndian.Uint64(data),
	}, nil
}

func (e *PHashEngine) Similarity(f1, f2 Fingerprint) SimilarityScore {
	p1, ok1 := f1.(PHashFingerprint)
	p2, ok2 := f2.(PHashFingerprint)
	if !ok1 || !ok2 {
		return NewSimilarityScore(0)
	}

	dist := bits.OnesCount64(p1.Hash ^ p2.Hash)
	score := (1.0 - (float32(dist) / 64.0)) * 100.0
	return NewSimilarityScore(score)
}

func (e *PHashEngine) FormatDistance(f1, f2 Fingerprint) string {
	p1, ok1 := f1.(PHashFingerprint)
	p2, ok2 := f2.(PHashFingerprint)
	if !ok1 || !ok2 {
		return "N/A"
	}
	dist := bits.OnesCount64(p1.Hash ^ p2.Hash)
	return fmt.Sprintf("%d/64 bits", dist)
}
