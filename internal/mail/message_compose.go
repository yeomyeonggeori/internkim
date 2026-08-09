package mail

import (
	"bytes"
	"io"
	"strings"
	"time"

	messagemail "github.com/emersion/go-message/mail"
)

func createMessageDocument(account Account, input MessageSendRequest) ([]byte, []string, error) {
	from, errorValue := messagemail.ParseAddress(account.FromAddress)
	if errorValue != nil {
		return nil, nil, errorValue
	}
	if strings.TrimSpace(account.DisplayName) != "" {
		from.Name = account.DisplayName
	}
	to, errorValue := parseAddresses(input.To)
	if errorValue != nil {
		return nil, nil, errorValue
	}
	cc, errorValue := parseAddresses(input.CC)
	if errorValue != nil {
		return nil, nil, errorValue
	}
	bcc, errorValue := parseAddresses(input.BCC)
	if errorValue != nil {
		return nil, nil, errorValue
	}
	var header messagemail.Header
	header.SetAddressList("From", []*messagemail.Address{from})
	header.SetAddressList("To", to)
	header.SetAddressList("Cc", cc)
	header.SetSubject(input.Subject)
	header.SetDate(time.Now())
	_ = header.GenerateMessageID()
	var document bytes.Buffer
	writer, errorValue := messagemail.CreateSingleInlineWriter(&document, header)
	if errorValue != nil {
		return nil, nil, errorValue
	}
	if _, errorValue := io.WriteString(writer, input.Body); errorValue != nil {
		_ = writer.Close()
		return nil, nil, errorValue
	}
	if errorValue := writer.Close(); errorValue != nil {
		return nil, nil, errorValue
	}
	recipients := append(addressStrings(to), addressStrings(cc)...)
	recipients = append(recipients, addressStrings(bcc)...)
	return document.Bytes(), recipients, nil
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
