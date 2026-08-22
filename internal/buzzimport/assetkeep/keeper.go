// Package assetkeep puts a file the messenger's own store refuses into the
// company's asset bucket, so a message can name it and a reader can open it.
//
// It reaches the bucket the way the relay does (host/relay/relay.ts): an agent
// key buys a session that acts as the company's host, and the publishable key
// goes beside it. No service key is involved, so what this writes is scoped to
// one company by the policies in the asset bucket migration rather than by this
// process being careful.
package assetkeep

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const bucket = "asset"
const attachmentKind = "attachment"

// extensions matches extensionOf in host/relay/asset-store.ts, which decides
// the same object names from the other side. TestAssetPathMatchesTheRelay reads
// that file and fails when the two drift apart.
var extensions = map[string]string{
	"image/png":     ".png",
	"image/jpeg":    ".jpg",
	"image/gif":     ".gif",
	"image/webp":    ".webp",
	"image/svg+xml": ".svg",
}

type Keeper struct {
	AppURL         string
	AgentKey       string
	ProjectURL     string
	PublishableKey string
	HTTPClient     *http.Client

	session hostSession
}

type Kept struct {
	Address   string
	Digest    string
	SizeBytes int64
}

type hostSession struct {
	CompanyID   string `json:"companyID"`
	AccessToken string `json:"accessToken"`
	ExpiresAt   int64  `json:"expiresAt"`
}

func (keeper *Keeper) httpClient() *http.Client {
	if keeper.HTTPClient != nil {
		return keeper.HTTPClient
	}
	return &http.Client{Timeout: 5 * time.Minute}
}

func (keeper *Keeper) Keep(ctx context.Context, content []byte, contentType string) (Kept, error) {
	session, errorValue := keeper.hostOfTheCompany(ctx)
	if errorValue != nil {
		return Kept{}, errorValue
	}
	digest := sha256.Sum256(content)
	digestHex := hex.EncodeToString(digest[:])
	path := AttachmentPath(session.CompanyID, digestHex, contentType)

	if errorValue := keeper.write(ctx, session, path, content, contentType); errorValue != nil {
		return Kept{}, errorValue
	}
	return Kept{Address: keeper.address(path), Digest: digestHex, SizeBytes: int64(len(content))}, nil
}

func AttachmentPath(companyID, digestHex, contentType string) string {
	return fmt.Sprintf("%s/shared/%s/%s%s", companyID, attachmentKind, digestHex, ExtensionOf(contentType))
}

func ExtensionOf(contentType string) string {
	return extensions[strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))]
}

func (keeper *Keeper) address(path string) string {
	return fmt.Sprintf("%s/storage/v1/object/%s/%s", strings.TrimRight(keeper.ProjectURL, "/"), bucket, path)
}

// The bucket is closed, so reading one of its objects takes the same host
// session writing one does. Anything the company keeps there and wants a Buzz
// app to open has to come through here first and be put somewhere open.
func (keeper *Keeper) Read(ctx context.Context, path string) ([]byte, string, error) {
	session, errorValue := keeper.hostOfTheCompany(ctx)
	if errorValue != nil {
		return nil, "", errorValue
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, keeper.address(path), nil)
	if errorValue != nil {
		return nil, "", errorValue
	}
	request.Header.Set("Authorization", "Bearer "+session.AccessToken)
	request.Header.Set("apikey", keeper.PublishableKey)

	response, errorValue := keeper.httpClient().Do(request)
	if errorValue != nil {
		return nil, "", errorValue
	}
	defer response.Body.Close()
	body, errorValue := io.ReadAll(response.Body)
	if errorValue != nil {
		return nil, "", errorValue
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, "", fmt.Errorf("the asset store would not give up %s: %d %s", path, response.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, response.Header.Get("Content-Type"), nil
}

func (keeper *Keeper) write(ctx context.Context, session hostSession, path string, content []byte, contentType string) error {
	request, errorValue := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("%s/storage/v1/object/%s/%s", strings.TrimRight(keeper.ProjectURL, "/"), bucket, path),
		bytes.NewReader(content),
	)
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Authorization", "Bearer "+session.AccessToken)
	request.Header.Set("apikey", keeper.PublishableKey)
	request.Header.Set("Content-Type", contentType)
	// The bucket is addressed by content, so the object this writes over holds
	// the same bytes it is being given. The same file posted in two
	// conversations is written twice and read once.
	request.Header.Set("x-upsert", "true")

	response, errorValue := keeper.httpClient().Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("the asset store refused %s: %d %s", path, response.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (keeper *Keeper) hostOfTheCompany(ctx context.Context) (hostSession, error) {
	if keeper.session.AccessToken != "" && keeper.session.ExpiresAt-time.Now().Unix() > 300 {
		return keeper.session, nil
	}
	request, errorValue := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(keeper.AppURL, "/")+"/api/agent/host-session",
		nil,
	)
	if errorValue != nil {
		return hostSession{}, errorValue
	}
	request.Header.Set("Authorization", "Bearer "+keeper.AgentKey)

	response, errorValue := keeper.httpClient().Do(request)
	if errorValue != nil {
		return hostSession{}, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return hostSession{}, fmt.Errorf("the central plane refused this agent key (%d)", response.StatusCode)
	}
	var session hostSession
	if errorValue := json.NewDecoder(response.Body).Decode(&session); errorValue != nil {
		return hostSession{}, errorValue
	}
	if session.CompanyID == "" || session.AccessToken == "" {
		return hostSession{}, fmt.Errorf("the central plane named no company for this agent key")
	}
	keeper.session = session
	return session, nil
}
