package engine

import (
	"image"
	"math"
	"sort"
)

// ResizeGrayscale downsamples an image to targetW x targetH grayscale using bilinear interpolation.
func ResizeGrayscale(img image.Image, targetW, targetH int) []uint8 {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	out := make([]uint8, targetW*targetH)

	if srcW == 0 || srcH == 0 {
		return out
	}

	xRatio := float64(srcW) / float64(targetW)
	yRatio := float64(srcH) / float64(targetH)

	for y := 0; y < targetH; y++ {
		srcY := (float64(y)+0.5)*yRatio - 0.5
		if srcY < 0 {
			srcY = 0
		}
		y0 := int(math.Floor(srcY))
		y1 := y0 + 1
		if y1 >= srcH {
			y1 = srcH - 1
		}
		yWeight := srcY - float64(y0)

		for x := 0; x < targetW; x++ {
			srcX := (float64(x)+0.5)*xRatio - 0.5
			if srcX < 0 {
				srcX = 0
			}
			x0 := int(math.Floor(srcX))
			x1 := x0 + 1
			if x1 >= srcW {
				x1 = srcW - 1
			}
			xWeight := srcX - float64(x0)

			p00 := lumaAt(img, bounds.Min.X+x0, bounds.Min.Y+y0)
			p10 := lumaAt(img, bounds.Min.X+x1, bounds.Min.Y+y0)
			p01 := lumaAt(img, bounds.Min.X+x0, bounds.Min.Y+y1)
			p11 := lumaAt(img, bounds.Min.X+x1, bounds.Min.Y+y1)

			top := p00*(1.0-xWeight) + p10*xWeight
			bottom := p01*(1.0-xWeight) + p11*xWeight
			val := top*(1.0-yWeight) + bottom*yWeight

			out[y*targetW+x] = uint8(math.Round(val))
		}
	}
	return out
}

func lumaAt(img image.Image, x, y int) float64 {
	r, g, b, _ := img.At(x, y).RGBA()
	// Convert 16-bit color channel to 8-bit scale
	r8 := float64(r >> 8)
	g8 := float64(g >> 8)
	b8 := float64(b >> 8)
	// Rec. 601 luma calculation
	return 0.299*r8 + 0.587*g8 + 0.114*b8
}

// ResizeRGB downsamples an image to targetW x targetH RGB buffer (3 bytes per pixel).
func ResizeRGB(img image.Image, targetW, targetH int) []uint8 {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	out := make([]uint8, targetW*targetH*3)

	if srcW == 0 || srcH == 0 {
		return out
	}

	xRatio := float64(srcW) / float64(targetW)
	yRatio := float64(srcH) / float64(targetH)

	for y := 0; y < targetH; y++ {
		srcY := (float64(y)+0.5)*yRatio - 0.5
		if srcY < 0 {
			srcY = 0
		}
		y0 := int(math.Floor(srcY))
		y1 := y0 + 1
		if y1 >= srcH {
			y1 = srcH - 1
		}
		yWeight := srcY - float64(y0)

		for x := 0; x < targetW; x++ {
			srcX := (float64(x)+0.5)*xRatio - 0.5
			if srcX < 0 {
				srcX = 0
			}
			x0 := int(math.Floor(srcX))
			x1 := x0 + 1
			if x1 >= srcW {
				x1 = srcW - 1
			}
			xWeight := srcX - float64(x0)

			r00, g00, b00 := rgbAt(img, bounds.Min.X+x0, bounds.Min.Y+y0)
			r10, g10, b10 := rgbAt(img, bounds.Min.X+x1, bounds.Min.Y+y0)
			r01, g01, b01 := rgbAt(img, bounds.Min.X+x0, bounds.Min.Y+y1)
			r11, g11, b11 := rgbAt(img, bounds.Min.X+x1, bounds.Min.Y+y1)

			r := (r00*(1-xWeight)+r10*xWeight)*(1-yWeight) + (r01*(1-xWeight)+r11*xWeight)*yWeight
			g := (g00*(1-xWeight)+g10*xWeight)*(1-yWeight) + (g01*(1-xWeight)+g11*xWeight)*yWeight
			b := (b00*(1-xWeight)+b10*xWeight)*(1-yWeight) + (b01*(1-xWeight)+b11*xWeight)*yWeight

			idx := (y*targetW + x) * 3
			out[idx] = uint8(math.Round(r))
			out[idx+1] = uint8(math.Round(g))
			out[idx+2] = uint8(math.Round(b))
		}
	}
	return out
}

func rgbAt(img image.Image, x, y int) (float64, float64, float64) {
	r, g, b, _ := img.At(x, y).RGBA()
	return float64(r >> 8), float64(g >> 8), float64(b >> 8)
}

// ComputeDCT2D performs a 2D Discrete Cosine Transform (DCT-II) on an N x N matrix.
func ComputeDCT2D(input []float64, N int) []float64 {
	temp := make([]float64, N*N)
	output := make([]float64, N*N)

	// 1D DCT on rows
	for y := 0; y < N; y++ {
		rowOffset := y * N
		for k := 0; k < N; k++ {
			var sum float64
			for n := 0; n < N; n++ {
				sum += input[rowOffset+n] * math.Cos(math.Pi*float64(2*n+1)*float64(k)/(2.0*float64(N)))
			}
			factor := math.Sqrt(2.0 / float64(N))
			if k == 0 {
				factor = math.Sqrt(1.0 / float64(N))
			}
			temp[rowOffset+k] = sum * factor
		}
	}

	// 1D DCT on columns of temp
	for x := 0; x < N; x++ {
		for k := 0; k < N; k++ {
			var sum float64
			for n := 0; n < N; n++ {
				sum += temp[n*N+x] * math.Cos(math.Pi*float64(2*n+1)*float64(k)/(2.0*float64(N)))
			}
			factor := math.Sqrt(2.0 / float64(N))
			if k == 0 {
				factor = math.Sqrt(1.0 / float64(N))
			}
			output[k*N+x] = sum * factor
		}
	}

	return output
}

// ComputeHaar2D performs a 2D Discrete Haar Wavelet Transform on an N x N matrix down to specified levels.
func ComputeHaar2D(input []float64, N int, levels int) []float64 {
	out := make([]float64, len(input))
	copy(out, input)

	currentSize := N
	sqrt2 := math.Sqrt(2.0)

	for level := 0; level < levels; level++ {
		if currentSize <= 1 {
			break
		}

		half := currentSize / 2
		temp := make([]float64, currentSize*currentSize)

		// Row transform
		for y := 0; y < currentSize; y++ {
			for x := 0; x < half; x++ {
				a := out[y*N+2*x]
				b := out[y*N+2*x+1]
				temp[y*currentSize+x] = (a + b) / sqrt2
				temp[y*currentSize+x+half] = (a - b) / sqrt2
			}
		}

		// Column transform
		for x := 0; x < currentSize; x++ {
			for y := 0; y < half; y++ {
				a := temp[(2*y)*currentSize+x]
				b := temp[(2*y+1)*currentSize+x]
				out[y*N+x] = (a + b) / sqrt2
				out[(y+half)*N+x] = (a - b) / sqrt2
			}
		}

		currentSize = half
	}

	return out
}

// Median calculates the median value of a slice of float64 numbers.
func Median(values []float64) float64 {
	if len(values) == 0 {
		return 0.0
	}
	cp := make([]float64, len(values))
	copy(cp, values)
	sort.Float64s(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 0 {
		return (cp[mid-1] + cp[mid]) / 2.0
	}
	return cp[mid]
}
