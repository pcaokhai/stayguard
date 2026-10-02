package images

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func solid(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func jpegBytes(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// withExif inserts an APP1 segment (orientation and a GPS-like comment) after the SOI marker.
func withExif(jpg []byte, orientation byte, extra string) []byte {
	tiff := []byte{'M', 'M', 0, 42, 0, 0, 0, 8, 0, 1, 0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, orientation, 0, 0, 0, 0, 0, 0}
	seg := append([]byte("Exif\x00\x00"), tiff...)
	seg = append(seg, []byte(extra)...)
	n := len(seg) + 2
	app1 := append([]byte{0xFF, 0xE1, byte(n >> 8 & 0xFF), byte(n & 0xFF)}, seg...)
	return append(append(append([]byte{}, jpg[:2]...), app1...), jpg[2:]...)
}

func TestSanitize_StripsMetadataAndReencodes_SG805_AC1(t *testing.T) {
	src := withExif(jpegBytes(t, solid(40, 20, color.RGBA{200, 10, 10, 255})), 1, "GPS 10.7769N 106.7009E Canon EOS")
	out, err := Sanitizer{}.Sanitize(src)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("Exif")) || bytes.Contains(out, []byte("GPS")) || bytes.Contains(out, []byte("Canon")) {
		t.Fatal("metadata survived")
	}
	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil || img.Bounds().Dx() != 40 || img.Bounds().Dy() != 20 {
		t.Fatalf("not a valid jpeg of the same size: %v", err)
	}
}

func TestSanitize_AppliesExifOrientation_SG805(t *testing.T) {
	// A wide image tagged "rotate 90 clockwise" must come out tall.
	src := withExif(jpegBytes(t, solid(40, 20, color.RGBA{0, 0, 200, 255})), 6, "")
	out, err := Sanitizer{}.Sanitize(src)
	if err != nil {
		t.Fatal(err)
	}
	img, _ := jpeg.Decode(bytes.NewReader(out))
	if img.Bounds().Dx() != 20 || img.Bounds().Dy() != 40 {
		t.Fatalf("orientation not applied: %v", img.Bounds())
	}
}

func TestSanitize_PngBecomesJpegWithWhiteBackground_SG805(t *testing.T) {
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 8, 8))) // fully transparent
	out, err := Sanitizer{}.Sanitize(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	img, _ := jpeg.Decode(bytes.NewReader(out))
	r, g, bl, _ := img.At(4, 4).RGBA()
	if r>>8 < 240 || g>>8 < 240 || bl>>8 < 240 {
		t.Fatal("transparent pixels must become white")
	}
}

func TestSanitize_Rejects_SG805(t *testing.T) {
	big := make([]byte, MaxBytes+1)
	for name, c := range map[string]struct {
		in   []byte
		want error
	}{
		"too large":   {big, ErrTooLarge},
		"text":        {[]byte("hello, not an image"), ErrUnsupported},
		"gif":         {[]byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"), ErrUnsupported},
		"empty":       {nil, ErrUnsupported},
		"broken jpeg": {append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 20)...), ErrInvalid},
	} {
		if _, err := (Sanitizer{}).Sanitize(c.in); !errors.Is(err, c.want) {
			t.Errorf("%s: %v want %v", name, err, c.want)
		}
	}
}

func TestSanitize_ScalesLargeImages_SG805(t *testing.T) {
	out, err := Sanitizer{}.Sanitize(jpegBytes(t, solid(3000, 1000, color.Gray{128})))
	if err != nil {
		t.Fatal(err)
	}
	img, _ := jpeg.Decode(bytes.NewReader(out))
	if img.Bounds().Dx() != maxSide || img.Bounds().Dy() != 800 {
		t.Fatalf("size %v", img.Bounds())
	}
}
