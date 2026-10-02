// Package images prepares guest ID photos for storage: only JPEG or PNG, no metadata, re-encoded as JPEG.
package images

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"net/http"

	"github.com/pcaokhai/stayguard/api/internal/app"
)

const (
	// MaxBytes is the largest upload accepted (docs/15 rule 24: 5 MB).
	MaxBytes = 5 << 20
	// maxPixels bounds the decoded size: a small file can describe a huge bitmap.
	maxPixels = 30_000_000
	// maxSide keeps the stored photo readable but small: an ID card needs no more than this.
	maxSide     = 2400
	jpegQuality = 85
)

// The errors are the use case's, so the handler maps them in one place.
var (
	ErrTooLarge    = app.ErrPhotoTooLarge
	ErrUnsupported = app.ErrUnsupportedMedia
	ErrInvalid     = app.ErrPhotoInvalid
)

// Sanitizer implements app.ImageSanitizer.
type Sanitizer struct{}

// Sanitize decodes a JPEG or PNG, applies the EXIF orientation of a JPEG, scales it down when large and encodes a
// new JPEG. The output carries no EXIF, GPS or other metadata because it is built from pixels only.
func (Sanitizer) Sanitize(raw []byte) ([]byte, error) {
	if len(raw) > MaxBytes {
		return nil, ErrTooLarge
	}
	kind := http.DetectContentType(raw)
	if kind != "image/jpeg" && kind != "image/png" {
		return nil, ErrUnsupported
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, ErrInvalid
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxPixels {
		return nil, ErrInvalid
	}
	var img image.Image
	if kind == "image/jpeg" {
		img, err = jpeg.Decode(bytes.NewReader(raw))
		if err == nil {
			img = orient(img, exifOrientation(raw))
		}
	} else {
		img, err = png.Decode(bytes.NewReader(raw))
	}
	if err != nil {
		return nil, ErrInvalid
	}
	img = scaleDown(flatten(img))
	var out bytes.Buffer
	if err = jpeg.Encode(&out, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, ErrInvalid
	}
	return out.Bytes(), nil
}

// flatten draws the image on white: JPEG has no alpha, and transparent pixels must not turn black.
func flatten(src image.Image) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Over)
	return dst
}

// scaleDown shrinks by whole-pixel nearest sampling when the longer side exceeds maxSide.
func scaleDown(src *image.RGBA) *image.RGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	long := max(w, h)
	if long <= maxSide {
		return src
	}
	nw, nh := max(1, w*maxSide/long), max(1, h*maxSide/long)
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		for x := 0; x < nw; x++ {
			dst.Set(x, y, src.At(x*w/nw, y*h/nh))
		}
	}
	return dst
}

// exifOrientation reads the orientation tag (1 to 8) of a JPEG, or 1 when there is none or it cannot be read.
func exifOrientation(b []byte) int {
	if len(b) < 4 || b[0] != 0xFF || b[1] != 0xD8 {
		return 1
	}
	i := 2
	for i+4 <= len(b) && b[i] == 0xFF {
		marker, size := b[i+1], int(b[i+2])<<8|int(b[i+3])
		if marker == 0xDA || size < 2 || i+2+size > len(b) { // start of scan or a broken segment
			return 1
		}
		if marker == 0xE1 {
			if o := tiffOrientation(b[i+4 : i+2+size]); o != 0 {
				return o
			}
		}
		i += 2 + size
	}
	return 1
}

func tiffOrientation(seg []byte) int {
	if len(seg) < 14 || string(seg[:6]) != "Exif\x00\x00" {
		return 0
	}
	t := seg[6:]
	var u16 func([]byte) int
	var u32 func([]byte) int
	switch string(t[:2]) {
	case "II":
		u16 = func(p []byte) int { return int(p[0]) | int(p[1])<<8 }
		u32 = func(p []byte) int { return int(p[0]) | int(p[1])<<8 | int(p[2])<<16 | int(p[3])<<24 }
	case "MM":
		u16 = func(p []byte) int { return int(p[0])<<8 | int(p[1]) }
		u32 = func(p []byte) int { return int(p[0])<<24 | int(p[1])<<16 | int(p[2])<<8 | int(p[3]) }
	default:
		return 0
	}
	off := u32(t[4:8])
	if off < 8 || off+2 > len(t) {
		return 0
	}
	n := u16(t[off:])
	for k := 0; k < n; k++ {
		e := off + 2 + 12*k
		if e+12 > len(t) {
			return 0
		}
		if u16(t[e:]) == 0x0112 {
			if v := u16(t[e+8:]); v >= 1 && v <= 8 {
				return v
			}
			return 0
		}
	}
	return 0
}

// orient applies EXIF orientation 1..8 so the stored pixels are upright.
func orient(src image.Image, o int) image.Image {
	if o <= 1 || o > 8 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	nw, nh := w, h
	if o >= 5 {
		nw, nh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch o {
			case 2:
				dx, dy = w-1-x, y
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dx, dy = x, h-1-y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			dst.Set(dx, dy, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}
