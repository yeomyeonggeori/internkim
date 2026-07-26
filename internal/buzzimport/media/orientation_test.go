package media

import (
	"image"
	"image/color"
	"testing"
)

func minimalJPEGWithOrientation(orientation byte) []byte {
	tiff := []byte{
		'I', 'I', 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00, // TIFF header, IFD0 at offset 8
		0x01, 0x00, // one entry
		0x12, 0x01, 0x03, 0x00, 0x01, 0x00, 0x00, 0x00, orientation, 0x00, 0x00, 0x00, // Orientation SHORT
		0x00, 0x00, 0x00, 0x00, // next IFD
	}
	payload := append([]byte("Exif\x00\x00"), tiff...)
	segmentLength := len(payload) + 2
	content := []byte{0xFF, 0xD8, 0xFF, 0xE1, byte(segmentLength >> 8), byte(segmentLength)}
	return append(content, payload...)
}

func TestReadExifOrientation(t *testing.T) {
	for _, orientation := range []byte{1, 3, 6, 8} {
		if got := readExifOrientation(minimalJPEGWithOrientation(orientation)); got != int(orientation) {
			t.Fatalf("orientation %d: got %d", orientation, got)
		}
	}
	if got := readExifOrientation([]byte("not a jpeg")); got != 1 {
		t.Fatalf("non-jpeg should default to 1, got %d", got)
	}
}

func TestApplyOrientationRotate90(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 2, 1))
	topLeft := color.RGBA{R: 10, A: 255}
	topRight := color.RGBA{R: 20, A: 255}
	source.Set(0, 0, topLeft)
	source.Set(1, 0, topRight)

	rotated := applyOrientation(source, 6) // 90° clockwise
	if rotated.Bounds().Dx() != 1 || rotated.Bounds().Dy() != 2 {
		t.Fatalf("expected 1x2, got %dx%d", rotated.Bounds().Dx(), rotated.Bounds().Dy())
	}
	// After 90° CW the original top-left moves to the top-right (0,0) of a 1-wide image.
	if r, _, _, _ := rotated.At(0, 0).RGBA(); uint8(r>>8) != 10 {
		t.Fatalf("top-left pixel not where expected after rotation")
	}
	if r, _, _, _ := rotated.At(0, 1).RGBA(); uint8(r>>8) != 20 {
		t.Fatalf("top-right pixel not where expected after rotation")
	}
}

func TestApplyOrientationNormalIsNoOp(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 3, 2))
	if applyOrientation(source, 1) != image.Image(source) {
		t.Fatal("orientation 1 should return the source unchanged")
	}
}
