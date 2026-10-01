package admind

import (
	"context"

	"gitlab.com/eastriver/internkim/internal/centralplane"
	"gitlab.com/eastriver/internkim/internal/mail"
)

func (service *Service) readMailAccount(ctx context.Context, actorEmail string) (mail.Account, bool, error) {
	client := service.centralPlane()
	if client == nil {
		return mail.Account{}, false, errNoCompanyDirectory
	}
	held, found, errorValue := client.MailAccount(ctx, actorEmail)
	if errorValue != nil {
		return mail.Account{}, false, errorValue
	}
	if !found {
		return mail.DefaultAccount(actorEmail), false, nil
	}
	opened, errorValue := service.mailPasswords(accountOfRecord(actorEmail, held), held.MemberID)
	if errorValue != nil {
		return mail.Account{}, false, errorValue
	}
	return mail.NormalizeAccount(opened), true, nil
}

func (service *Service) saveMailAccountRecord(ctx context.Context, account mail.Account) error {
	client := service.centralPlane()
	if client == nil {
		return errNoCompanyDirectory
	}
	account = mail.NormalizeAccount(account)
	if errorValue := client.WriteMailAccount(ctx, account.ActorEmail, recordAccountOf(account)); errorValue != nil {
		return errorValue
	}
	return service.clearMailCache(ctx, account.ActorEmail)
}

func accountOfRecord(actorEmail string, held centralplane.MailAccount) mail.Account {
	return mail.Account{
		ActorEmail:     actorEmail,
		Email:          held.Email,
		FromAddress:    held.FromAddress,
		DisplayName:    held.DisplayName,
		IMAPHost:       held.IMAPHost,
		IMAPPort:       held.IMAPPort,
		IMAPSecurity:   held.IMAPSecurity,
		IMAPUsername:   held.IMAPUsername,
		IMAPPassword:   held.IMAPPassword,
		SMTPHost:       held.SMTPHost,
		SMTPPort:       held.SMTPPort,
		SMTPSecurity:   held.SMTPSecurity,
		SMTPUsername:   held.SMTPUsername,
		SMTPPassword:   held.SMTPPassword,
		DefaultMailbox: held.DefaultMailbox,
		SentMailbox:    held.SentMailbox,

		SealedIMAPPassword: held.SealedIMAPPassword,
		SealedSMTPPassword: held.SealedSMTPPassword,
	}
}

func recordAccountOf(account mail.Account) centralplane.MailAccount {
	return centralplane.MailAccount{
		ActorEmail:     account.ActorEmail,
		Email:          account.Email,
		FromAddress:    account.FromAddress,
		DisplayName:    account.DisplayName,
		IMAPHost:       account.IMAPHost,
		IMAPPort:       account.IMAPPort,
		IMAPSecurity:   account.IMAPSecurity,
		IMAPUsername:   account.IMAPUsername,
		IMAPPassword:   account.IMAPPassword,
		SMTPHost:       account.SMTPHost,
		SMTPPort:       account.SMTPPort,
		SMTPSecurity:   account.SMTPSecurity,
		SMTPUsername:   account.SMTPUsername,
		SMTPPassword:   account.SMTPPassword,
		DefaultMailbox: account.DefaultMailbox,
		SentMailbox:    account.SentMailbox,
	}
}
