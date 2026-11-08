package engine

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"math/bits"
)

// WaveletFingerprint holds a 64-bit 2D Haar Wavelet fingerprint.
type WaveletFingerprint struct {
	Hash uint64
}

func (f WaveletFingerprint) Bytes() []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, f.Hash)
	return buf
}

func (f WaveletFingerprint) String() string {
	return hex.EncodeToString(f.Bytes())
}

// WaveletEngine computes 64-bit perceptual hashes using 2D Haar Discrete Wavelet Transform.
type WaveletEngine struct{}

func NewWaveletEngine() *WaveletEngine {
	return &WaveletEngine{}
}

func (e *WaveletEngine) Metric() Metric {
	return MetricWavelet
}

func (e *WaveletEngine) Compute(img image.Image) Fingerprint {
	const N = 32
	gray := ResizeGrayscale(img, N, N)

	floatBuf := make([]float64, N*N)
	for i, v := range gray {
		floatBuf[i] = float64(v)
	}

	// 2 levels of Haar DWT -> produces an 8x8 LL approximation subband in top-left
	dwt := ComputeHaar2D(floatBuf, N, 2)

	// Extract the 8x8 LL subband
	llVals := make([]float64, 0, 63)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if x == 0 && y == 0 {
				continue // Skip DC
			}
			llVals = append(llVals, dwt[y*N+x])
		}
	}

	med := Median(llVals)

	var hash uint64
	bitIdx := 0
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			val := dwt[y*N+x]
			if val > med {
				hash |= 1 << bitIdx
			}
			bitIdx++
		}
	}

	return WaveletFingerprint{Hash: hash}
}

func (e *WaveletEngine) DecodeFingerprint(data []byte) (Fingerprint, error) {
	if len(data) != 8 {
		return nil, errors.New("invalid wavelet fingerprint length: expected 8 bytes")
	}
	return WaveletFingerprint{
		Hash: binary.BigEndian.Uint64(data),
	}, nil
}

func (e *WaveletEngine) Similarity(f1, f2 Fingerprint) SimilarityScore {
	w1, ok1 := f1.(WaveletFingerprint)
	w2, ok2 := f2.(WaveletFingerprint)
	if !ok1 || !ok2 {
		return NewSimilarityScore(0)
	}

	dist := bits.OnesCount64(w1.Hash ^ w2.Hash)
	score := (1.0 - (float32(dist) / 64.0)) * 100.0
	return NewSimilarityScore(score)
}

func (e *WaveletEngine) FormatDistance(f1, f2 Fingerprint) string {
	w1, ok1 := f1.(WaveletFingerprint)
	w2, ok2 := f2.(WaveletFingerprint)
	if !ok1 || !ok2 {
		return "N/A"
	}
	dist := bits.OnesCount64(w1.Hash ^ w2.Hash)
	return fmt.Sprintf("%d/64 bits", dist)
}
