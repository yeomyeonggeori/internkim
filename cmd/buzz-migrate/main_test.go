package main

import (
	"testing"

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

func TestLoopbackCommunityHostIsRefused(t *testing.T) {
	for _, host := range []string{"localhost:3000", "LOCALHOST", "127.0.0.1:3000", "127.0.0.53", "[::1]:3000"} {
		if loopbackCommunityHostRefusal(host, false) == "" {
			t.Errorf("community host %q was accepted", host)
		}
	}
}

func TestReachableCommunityHostIsAccepted(t *testing.T) {
	for _, host := range []string{"buzz.example.com", "relay.example.com:3000", "10.0.0.4:3000", "[2001:db8::1]:3000"} {
		if refusal := loopbackCommunityHostRefusal(host, false); refusal != "" {
			t.Errorf("community host %q was refused: %s", host, refusal)
		}
	}
}

func TestAThrowawayImportMayUseLoopback(t *testing.T) {
	if refusal := loopbackCommunityHostRefusal("localhost:3000", true); refusal != "" {
		t.Errorf("throwaway import was refused: %s", refusal)
	}
}
