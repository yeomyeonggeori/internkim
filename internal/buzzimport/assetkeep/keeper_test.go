package assetkeep

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

const company = "43000000-0000-0000-0000-0000000000a0"

func keeperAgainst(t *testing.T, store http.HandlerFunc) (*Keeper, func()) {
	t.Helper()
	plane := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer an-agent-key" {
			writer.WriteHeader(http.StatusForbidden)
			return
		}
		json.NewEncoder(writer).Encode(map[string]any{
			"companyID":   company,
			"accessToken": "a-host-session",
			"expiresAt":   1<<62 - 1,
		})
	}))
	storeServer := httptest.NewServer(store)
	keeper := &Keeper{
		AppURL:         plane.URL,
		AgentKey:       "an-agent-key",
		ProjectURL:     storeServer.URL,
		PublishableKey: "a-publishable-key",
	}
	return keeper, func() {
		plane.Close()
		storeServer.Close()
	}
}

func TestKeepWritesTheFileWhereTheCompanyCanReadIt(t *testing.T) {
	var seen struct {
		path          string
		authorization string
		apiKey        string
		contentType   string
		body          string
	}
	keeper, done := keeperAgainst(t, func(writer http.ResponseWriter, request *http.Request) {
		body := make([]byte, request.ContentLength)
		request.Body.Read(body)
		seen.path = request.URL.Path
		seen.authorization = request.Header.Get("Authorization")
		seen.apiKey = request.Header.Get("apikey")
		seen.contentType = request.Header.Get("Content-Type")
		seen.body = string(body)
		writer.WriteHeader(http.StatusOK)
	})
	defer done()

	kept, errorValue := keeper.Keep(context.Background(), []byte("a page"), "text/html")

	if errorValue != nil {
		t.Fatalf("keep: %v", errorValue)
	}
	wantedPath := "/storage/v1/object/asset/" + AttachmentPath(company, kept.Digest, "text/html")
	if seen.path != wantedPath {
		t.Errorf("path = %q, wanted %q", seen.path, wantedPath)
	}
	if seen.authorization != "Bearer a-host-session" {
		t.Errorf("authorization = %q", seen.authorization)
	}
	if seen.apiKey != "a-publishable-key" {
		t.Errorf("apikey = %q", seen.apiKey)
	}
	if seen.contentType != "text/html" {
		t.Errorf("content type = %q", seen.contentType)
	}
	if seen.body != "a page" {
		t.Errorf("body = %q", seen.body)
	}
	if !strings.HasSuffix(kept.Address, wantedPath) {
		t.Errorf("address = %q, wanted it to end in %q", kept.Address, wantedPath)
	}
	if kept.SizeBytes != 6 {
		t.Errorf("size = %d", kept.SizeBytes)
	}
}

// The same file posted in two conversations is kept twice. Supabase refuses a
// second write to an existing object with HTTP 400 whose body says 409, so
// asking it to overwrite is what keeps the second message's attachment: the
// object is named by the hash of its own contents, so the bytes do not change.
func TestTheSameBytesAreKeptTwiceWithoutLosingTheSecondMessage(t *testing.T) {
	upserts := []string{}
	keeper, done := keeperAgainst(t, func(writer http.ResponseWriter, request *http.Request) {
		upserts = append(upserts, request.Header.Get("x-upsert"))
		writer.WriteHeader(http.StatusOK)
	})
	defer done()

	first, errorValue := keeper.Keep(context.Background(), []byte("a page"), "text/html")
	if errorValue != nil {
		t.Fatalf("first keep: %v", errorValue)
	}
	second, errorValue := keeper.Keep(context.Background(), []byte("a page"), "text/html")
	if errorValue != nil {
		t.Fatalf("a file the bucket already holds was reported as a failure: %v", errorValue)
	}

	if second.Address != first.Address {
		t.Errorf("the same bytes were addressed twice: %q then %q", first.Address, second.Address)
	}
	if upserts[0] != "true" || upserts[1] != "true" {
		t.Errorf("x-upsert = %v, so a second write would be refused", upserts)
	}
}

func TestAStoreThatRefusesIsReportedRatherThanAddressed(t *testing.T) {
	keeper, done := keeperAgainst(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		writer.Write([]byte("no"))
	})
	defer done()

	_, errorValue := keeper.Keep(context.Background(), []byte("a page"), "text/html")

	if errorValue == nil {
		t.Fatal("a refused write was reported as kept")
	}
	if !strings.Contains(errorValue.Error(), "401") {
		t.Errorf("error = %v", errorValue)
	}
}

func TestOneSessionServesEveryFile(t *testing.T) {
	sessions := 0
	plane := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		sessions++
		json.NewEncoder(writer).Encode(map[string]any{
			"companyID":   company,
			"accessToken": "a-host-session",
			"expiresAt":   1<<62 - 1,
		})
	}))
	defer plane.Close()
	store := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	defer store.Close()
	keeper := &Keeper{AppURL: plane.URL, AgentKey: "k", ProjectURL: store.URL, PublishableKey: "p"}

	for index := 0; index < 3; index++ {
		if _, errorValue := keeper.Keep(context.Background(), []byte{byte(index)}, "text/html"); errorValue != nil {
			t.Fatalf("keep: %v", errorValue)
		}
	}

	if sessions != 1 {
		t.Errorf("asked the central plane for %d sessions", sessions)
	}
}

// The relay decides the same object names from TypeScript. Two hand-kept copies
// of a path layout drift; this reads the other one.
func TestAssetPathMatchesTheRelay(t *testing.T) {
	source, errorValue := os.ReadFile("../../../host/relay/asset-store.ts")
	if errorValue != nil {
		t.Fatalf("read the relay's asset store: %v", errorValue)
	}
	relay := string(source)

	if !strings.Contains(relay, "return `${companyID}/shared/${kind}/${digest}${extensionOf(contentType)}`") {
		t.Error("the relay no longer names an object <company>/shared/<kind>/<digest><extension>")
	}
	if !strings.Contains(relay, `export const attachmentKind = 'attachment'`) {
		t.Error("the relay no longer calls a message attachment 'attachment'")
	}
	for contentType, extension := range extensions {
		if !strings.Contains(relay, "'"+contentType+"': '"+extension+"'") {
			t.Errorf("the relay does not give %s the extension %s", contentType, extension)
		}
	}
	if strings.Count(relay, "': '.") != len(extensions) {
		t.Errorf("the relay knows %d extensions, this knows %d", strings.Count(relay, "': '."), len(extensions))
	}
	if !strings.Contains(relay, "export const assetBucket = '"+bucket+"'") {
		t.Errorf("the relay keeps assets somewhere other than the %s bucket", bucket)
	}
	if !strings.Contains(relay, "return `${projectURL.replace(/\\/+$/, '')}/storage/v1/object/${assetBucket}/${path}`") {
		t.Error("the relay no longer addresses an object as <project>/storage/v1/object/<bucket>/<path>")
	}
}
