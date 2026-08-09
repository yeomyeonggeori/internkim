package mail

import (
	"errors"
	"fmt"
	"strings"

	messagemail "github.com/emersion/go-message/mail"
)

func DefaultAccount(actorEmail string) Account {
	return Account{
		ActorEmail:     strings.ToLower(strings.TrimSpace(actorEmail)),
		Email:          strings.ToLower(strings.TrimSpace(actorEmail)),
		FromAddress:    strings.ToLower(strings.TrimSpace(actorEmail)),
		IMAPPort:       993,
		IMAPSecurity:   securityTLS,
		SMTPPort:       587,
		SMTPSecurity:   securityStartTLS,
		DefaultMailbox: "INBOX",
		SentMailbox:    "Sent",
	}
}

func MergeAccountWriteRequest(account Account, payload AccountWriteRequest) (Account, error) {
	account.Email = strings.ToLower(strings.TrimSpace(firstNonEmpty(payload.Email, account.Email, account.ActorEmail)))
	account.FromAddress = strings.TrimSpace(firstNonEmpty(payload.FromAddress, account.FromAddress, account.Email))
	account.DisplayName = strings.TrimSpace(payload.DisplayName)
	account.IMAPHost = strings.ToLower(strings.TrimSpace(payload.IMAPHost))
	account.IMAPPort = firstPositiveInteger(payload.IMAPPort, account.IMAPPort, 993)
	account.IMAPSecurity = normalizeSecurity(firstNonEmpty(payload.IMAPSecurity, account.IMAPSecurity, securityTLS))
	account.IMAPUsername = strings.TrimSpace(firstNonEmpty(payload.IMAPUsername, account.IMAPUsername, account.Email))
	account.SMTPHost = strings.ToLower(strings.TrimSpace(payload.SMTPHost))
	account.SMTPPort = firstPositiveInteger(payload.SMTPPort, account.SMTPPort, 587)
	account.SMTPSecurity = normalizeSecurity(firstNonEmpty(payload.SMTPSecurity, account.SMTPSecurity, securityStartTLS))
	account.SMTPUsername = strings.TrimSpace(firstNonEmpty(payload.SMTPUsername, account.SMTPUsername, account.Email))
	account.DefaultMailbox = strings.TrimSpace(firstNonEmpty(payload.DefaultMailbox, account.DefaultMailbox, "INBOX"))
	if payload.SentMailbox != nil {
		account.SentMailbox = strings.TrimSpace(*payload.SentMailbox)
	}
	if strings.TrimSpace(payload.IMAPPassword) != "" {
		account.IMAPPassword = strings.TrimSpace(payload.IMAPPassword)
	}
	if strings.TrimSpace(payload.SMTPPassword) != "" {
		account.SMTPPassword = strings.TrimSpace(payload.SMTPPassword)
	}
	if account.IMAPSecurity == "" || account.SMTPSecurity == "" {
		return Account{}, errors.New("mail security must be tls, starttls, or none")
	}
	return NormalizeAccount(account), nil
}

func NormalizeAccount(account Account) Account {
	account.ActorEmail = strings.ToLower(strings.TrimSpace(account.ActorEmail))
	account.Email = strings.ToLower(strings.TrimSpace(account.Email))
	account.FromAddress = strings.TrimSpace(account.FromAddress)
	account.DisplayName = strings.TrimSpace(account.DisplayName)
	account.IMAPHost = strings.ToLower(strings.TrimSpace(account.IMAPHost))
	account.IMAPSecurity = normalizeSecurity(account.IMAPSecurity)
	account.IMAPUsername = strings.TrimSpace(account.IMAPUsername)
	account.SMTPHost = strings.ToLower(strings.TrimSpace(account.SMTPHost))
	account.SMTPSecurity = normalizeSecurity(account.SMTPSecurity)
	account.SMTPUsername = strings.TrimSpace(account.SMTPUsername)
	account.DefaultMailbox = strings.TrimSpace(firstNonEmpty(account.DefaultMailbox, "INBOX"))
	account.SentMailbox = strings.TrimSpace(account.SentMailbox)
	if account.IMAPPort == 0 {
		account.IMAPPort = 993
	}
	if account.SMTPPort == 0 {
		account.SMTPPort = 587
	}
	return account
}

func ValidateAccountForSave(account Account) error {
	if _, errorValue := messagemail.ParseAddress(account.Email); errorValue != nil {
		return fmt.Errorf("email is invalid: %w", errorValue)
	}
	if _, errorValue := messagemail.ParseAddress(account.FromAddress); errorValue != nil {
		return fmt.Errorf("fromAddress is invalid: %w", errorValue)
	}
	if strings.TrimSpace(account.IMAPHost) == "" {
		return errors.New("imapHost is required")
	}
	if strings.TrimSpace(account.IMAPUsername) == "" {
		return errors.New("imapUsername is required")
	}
	if strings.TrimSpace(account.IMAPPassword) == "" {
		return errors.New("imapPassword is required")
	}
	if strings.TrimSpace(account.SMTPHost) == "" {
		return errors.New("smtpHost is required")
	}
	if strings.TrimSpace(account.SMTPUsername) == "" {
		return errors.New("smtpUsername is required")
	}
	if strings.TrimSpace(account.SMTPPassword) == "" {
		return errors.New("smtpPassword is required")
	}
	if !isValidPort(account.IMAPPort) {
		return errors.New("imapPort must be between 1 and 65535")
	}
	if !isValidPort(account.SMTPPort) {
		return errors.New("smtpPort must be between 1 and 65535")
	}
	if normalizeSecurity(account.IMAPSecurity) == "" || normalizeSecurity(account.SMTPSecurity) == "" {
		return errors.New("mail security must be tls, starttls, or none")
	}
	return nil
}

func AccountToResponse(account Account) AccountResponse {
	return AccountResponse{
		Email:           account.Email,
		FromAddress:     account.FromAddress,
		DisplayName:     account.DisplayName,
		IMAPHost:        account.IMAPHost,
		IMAPPort:        account.IMAPPort,
		IMAPSecurity:    account.IMAPSecurity,
		IMAPUsername:    account.IMAPUsername,
		SMTPHost:        account.SMTPHost,
		SMTPPort:        account.SMTPPort,
		SMTPSecurity:    account.SMTPSecurity,
		SMTPUsername:    account.SMTPUsername,
		DefaultMailbox:  account.DefaultMailbox,
		SentMailbox:     account.SentMailbox,
		IsConfigured:    account.IsConfigured(),
		HasIMAPPassword: strings.TrimSpace(account.IMAPPassword) != "",
		HasSMTPPassword: strings.TrimSpace(account.SMTPPassword) != "",
	}
}

func (account Account) IsConfigured() bool {
	return strings.TrimSpace(account.IMAPHost) != "" &&
		strings.TrimSpace(account.IMAPUsername) != "" &&
		strings.TrimSpace(account.IMAPPassword) != "" &&
		strings.TrimSpace(account.SMTPHost) != "" &&
		strings.TrimSpace(account.SMTPUsername) != "" &&
		strings.TrimSpace(account.SMTPPassword) != ""
}

func normalizeSecurity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", securityTLS:
		return securityTLS
	case securityStartTLS:
		return securityStartTLS
	case securityNone:
		return securityNone
	default:
		return ""
	}
}

func isValidPort(port int) bool {
	return port > 0 && port <= 65535
}

func firstPositiveInteger(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
