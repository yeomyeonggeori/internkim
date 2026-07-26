package media

import (
	"context"
	"os"
	"strings"
	"testing"
)

// A 1x1 PNG.
var onePixelPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x62, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
	0x42, 0x60, 0x82,
}

func TestBlossomUploadStoresAndReturnsAServableURL(t *testing.T) {
	httpBaseURL := os.Getenv("BUZZ_MEDIA_TEST_HTTP_URL")
	actorSecret := os.Getenv("BUZZ_MEDIA_TEST_ACTOR_SECRET")
	if httpBaseURL == "" || actorSecret == "" {
		t.Skip("set BUZZ_MEDIA_TEST_HTTP_URL and BUZZ_MEDIA_TEST_ACTOR_SECRET to run")
	}
	imageBytes := onePixelPNG
	if imagePath := os.Getenv("BUZZ_MEDIA_TEST_IMAGE"); imagePath != "" {
		loaded, readError := os.ReadFile(imagePath)
		if readError != nil {
			t.Fatalf("read test image: %v", readError)
		}
		imageBytes = loaded
	}
	uploader := Uploader{HTTPBaseURL: strings.TrimRight(httpBaseURL, "/")}
	blob, errorValue := uploader.Upload(context.Background(), actorSecret, imageBytes, "image/png")
	if errorValue != nil {
		t.Fatalf("upload: %v", errorValue)
	}
	if blob.SHA256 == "" || blob.URL == "" {
		t.Fatalf("expected a stored blob, got %+v", blob)
	}
	if !strings.Contains(blob.URL, blob.SHA256) {
		t.Fatalf("served URL should carry the content hash, got %q", blob.URL)
	}
	tag := blob.IMetaTag()
	if tag[0] != "imeta" || !strings.HasPrefix(tag[1], "url ") {
		t.Fatalf("unexpected imeta tag: %v", tag)
	}
}
