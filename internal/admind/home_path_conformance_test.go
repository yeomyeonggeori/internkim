package admind

import (
	"encoding/json"
	"os"
	"testing"
)

func TestTheDeviceSendsPeopleWhereTheWebAppStarts(t *testing.T) {
	document, errorValue := os.ReadFile("../../web/static/manifest.webmanifest")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var manifest struct {
		StartURL string `json:"start_url"`
	}
	if errorValue := json.Unmarshal(document, &manifest); errorValue != nil {
		t.Fatal(errorValue)
	}
	if manifest.StartURL == "" {
		t.Fatal("the manifest is what keeps the device and the browser landing in the same place")
	}
	if webHomePath != manifest.StartURL {
		t.Errorf("webHomePath = %q, manifest start_url = %q", webHomePath, manifest.StartURL)
	}
}
