package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	nostr "github.com/nbd-wtf/go-nostr"
)

type Blob struct {
	URL      string `json:"url"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
	MimeType string `json:"type"`
	Dim      string `json:"dim"`
}

type Uploader struct {
	HTTPBaseURL string
	HTTPClient  *http.Client
}

func (uploader Uploader) httpClient() *http.Client {
	if uploader.HTTPClient != nil {
		return uploader.HTTPClient
	}
	return &http.Client{Timeout: 60 * time.Second}
}

// Upload stores the bytes on the relay's Blossom endpoint, signing the BUD-02
// auth event with the given identity so the blob is attributable to that person.
func (uploader Uploader) Upload(ctx context.Context, actorSecretHex string, content []byte, mimeType string) (Blob, error) {
	digest := sha256.Sum256(content)
	digestHex := hex.EncodeToString(digest[:])

	authEvent := nostr.Event{
		CreatedAt: nostr.Now(),
		Kind:      24242,
		Tags: nostr.Tags{
			nostr.Tag{"t", "upload"},
			nostr.Tag{"x", digestHex},
			nostr.Tag{"expiration", strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)},
		},
		Content: "upload",
	}
	if errorValue := authEvent.Sign(actorSecretHex); errorValue != nil {
		return Blob{}, errorValue
	}
	authorization, errorValue := blossomAuthorization(authEvent)
	if errorValue != nil {
		return Blob{}, errorValue
	}

	var response *http.Response
	for attempt := 0; ; attempt++ {
		request, errorValue := http.NewRequestWithContext(ctx, http.MethodPut, uploader.HTTPBaseURL+"/upload", bytes.NewReader(content))
		if errorValue != nil {
			return Blob{}, errorValue
		}
		request.Header.Set("Authorization", authorization)
		request.Header.Set("Content-Type", mimeType)
		request.Header.Set("X-SHA-256", digestHex)
		response, errorValue = uploader.httpClient().Do(request)
		if errorValue != nil {
			return Blob{}, errorValue
		}
		if response.StatusCode != http.StatusTooManyRequests || attempt >= 5 {
			break
		}
		response.Body.Close()
		select {
		case <-ctx.Done():
			return Blob{}, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * time.Second):
		}
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Blob{}, fmt.Errorf("blossom upload returned %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	var blob Blob
	if errorValue := json.Unmarshal(body, &blob); errorValue != nil {
		return Blob{}, errorValue
	}
	if blob.SHA256 == "" {
		blob.SHA256 = digestHex
	}
	if blob.Size == 0 {
		blob.Size = int64(len(content))
	}
	if blob.MimeType == "" {
		blob.MimeType = mimeType
	}
	return blob, nil
}

func (blob Blob) IMetaTag() []string {
	tag := []string{
		"imeta",
		"url " + blob.URL,
		"m " + blob.MimeType,
		"x " + blob.SHA256,
		"size " + strconv.FormatInt(blob.Size, 10),
	}
	if strings.TrimSpace(blob.Dim) != "" {
		tag = append(tag, "dim "+blob.Dim)
	}
	return tag
}

func blossomAuthorization(authEvent nostr.Event) (string, error) {
	document, errorValue := json.Marshal(authEvent)
	if errorValue != nil {
		return "", errorValue
	}
	return "Nostr " + base64.StdEncoding.EncodeToString(document), nil
}
