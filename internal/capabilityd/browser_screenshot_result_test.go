package capabilityd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"testing"

	browserruntime "github.com/yeomyeonggeori/internkim/internal/browser"
)

func TestBrowserScreenshotResultCarriesImageBytesWithoutADevicePath(t *testing.T) {
	screenshotBytes := []byte("complete screenshot bytes")
	result := browserScreenshotResultOf(browserruntime.ScreenshotResult{
		Content:     screenshotBytes,
		Filename:    "browser-screenshot-sample.png",
		SizeBytes:   int64(len(screenshotBytes)),
		ContentType: "image/png",
		CapturedAt:  "2026-10-05T00:00:00Z",
	})
	document, errorValue := json.Marshal(result)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	var decodedResult struct {
		Attachments []map[string]json.RawMessage `json:"attachments"`
	}
	if errorValue := json.Unmarshal(document, &decodedResult); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(decodedResult.Attachments) != 1 {
		t.Fatalf("expected one screenshot attachment, got %s", document)
	}
	attachment := decodedResult.Attachments[0]
	if _, hasDevicePath := attachment["devicePath"]; hasDevicePath {
		t.Fatalf("screenshot result claimed a device path: %s", document)
	}

	var filename string
	if errorValue := json.Unmarshal(attachment["filename"], &filename); errorValue != nil || filename != "browser-screenshot-sample.png" {
		t.Fatalf("expected screenshot filename to survive, got %q (%v)", filename, errorValue)
	}
	var contentType string
	if errorValue := json.Unmarshal(attachment["contentType"], &contentType); errorValue != nil || contentType != "image/png" {
		t.Fatalf("expected screenshot content type to survive, got %q (%v)", contentType, errorValue)
	}
	var sizeBytes int64
	if errorValue := json.Unmarshal(attachment["sizeBytes"], &sizeBytes); errorValue != nil || sizeBytes != int64(len(screenshotBytes)) {
		t.Fatalf("expected screenshot size to match its bytes, got %d (%v)", sizeBytes, errorValue)
	}
	var contentBase64 string
	if errorValue := json.Unmarshal(attachment["contentBase64"], &contentBase64); errorValue != nil {
		t.Fatal(errorValue)
	}
	decodedBytes, errorValue := base64.StdEncoding.DecodeString(contentBase64)
	if errorValue != nil || !bytes.Equal(decodedBytes, screenshotBytes) {
		t.Fatalf("expected the complete screenshot bytes, got %q (%v)", decodedBytes, errorValue)
	}
}
