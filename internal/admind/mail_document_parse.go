package admind

import (
	"bytes"
	"errors"
	"html"
	"io"
	"mime"
	"strings"

	messagemail "github.com/emersion/go-message/mail"
)

type parsedMailDocument struct {
	PlainText string
	HTML      string
}

const maxMailPreviewCharacters = 220

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
