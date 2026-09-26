package admind

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestLearningProxyPreservesSafeUpstreamClientStatuses(t *testing.T) {
	for _, statusCode := range []int{http.StatusForbidden, http.StatusConflict} {
		service := NewService(Configuration{BlueclawBaseURL: "http://blueclaw.local", BlueclawAssertionKeyPath: writeTestFile(t, "fixture-secret")})
		service.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return jsonResponse(statusCode, `private provider detail`, nil), nil
		})}
		var response map[string]any
		errorValue := service.blueclawSignedRequest(context.Background(), http.MethodGet, "/admin/api/agent-learning/settings", nil, "sample-person", &response)
		var httpError blueclawHTTPError
		if !errors.As(errorValue, &httpError) || httpError.statusCode != statusCode {
			t.Fatalf("status %d was not preserved: %v", statusCode, errorValue)
		}
	}
}

func TestLearningProxySignsTheCompleteRequestTarget(t *testing.T) {
	service := NewService(Configuration{BlueclawBaseURL: "http://blueclaw.local", BlueclawAssertionKeyPath: writeTestFile(t, "fixture-secret")})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		header := request.Header.Get(memoryAssertionHeader)
		parts := strings.Split(header, ".")
		if len(parts) != 2 {
			t.Fatal("missing signed identity")
		}
		mac := hmac.New(sha256.New, []byte("fixture-secret"))
		_, _ = mac.Write([]byte(request.Method + "\n" + request.URL.RequestURI() + "\n" + parts[0]))
		if !hmac.Equal([]byte(base64.RawURLEncoding.EncodeToString(mac.Sum(nil))), []byte(parts[1])) {
			t.Fatal("outgoing signature does not bind the query")
		}
		var claims struct {
			ReaderPersonID string `json:"readerPersonID"`
			ExpiresAt      int64  `json:"expiresAt"`
		}
		document, errorValue := base64.RawURLEncoding.DecodeString(parts[0])
		if errorValue != nil || json.Unmarshal(document, &claims) != nil || claims.ReaderPersonID != "sample-person" || claims.ExpiresAt <= time.Now().Unix() {
			t.Fatal("invalid reader claims")
		}
		return jsonResponse(http.StatusOK, `{"skills":[]}`, nil), nil
	})}
	var response map[string]any
	if errorValue := service.blueclawSignedRequest(context.Background(), http.MethodGet, "/admin/api/agent-learning/skills?includeRetired=true", nil, "sample-person", &response); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestLearningRoutesRejectUnsignedReadersAndSoulWrites(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir()})
	for _, path := range []string{"/agent-learning/api/skills", "/agent-learning/api/soul", "/agent-learning/api/soul/history"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set(requesterEmailHeader, "sample@example.com")
		response := httptest.NewRecorder()
		service.router().ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("unsigned reader reached %s: %d", path, response.Code)
		}
	}
	for _, path := range []string{"/agent-learning/api/soul", "/agent-learning/api/soul/history", "/agent-learning/api/soul/restore"} {
		response := httptest.NewRecorder()
		service.router().ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`)))
		if response.Code != http.StatusMethodNotAllowed && response.Code != http.StatusNotFound {
			t.Fatalf("soul write route accepted %s: %d", path, response.Code)
		}
	}
}

func TestLearningAssertionBindsQuery(t *testing.T) {
	body := []byte(`{"includeRetired":true}`)
	header, errorValue := signRequestAssertion(http.MethodGet, "/admin/api/agent-learning/skills?includeRetired=true", body, "person-example", 4102444800, "fixture-secret")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	requestURL := &url.URL{Path: "/admin/api/agent-learning/skills", RawQuery: "includeRetired=true"}
	if requestURL.RawQuery == "" {
		t.Fatal("query was not preserved for the request")
	}
	parts := strings.Split(header, ".")
	if len(parts) != 2 {
		t.Fatalf("invalid assertion %q", header)
	}
	var claims struct {
		ReaderPersonID string `json:"readerPersonID"`
		ExpiresAt      int64  `json:"expiresAt"`
		BodySHA256     string `json:"bodySHA256"`
	}
	claimsBytes, errorValue := base64.RawURLEncoding.DecodeString(parts[0])
	if errorValue != nil || json.Unmarshal(claimsBytes, &claims) != nil {
		t.Fatal("assertion claims are not decodable")
	}
	mac := hmac.New(sha256.New, []byte("fixture-secret"))
	_, _ = mac.Write([]byte(http.MethodGet + "\n" + requestURL.RequestURI() + "\n" + parts[0]))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[1])) || claims.ReaderPersonID != "person-example" {
		t.Fatalf("assertion does not match the Blueclaw path contract: %q", header)
	}
	mac = hmac.New(sha256.New, []byte("fixture-secret"))
	requestURL.RawQuery = "includeRetired=false"
	_, _ = mac.Write([]byte(http.MethodGet + "\n" + requestURL.RequestURI() + "\n" + parts[0]))
	querySigned := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if hmac.Equal([]byte(querySigned), []byte(parts[1])) {
		t.Fatal("assertion accepted a changed query string")
	}
}
