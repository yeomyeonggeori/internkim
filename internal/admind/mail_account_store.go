package admind

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func (service *Service) openMailDatabase(ctx context.Context) (*sql.DB, error) {
	return service.openStateDatabase(ctx, "mail", ensureMailSchema, sqliteDatabaseOptions{})
}

func ensureMailSchema(ctx context.Context, database *sql.DB) error {
	_, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS mail_accounts (
	actor_email TEXT PRIMARY KEY,
	email TEXT NOT NULL,
	from_address TEXT NOT NULL,
	display_name TEXT NOT NULL,
	imap_host TEXT NOT NULL,
	imap_port INTEGER NOT NULL,
	imap_security TEXT NOT NULL,
	imap_username TEXT NOT NULL,
	imap_password TEXT NOT NULL,
	smtp_host TEXT NOT NULL,
	smtp_port INTEGER NOT NULL,
	smtp_security TEXT NOT NULL,
	smtp_username TEXT NOT NULL,
	smtp_password TEXT NOT NULL,
	default_mailbox TEXT NOT NULL,
	sent_mailbox TEXT NOT NULL,
	updated_at TEXT NOT NULL
)`)
	if errorValue != nil {
		return errorValue
	}
	_, errorValue = database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS mail_mailbox_cache (
	actor_email TEXT NOT NULL,
	name TEXT NOT NULL,
	display_name TEXT NOT NULL,
	unseen INTEGER NOT NULL,
	total INTEGER NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY(actor_email, name)
);
CREATE TABLE IF NOT EXISTS mail_message_cache (
	actor_email TEXT NOT NULL,
	mailbox TEXT NOT NULL,
	uid INTEGER NOT NULL,
	subject TEXT NOT NULL,
	from_address TEXT NOT NULL,
	to_addresses TEXT NOT NULL,
	cc_addresses TEXT NOT NULL,
	date TEXT NOT NULL,
	preview TEXT NOT NULL,
	body TEXT NOT NULL,
	body_html TEXT NOT NULL,
	is_read INTEGER NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY(actor_email, mailbox, uid)
)`)
	if errorValue != nil {
		return errorValue
	}
	return ensureMailMessageListCacheSchema(ctx, database)
}

func (service *Service) readMailAccount(ctx context.Context, actorEmail string) (mailAccount, bool, error) {
	account := defaultMailAccount(actorEmail)
	database, errorValue := service.openMailDatabase(ctx)
	if errorValue != nil {
		return mailAccount{}, false, errorValue
	}
	defer database.Close()
	row := database.QueryRowContext(ctx, `
SELECT actor_email, email, from_address, display_name, imap_host, imap_port, imap_security, imap_username, imap_password, smtp_host, smtp_port, smtp_security, smtp_username, smtp_password, default_mailbox, sent_mailbox, updated_at
FROM mail_accounts
WHERE actor_email = ?`, actorEmail)
	errorValue = row.Scan(
		&account.ActorEmail,
		&account.Email,
		&account.FromAddress,
		&account.DisplayName,
		&account.IMAPHost,
		&account.IMAPPort,
		&account.IMAPSecurity,
		&account.IMAPUsername,
		&account.IMAPPassword,
		&account.SMTPHost,
		&account.SMTPPort,
		&account.SMTPSecurity,
		&account.SMTPUsername,
		&account.SMTPPassword,
		&account.DefaultMailbox,
		&account.SentMailbox,
		&account.UpdatedAt,
	)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return account, false, nil
	}
	if errorValue != nil {
		return mailAccount{}, false, errorValue
	}
	return normalizeMailAccount(account), true, nil
}

func (service *Service) saveMailAccountRecord(ctx context.Context, account mailAccount) error {
	database, errorValue := service.openMailDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	account = normalizeMailAccount(account)
	account.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	_, errorValue = database.ExecContext(ctx, `
INSERT INTO mail_accounts(actor_email, email, from_address, display_name, imap_host, imap_port, imap_security, imap_username, imap_password, smtp_host, smtp_port, smtp_security, smtp_username, smtp_password, default_mailbox, sent_mailbox, updated_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(actor_email) DO UPDATE SET
	email = excluded.email,
	from_address = excluded.from_address,
	display_name = excluded.display_name,
	imap_host = excluded.imap_host,
	imap_port = excluded.imap_port,
	imap_security = excluded.imap_security,
	imap_username = excluded.imap_username,
	imap_password = excluded.imap_password,
	smtp_host = excluded.smtp_host,
	smtp_port = excluded.smtp_port,
	smtp_security = excluded.smtp_security,
	smtp_username = excluded.smtp_username,
	smtp_password = excluded.smtp_password,
	default_mailbox = excluded.default_mailbox,
	sent_mailbox = excluded.sent_mailbox,
	updated_at = excluded.updated_at`,
		account.ActorEmail,
		account.Email,
		account.FromAddress,
		account.DisplayName,
		account.IMAPHost,
		account.IMAPPort,
		account.IMAPSecurity,
		account.IMAPUsername,
		account.IMAPPassword,
		account.SMTPHost,
		account.SMTPPort,
		account.SMTPSecurity,
		account.SMTPUsername,
		account.SMTPPassword,
		account.DefaultMailbox,
		account.SentMailbox,
		account.UpdatedAt,
	)
	if errorValue != nil {
		return errorValue
	}
	return service.clearMailCache(ctx, account.ActorEmail)
}
