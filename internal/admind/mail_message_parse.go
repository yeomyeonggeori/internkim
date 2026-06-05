package admind

import (
	"bytes"
	"errors"
	"html"
	"io"
	"mime"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/charset"
	messagemail "github.com/emersion/go-message/mail"
)

type parsedMailDocument struct {
	PlainText string
	HTML      string
}

const maxMailPreviewCharacters = 220

func mailMessageResponseFromBuffer(mailbox string, message *imapclient.FetchMessageBuffer, bodySection *imap.FetchItemBodySection) mailMessageResponse {
	if message == nil {
		return mailMessageResponse{}
	}
	document := bodySectionBytes(message, bodySection)
	parsedDocument := parseMailDocument(document)
	date := message.InternalDate
	subject := ""
	from := ""
	if message.Envelope != nil {
		subject = decodeMailHeader(message.Envelope.Subject)
		from = imapAddressListString(message.Envelope.From)
		if !message.Envelope.Date.IsZero() {
			date = message.Envelope.Date
		}
	}
	return mailMessageResponse{
		UID:     uint32(message.UID),
		Mailbox: mailbox,
		Subject: subject,
		From:    from,
		Date:    formatMailDate(date),
		Preview: mailPreview(parsedDocument.previewText()),
		IsRead:  containsMailFlag(message.Flags, imap.FlagSeen),
	}
}

func mailMessageDetailFromBuffer(mailbox string, message *imapclient.FetchMessageBuffer, bodySection *imap.FetchItemBodySection) mailMessageDetailResponse {
	response := mailMessageResponseFromBuffer(mailbox, message, bodySection)
	parsedDocument := parseMailDocument(bodySectionBytes(message, bodySection))
	to := ""
	cc := ""
	if message != nil && message.Envelope != nil {
		to = imapAddressListString(message.Envelope.To)
		cc = imapAddressListString(message.Envelope.Cc)
	}
	return mailMessageDetailResponse{
		UID:      response.UID,
		Mailbox:  response.Mailbox,
		Subject:  response.Subject,
		From:     response.From,
		To:       to,
		CC:       cc,
		Date:     response.Date,
		Body:     parsedDocument.PlainText,
		BodyHTML: parsedDocument.HTML,
		IsRead:   response.IsRead,
	}
}

func bodySectionBytes(message *imapclient.FetchMessageBuffer, bodySection *imap.FetchItemBodySection) []byte {
	if message == nil {
		return nil
	}
	for _, section := range message.BodySection {
		if bodySection == nil || section.Section == nil || section.Section.Specifier == bodySection.Specifier {
			return section.Bytes
		}
	}
	return nil
}

func plainTextFromMailDocument(document []byte) string {
	return parseMailDocument(document).PlainText
}

func parseMailDocument(document []byte) parsedMailDocument {
	if len(bytes.TrimSpace(document)) == 0 {
		return parsedMailDocument{}
	}
	reader, errorValue := messagemail.CreateReader(bytes.NewReader(document))
	if errorValue != nil && reader == nil {
		return parsedMailDocument{PlainText: strings.TrimSpace(string(document))}
	}
	defer reader.Close()
	result := parsedMailDocument{}
	for {
		part, errorValue := reader.NextPart()
		if errors.Is(errorValue, io.EOF) {
			break
		}
		if errorValue != nil || part == nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(part.Body, 1024*1024))
		contentType := mailPartContentType(part)
		if result.PlainText == "" && contentType == "text/plain" {
			result.PlainText = strings.TrimSpace(string(body))
		}
		if result.HTML == "" && contentType == "text/html" {
			result.HTML = strings.TrimSpace(string(body))
		}
	}
	if result.PlainText == "" && result.HTML != "" {
		result.PlainText = stripHTML(result.HTML)
	}
	if result.PlainText == "" {
		result.PlainText = strings.TrimSpace(string(document))
	}
	return result
}

func mailPartContentType(part *messagemail.Part) string {
	if part == nil || part.Header == nil {
		return ""
	}
	if header, ok := part.Header.(interface {
		ContentType() (string, map[string]string, error)
	}); ok {
		contentType, _, errorValue := header.ContentType()
		if errorValue == nil {
			return strings.ToLower(contentType)
		}
	}
	contentType, _, errorValue := mime.ParseMediaType(part.Header.Get("Content-Type"))
	if errorValue == nil {
		return strings.ToLower(contentType)
	}
	return strings.ToLower(strings.TrimSpace(part.Header.Get("Content-Type")))
}

func (document parsedMailDocument) previewText() string {
	if strings.TrimSpace(document.PlainText) != "" {
		return document.PlainText
	}
	return stripHTML(document.HTML)
}

func decodeMailHeader(value string) string {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return ""
	}
	decodedValue, errorValue := mailWordDecoder().DecodeHeader(trimmedValue)
	if errorValue != nil {
		return trimmedValue
	}
	return strings.TrimSpace(decodedValue)
}

func mailWordDecoder() *mime.WordDecoder {
	return &mime.WordDecoder{CharsetReader: charset.Reader}
}

func stripHTML(document string) string {
	builder := strings.Builder{}
	isTag := false
	for _, value := range document {
		switch value {
		case '<':
			isTag = true
		case '>':
			isTag = false
			builder.WriteRune(' ')
		default:
			if !isTag {
				builder.WriteRune(value)
			}
		}
	}
	return strings.Join(strings.Fields(html.UnescapeString(builder.String())), " ")
}

func mailPreview(body string) string {
	preview := strings.Join(strings.Fields(body), " ")
	runes := []rune(preview)
	if len(runes) > maxMailPreviewCharacters {
		return string(runes[:maxMailPreviewCharacters])
	}
	return preview
}

func imapAddressListString(addresses []imap.Address) string {
	values := []string{}
	for _, address := range addresses {
		emailAddress := address.Addr()
		if emailAddress == "" {
			continue
		}
		if strings.TrimSpace(address.Name) != "" {
			values = append(values, displayMailAddress(decodeMailHeader(address.Name), emailAddress))
		} else {
			values = append(values, emailAddress)
		}
	}
	return strings.Join(values, ", ")
}

func displayMailAddress(name string, address string) string {
	displayName := strings.TrimSpace(name)
	emailAddress := strings.TrimSpace(address)
	if displayName == "" {
		return emailAddress
	}
	return displayName + " <" + emailAddress + ">"
}

func formatMailDate(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
