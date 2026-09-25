package mail

import (
	"bytes"
	"io"
	"strings"
	"time"

	messagemail "github.com/emersion/go-message/mail"
)

type composedMessage struct {
	document   []byte
	recipients []string
	messageID  string
}

func createMessageDocument(account Account, input MessageSendRequest) (composedMessage, error) {
	from, errorValue := messagemail.ParseAddress(account.FromAddress)
	if errorValue != nil {
		return composedMessage{}, errorValue
	}
	if strings.TrimSpace(account.DisplayName) != "" {
		from.Name = account.DisplayName
	}
	to, errorValue := parseAddresses(input.To)
	if errorValue != nil {
		return composedMessage{}, errorValue
	}
	cc, errorValue := parseAddresses(input.CC)
	if errorValue != nil {
		return composedMessage{}, errorValue
	}
	bcc, errorValue := parseAddresses(input.BCC)
	if errorValue != nil {
		return composedMessage{}, errorValue
	}
	var header messagemail.Header
	header.SetAddressList("From", []*messagemail.Address{from})
	header.SetAddressList("To", to)
	header.SetAddressList("Cc", cc)
	header.SetSubject(input.Subject)
	header.SetDate(time.Now())
	if errorValue := header.GenerateMessageID(); errorValue != nil {
		return composedMessage{}, errorValue
	}
	messageID, errorValue := header.MessageID()
	if errorValue != nil {
		return composedMessage{}, errorValue
	}
	var document bytes.Buffer
	writer, errorValue := messagemail.CreateSingleInlineWriter(&document, header)
	if errorValue != nil {
		return composedMessage{}, errorValue
	}
	if _, errorValue := io.WriteString(writer, input.Body); errorValue != nil {
		_ = writer.Close()
		return composedMessage{}, errorValue
	}
	if errorValue := writer.Close(); errorValue != nil {
		return composedMessage{}, errorValue
	}
	recipients := append(addressStrings(to), addressStrings(cc)...)
	recipients = append(recipients, addressStrings(bcc)...)
	return composedMessage{document: document.Bytes(), recipients: recipients, messageID: messageID}, nil
}

func parseAddresses(values []string) ([]*messagemail.Address, error) {
	addresses := make([]*messagemail.Address, 0, len(values))
	for _, value := range values {
		address, errorValue := messagemail.ParseAddress(value)
		if errorValue != nil {
			return nil, errorValue
		}
		addresses = append(addresses, address)
	}
	return addresses, nil
}

func addressStrings(addresses []*messagemail.Address) []string {
	values := make([]string, 0, len(addresses))
	for _, address := range addresses {
		values = append(values, address.String())
	}
	return values
}
