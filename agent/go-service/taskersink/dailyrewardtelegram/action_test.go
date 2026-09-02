package dailyrewardtelegram

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func TestEncodeScreenshot(t *testing.T) {
	screenshot := image.NewRGBA(image.Rect(0, 0, 4, 3))
	screenshot.Set(1, 1, color.RGBA{R: 255, G: 200, B: 10, A: 255})

	encoded, err := encodeScreenshot(screenshot)
	if err != nil {
		t.Fatalf("encodeScreenshot() error = %v", err)
	}
	decoded, err := jpeg.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("jpeg.Decode() error = %v", err)
	}
	if decoded.Bounds() != screenshot.Bounds() {
		t.Errorf("decoded bounds = %v, want %v", decoded.Bounds(), screenshot.Bounds())
	}
}

func TestEncodeScreenshotRejectsNil(t *testing.T) {
	if _, err := encodeScreenshot(nil); err == nil {
		t.Fatal("encodeScreenshot(nil) error = nil, want error")
	}
}
