package mail

import (
	"fmt"

	"gitlab.com/eastriver/internkim/internal/box"
)

type PasswordOpener func(account Account, memberID string) (Account, error)

func BoxPasswords(boxStateDirectoryPath string) PasswordOpener {
	return func(account Account, memberID string) (Account, error) {
		if account.SealedIMAPPassword == nil && account.SealedSMTPPassword == nil {
			return account, nil
		}
		connected, errorValue := box.LoadConnected(boxStateDirectoryPath)
		if errorValue != nil {
			return Account{}, fmt.Errorf("the mail password of %s is sealed to a box: %w", account.ActorEmail, errorValue)
		}
		return openedPasswords(account, connected, memberID)
	}
}

func openedPasswords(account Account, connected box.Connected, memberID string) (Account, error) {
	imapPassword, errorValue := openedPassword(account.SealedIMAPPassword, account.IMAPPassword, connected, memberID, "IMAPPassword")
	if errorValue != nil {
		return Account{}, errorValue
	}
	smtpPassword, errorValue := openedPassword(account.SealedSMTPPassword, account.SMTPPassword, connected, memberID, "SMTPPassword")
	if errorValue != nil {
		return Account{}, errorValue
	}
	account.IMAPPassword, account.SMTPPassword = imapPassword, smtpPassword
	account.SealedIMAPPassword, account.SealedSMTPPassword = nil, nil
	return account, nil
}

func openedPassword(sealed *box.SealedSecret, held string, connected box.Connected, memberID, field string) (string, error) {
	if sealed == nil {
		return held, nil
	}
	opened, errorValue := connected.Identity.OpenSecret(*sealed, box.MailPasswordPurpose(connected.CompanyID, memberID, field))
	if errorValue != nil {
		return "", fmt.Errorf("the %s of member %s: %w", field, memberID, errorValue)
	}
	return opened, nil
}
