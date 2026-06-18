package admind

import (
	"mime"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/charset"
)

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
