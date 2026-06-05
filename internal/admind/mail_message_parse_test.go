package admind

import (
	"mime"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/emersion/go-imap/v2"
)

func TestMailMailboxResponsesFromListDataSkipsNonSelectableMailboxes(t *testing.T) {
	total := uint32(3)
	unseen := uint32(1)
	mailboxes := mailMailboxResponsesFromListData([]*imap.ListData{
		{Mailbox: "INBOX", Status: &imap.StatusData{NumMessages: &total, NumUnseen: &unseen}},
		{Mailbox: "Folders", Attrs: []imap.MailboxAttr{imap.MailboxAttrNoSelect}},
	})
	if len(mailboxes) != 1 {
		t.Fatalf("mailboxes = %#v", mailboxes)
	}
	if mailboxes[0].Name != "INBOX" || mailboxes[0].Total != 3 || mailboxes[0].Unseen != 1 {
		t.Fatalf("mailbox = %#v", mailboxes[0])
	}
}

func TestDecodeMailHeaderDecodesEncodedWords(t *testing.T) {
	encodedSubject := mime.QEncoding.Encode("utf-8", "테스트 제목")
	if subject := decodeMailHeader(encodedSubject); subject != "테스트 제목" {
		t.Fatalf("subject = %q", subject)
	}
}

func TestIMAPAddressListStringDecodesDisplayNames(t *testing.T) {
	encodedName := mime.QEncoding.Encode("utf-8", "네이버")
	addresses := []imap.Address{{Name: encodedName, Mailbox: "account_noreply", Host: "navercorp.com"}}
	if value := imapAddressListString(addresses); value != "네이버 <account_noreply@navercorp.com>" {
		t.Fatalf("address = %q", value)
	}
}

func TestParseMailDocumentKeepsHTMLBody(t *testing.T) {
	document := strings.Join([]string{
		"Content-Type: multipart/alternative; boundary=frontier",
		"",
		"--frontier",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"Plain body",
		"--frontier",
		"Content-Type: text/html; charset=utf-8",
		"",
		"<html><body><strong>HTML body</strong></body></html>",
		"--frontier--",
		"",
	}, "\r\n")
	parsedDocument := parseMailDocument([]byte(document))
	if parsedDocument.PlainText != "Plain body" {
		t.Fatalf("plain text = %q", parsedDocument.PlainText)
	}
	if !strings.Contains(parsedDocument.HTML, "<strong>HTML body</strong>") {
		t.Fatalf("html = %q", parsedDocument.HTML)
	}
}

func TestParseMailDocumentFallsBackToHTMLText(t *testing.T) {
	document := strings.Join([]string{
		"Content-Type: text/html; charset=utf-8",
		"",
		"<html><body><strong>HTML only</strong></body></html>",
	}, "\r\n")
	parsedDocument := parseMailDocument([]byte(document))
	if parsedDocument.PlainText != "HTML only" {
		t.Fatalf("plain text = %q", parsedDocument.PlainText)
	}
	if !strings.Contains(parsedDocument.HTML, "<strong>HTML only</strong>") {
		t.Fatalf("html = %q", parsedDocument.HTML)
	}
}

func TestMailPreviewTruncatesByRune(t *testing.T) {
	preview := mailPreview(strings.Repeat("가", 221))
	if !utf8.ValidString(preview) {
		t.Fatalf("preview is invalid UTF-8: %q", preview)
	}
	if len([]rune(preview)) != 220 {
		t.Fatalf("preview rune length = %d", len([]rune(preview)))
	}
}
