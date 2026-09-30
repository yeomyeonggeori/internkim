package mail

import (
	"bytes"
	"errors"
	"html"
	"io"
	"mime"
	"strings"

	messagemail "github.com/emersion/go-message/mail"
)

type parsedDocument struct {
	PlainText string
	HTML      string
}

const maxPreviewCharacters = 220

func ParseDocument(document []byte) parsedDocument {
	if len(bytes.TrimSpace(document)) == 0 {
		return parsedDocument{}
	}
	reader, errorValue := messagemail.CreateReader(bytes.NewReader(document))
	if errorValue != nil && reader == nil {
		return parsedDocument{PlainText: strings.TrimSpace(string(document))}
	}
	defer reader.Close()
	result := parsedDocument{}
	for {
		part, errorValue := reader.NextPart()
		if errors.Is(errorValue, io.EOF) {
			break
		}
		if errorValue != nil || part == nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(part.Body, 1024*1024))
		contentType := partContentType(part)
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

func partContentType(part *messagemail.Part) string {
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

func (document parsedDocument) previewText() string {
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

func Preview(body string) string {
	preview := strings.Join(strings.Fields(body), " ")
	runes := []rune(preview)
	if len(runes) > maxPreviewCharacters {
		return string(runes[:maxPreviewCharacters])
	}
	return preview
}
