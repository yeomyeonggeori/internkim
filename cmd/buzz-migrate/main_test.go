package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/buzzimport/assetkeep"

	"gitlab.com/eastriver/internkim/internal/buzzimport/media"
)

func TestDescribeMediaTagReadsEntriesByName(t *testing.T) {
	blob := media.Blob{
		URL:      "http://localhost:3000/abc.pdf",
		SHA256:   "abc",
		Size:     12,
		MimeType: "application/pdf",
	}.Named("2026 예산.pdf")

	described := describeMediaTag(blob.IMetaTag())

	if described.url != blob.URL {
		t.Fatalf("url = %q, want %q", described.url, blob.URL)
	}
	if described.mimeType != blob.MimeType {
		t.Fatalf("mimeType = %q, want %q", described.mimeType, blob.MimeType)
	}
	if described.filename != "2026 예산.pdf" {
		t.Fatalf("filename = %q, want %q", described.filename, "2026 예산.pdf")
	}
}

func TestAppendMediaMarkdownNamesWhatEachFileIs(t *testing.T) {
	picture := media.Blob{URL: "http://localhost:3000/a.jpg", MimeType: "image/jpeg"}.Named("회식.jpg")
	document := media.Blob{URL: "http://localhost:3000/b.pdf", MimeType: "application/pdf"}.Named("2026 예산.pdf")

	text := appendMediaMarkdown("보냅니다", [][]string{picture.IMetaTag(), document.IMetaTag()})

	want := "보냅니다\n![회식.jpg](http://localhost:3000/a.jpg)\n[2026 예산.pdf](http://localhost:3000/b.pdf)"
	if text != want {
		t.Fatalf("markdown =\n%q\nwant\n%q", text, want)
	}
}

func TestAppendMediaMarkdownFallsBackWhenTheFileHasNoName(t *testing.T) {
	unnamed := media.Blob{URL: "http://localhost:3000/c.bin", MimeType: "application/octet-stream"}

	text := appendMediaMarkdown("", [][]string{unnamed.IMetaTag()})

	if text != "[file](http://localhost:3000/c.bin)" {
		t.Fatalf("markdown = %q", text)
	}
}

func TestAFileTheStoreWillNotCarryIsKeptWhereTheCompanyCanReadIt(t *testing.T) {
	plane := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(writer).Encode(map[string]any{
			"companyID": "43000000-0000-0000-0000-0000000000a0", "accessToken": "t", "expiresAt": 1<<62 - 1,
		})
	}))
	defer plane.Close()
	store := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	defer store.Close()
	keeper := &assetkeep.Keeper{AppURL: plane.URL, AgentKey: "k", ProjectURL: store.URL, PublishableKey: "p"}
	refusal := media.Refusal{Status: 415, Reason: "disallowed content type: text/html"}

	blob, errorValue := keptWhereTheStoreWouldNot(context.Background(), keeper, []byte("a page"), "text/html", refusal)

	if errorValue != nil {
		t.Fatalf("kept: %v", errorValue)
	}
	if !strings.Contains(blob.URL, "/storage/v1/object/asset/") {
		t.Errorf("url = %q", blob.URL)
	}
	if blob.Size != 6 || blob.MimeType != "text/html" || blob.SHA256 == "" {
		t.Errorf("blob = %+v", blob)
	}
}

func TestAStoreThatIsOnlyBusyIsNotGivenASecondHome(t *testing.T) {
	kept := 0
	store := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		kept++
		writer.WriteHeader(http.StatusOK)
	}))
	defer store.Close()
	keeper := &assetkeep.Keeper{AppURL: store.URL, AgentKey: "k", ProjectURL: store.URL, PublishableKey: "p"}

	_, errorValue := keptWhereTheStoreWouldNot(
		context.Background(), keeper, []byte("a page"), "text/html",
		media.Refusal{Status: 429, Reason: "upload rate limit exceeded"},
	)

	if errorValue == nil {
		t.Fatal("a busy store was treated as a refusal")
	}
	if kept != 0 {
		t.Errorf("kept %d copies of a file that goes later", kept)
	}
}

func TestAFailureThatIsNotARefusalIsReportedAsItCame(t *testing.T) {
	broken := errors.New("connection reset")

	_, errorValue := keptWhereTheStoreWouldNot(context.Background(), nil, []byte("a"), "text/html", broken)

	if !errors.Is(errorValue, broken) {
		t.Errorf("error = %v", errorValue)
	}
}

func TestARefusalWithNowhereToKeepItSaysSo(t *testing.T) {
	refusal := media.Refusal{Status: 415, Reason: "disallowed content type: text/html"}

	_, errorValue := keptWhereTheStoreWouldNot(context.Background(), nil, []byte("a"), "text/html", refusal)

	if !errors.Is(errorValue, refusal) {
		t.Fatalf("error = %v", errorValue)
	}
	if !strings.Contains(errorValue.Error(), "no asset store") {
		t.Errorf("error = %v", errorValue)
	}
}
