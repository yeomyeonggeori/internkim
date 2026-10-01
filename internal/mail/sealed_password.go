package mail

import (
	"fmt"

	"github.com/yeomyeonggeori/internkim/internal/box"
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
	imapPurpose := box.MailPasswordPurpose(connected.CompanyID, memberID, "IMAPPassword", box.MailConnection{
		Host: account.IMAPHost, Port: account.IMAPPort, Security: account.IMAPSecurity, Username: account.IMAPUsername,
	})
	imapPassword, errorValue := openedPassword(account.SealedIMAPPassword, account.IMAPPassword, connected, imapPurpose)
	if errorValue != nil {
		return Account{}, fmt.Errorf("the IMAP password of member %s: %w", memberID, errorValue)
	}
	smtpPurpose := box.MailPasswordPurpose(connected.CompanyID, memberID, "SMTPPassword", box.MailConnection{
		Host: account.SMTPHost, Port: account.SMTPPort, Security: account.SMTPSecurity, Username: account.SMTPUsername,
	})
	smtpPassword, errorValue := openedPassword(account.SealedSMTPPassword, account.SMTPPassword, connected, smtpPurpose)
	if errorValue != nil {
		return Account{}, fmt.Errorf("the SMTP password of member %s: %w", memberID, errorValue)
	}
	account.IMAPPassword, account.SMTPPassword = imapPassword, smtpPassword
	account.SealedIMAPPassword, account.SealedSMTPPassword = nil, nil
	return account, nil
}

func openedPassword(sealed *box.SealedSecret, held string, connected box.Connected, purpose box.SealPurpose) (string, error) {
	if sealed == nil {
		return held, nil
	}
	return connected.Identity.OpenSecret(*sealed, purpose)
}
