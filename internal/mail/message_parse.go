package mail

import (
	"mime"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/charset"
)

func messageResponseFromBuffer(mailbox string, message *imapclient.FetchMessageBuffer, bodySection *imap.FetchItemBodySection) MessageResponse {
	if message == nil {
		return MessageResponse{}
	}
	document := bodySectionBytes(message, bodySection)
	parsedDocument := ParseDocument(document)
	date := message.InternalDate
	subject := ""
	from := ""
	if message.Envelope != nil {
		subject = DecodeHeader(message.Envelope.Subject)
		from = IMAPAddressListString(message.Envelope.From)
		if !message.Envelope.Date.IsZero() {
			date = message.Envelope.Date
		}
	}
	return MessageResponse{
		UID:     uint32(message.UID),
		Mailbox: mailbox,
		Subject: subject,
		From:    from,
		Date:    formatDate(date),
		Preview: Preview(parsedDocument.previewText()),
		IsRead:  containsFlag(message.Flags, imap.FlagSeen),
	}
}

func messageDetailFromBuffer(mailbox string, message *imapclient.FetchMessageBuffer, bodySection *imap.FetchItemBodySection) MessageDetailResponse {
	response := messageResponseFromBuffer(mailbox, message, bodySection)
	parsedDocument := ParseDocument(bodySectionBytes(message, bodySection))
	to := ""
	cc := ""
	if message != nil && message.Envelope != nil {
		to = IMAPAddressListString(message.Envelope.To)
		cc = IMAPAddressListString(message.Envelope.Cc)
	}
	return MessageDetailResponse{
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

func DecodeHeader(value string) string {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return ""
	}
	decodedValue, errorValue := wordDecoder().DecodeHeader(trimmedValue)
	if errorValue != nil {
		return trimmedValue
	}
	return strings.TrimSpace(decodedValue)
}

func wordDecoder() *mime.WordDecoder {
	return &mime.WordDecoder{CharsetReader: charset.Reader}
}

func IMAPAddressListString(addresses []imap.Address) string {
	values := []string{}
	for _, address := range addresses {
		emailAddress := address.Addr()
		if emailAddress == "" {
			continue
		}
		if strings.TrimSpace(address.Name) != "" {
			values = append(values, displayAddress(DecodeHeader(address.Name), emailAddress))
		} else {
			values = append(values, emailAddress)
		}
	}
	return strings.Join(values, ", ")
}

func displayAddress(name string, address string) string {
	displayName := strings.TrimSpace(name)
	emailAddress := strings.TrimSpace(address)
	if displayName == "" {
		return emailAddress
	}
	return displayName + " <" + emailAddress + ">"
}

func formatDate(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
