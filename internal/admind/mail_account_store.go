package admind

import (
	"context"
	"database/sql"

	"github.com/yeomyeonggeori/internkim/internal/mail"
)

func (service *Service) openMailDatabase(ctx context.Context) (*sql.DB, error) {
	return service.openStateDatabase(ctx, "mail", ensureMailSchema, sqliteDatabaseOptions{})
}

func ensureMailSchema(ctx context.Context, database *sql.DB) error {
	_, errorValue := database.ExecContext(ctx, `
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
	return mail.EnsureMessageListCacheSchema(ctx, database)
}
