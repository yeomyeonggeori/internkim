package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractOAuthClientCredentialsJSON(t *testing.T) {
	testCases := []struct {
		name             string
		body             string
		expectedClientID string
		expectedSecret   string
	}{
		{
			name:             "downloaded oauth json",
			body:             `{"installed":{"client_id":"desktop.apps.googleusercontent.com","client_secret":"secret-from-installed"}}`,
			expectedClientID: "desktop.apps.googleusercontent.com",
			expectedSecret:   "secret-from-installed",
		},
		{
			name:             "camel case credential response",
			body:             `{"oauthClientCredential":{"clientId":"desktop.apps.googleusercontent.com","clientSecret":"secret-from-response"}}`,
			expectedClientID: "desktop.apps.googleusercontent.com",
			expectedSecret:   "secret-from-response",
		},
		{
			name:             "secret only response",
			body:             `{"credential":{"clientSecret":"secret-only"}}`,
			expectedClientID: "",
			expectedSecret:   "secret-only",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			clientID, clientSecret, err := extractOAuthClientCredentialsJSON([]byte(testCase.body))
			if err != nil {
				t.Fatalf("extractOAuthClientCredentialsJSON returned error: %v", err)
			}
			if clientID != testCase.expectedClientID {
				t.Fatalf("clientID = %q, want %q", clientID, testCase.expectedClientID)
			}
			if clientSecret != testCase.expectedSecret {
				t.Fatalf("clientSecret = %q, want %q", clientSecret, testCase.expectedSecret)
			}
		})
	}
}

func TestParseOAuthClientJSONRequiresSecret(t *testing.T) {
	temporaryDir := t.TempDir()
	jsonPath := filepath.Join(temporaryDir, "client.json")
	if err := os.WriteFile(jsonPath, []byte(`{"installed":{"client_id":"desktop.apps.googleusercontent.com"}}`), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	_, _, err := parseOAuthClientJSON(jsonPath)
	if err == nil {
		t.Fatal("parseOAuthClientJSON returned nil error")
	}
}
