package engine

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"math/bits"
)

// PDQFingerprint holds a 256-bit PDQ perceptual hash (4 x uint64 = 32 bytes).
type PDQFingerprint struct {
	Words [4]uint64
}

func (f PDQFingerprint) Bytes() []byte {
	buf := make([]byte, 32)
	for i := 0; i < 4; i++ {
		binary.BigEndian.PutUint64(buf[i*8:(i+1)*8], f.Words[i])
	}
	return buf
}

func (f PDQFingerprint) String() string {
	return hex.EncodeToString(f.Bytes())
}

// PDQEngine computes 256-bit perceptual hashes based on the PDQ specification (16x16 DCT).
type PDQEngine struct{}

func NewPDQEngine() *PDQEngine {
	return &PDQEngine{}
}

func (e *PDQEngine) Metric() Metric {
	return MetricPDQ
}

func (e *PDQEngine) Compute(img image.Image) Fingerprint {
	const N = 64
	// Downsample to 64x64 grayscale
	gray := ResizeGrayscale(img, N, N)

	floatBuf := make([]float64, N*N)
	for i, v := range gray {
		floatBuf[i] = float64(v)
	}

	// 2D DCT on 64x64
	dct := ComputeDCT2D(floatBuf, N)

	// Extract 16x16 low-frequency matrix (256 coefficients)
	vals := make([]float64, 0, 255)
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			if x == 0 && y == 0 {
				continue // Skip DC
			}
			vals = append(vals, dct[y*N+x])
		}
	}

	med := Median(vals)

	var words [4]uint64
	bitIdx := 0
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			val := dct[y*N+x]
			if val > med {
				wordIdx := bitIdx / 64
				bitInWord := bitIdx % 64
				words[wordIdx] |= 1 << bitInWord
			}
			bitIdx++
		}
	}

	return PDQFingerprint{Words: words}
}

func (e *PDQEngine) DecodeFingerprint(data []byte) (Fingerprint, error) {
	if len(data) != 32 {
		return nil, errors.New("invalid pdq fingerprint length: expected 32 bytes")
	}
	var words [4]uint64
	for i := 0; i < 4; i++ {
		words[i] = binary.BigEndian.Uint64(data[i*8 : (i+1)*8])
	}
	return PDQFingerprint{Words: words}, nil
}

func (e *PDQEngine) Similarity(f1, f2 Fingerprint) SimilarityScore {
	p1, ok1 := f1.(PDQFingerprint)
	p2, ok2 := f2.(PDQFingerprint)
	if !ok1 || !ok2 {
		return NewSimilarityScore(0)
	}

	dist := bits.OnesCount64(p1.Words[0]^p2.Words[0]) +
		bits.OnesCount64(p1.Words[1]^p2.Words[1]) +
		bits.OnesCount64(p1.Words[2]^p2.Words[2]) +
		bits.OnesCount64(p1.Words[3]^p2.Words[3])

	score := (1.0 - (float32(dist) / 256.0)) * 100.0
	return NewSimilarityScore(score)
}

func (e *PDQEngine) FormatDistance(f1, f2 Fingerprint) string {
	p1, ok1 := f1.(PDQFingerprint)
	p2, ok2 := f2.(PDQFingerprint)
	if !ok1 || !ok2 {
		return "N/A"
	}
	dist := bits.OnesCount64(p1.Words[0]^p2.Words[0]) +
		bits.OnesCount64(p1.Words[1]^p2.Words[1]) +
		bits.OnesCount64(p1.Words[2]^p2.Words[2]) +
		bits.OnesCount64(p1.Words[3]^p2.Words[3])
	return fmt.Sprintf("%d/256 bits", dist)
}
