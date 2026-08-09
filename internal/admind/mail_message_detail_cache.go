package admind

import (
	"context"
	"gitlab.com/eastriver/internkim/internal/mail"
	"strings"
	"time"
)

func (service *Service) saveCachedMailMessageDetail(ctx context.Context, actorEmail string, message mail.MessageDetailResponse) error {
	if strings.TrimSpace(message.Mailbox) == "" || message.UID == 0 {
		return nil
	}
	database, errorValue := service.openMailDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, `
INSERT INTO mail_message_cache(actor_email, mailbox, uid, subject, from_address, to_addresses, cc_addresses, date, preview, body, body_html, is_read, updated_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, '', ?, ?, ?, ?)
ON CONFLICT(actor_email, mailbox, uid) DO UPDATE SET
	subject = excluded.subject,
	from_address = excluded.from_address,
	to_addresses = excluded.to_addresses,
	cc_addresses = excluded.cc_addresses,
	date = excluded.date,
	body = excluded.body,
	body_html = excluded.body_html,
	is_read = excluded.is_read,
	updated_at = excluded.updated_at`,
		strings.ToLower(strings.TrimSpace(actorEmail)),
		strings.TrimSpace(message.Mailbox),
		message.UID,
		message.Subject,
		message.From,
		message.To,
		message.CC,
		message.Date,
		message.Body,
		message.BodyHTML,
		boolInteger(message.IsRead),
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	return errorValue
}

func (service *Service) deleteCachedMailMessage(ctx context.Context, actorEmail string, mailbox string, uid uint32) error {
	database, errorValue := service.openMailDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	normalizedActorEmail := strings.ToLower(strings.TrimSpace(actorEmail))
	normalizedMailbox := strings.TrimSpace(mailbox)
	if _, errorValue = transaction.ExecContext(ctx, `
DELETE FROM mail_message_list_cache_item
WHERE actor_email = ? AND mailbox = ? AND uid = ?`, normalizedActorEmail, normalizedMailbox, uid); errorValue != nil {
		return rollbackMailCacheTransaction(transaction, errorValue)
	}
	if _, errorValue = transaction.ExecContext(ctx, `
DELETE FROM mail_message_cache
WHERE actor_email = ? AND mailbox = ? AND uid = ?`, normalizedActorEmail, normalizedMailbox, uid); errorValue != nil {
		return rollbackMailCacheTransaction(transaction, errorValue)
	}
	return transaction.Commit()
}

func (service *Service) updateCachedMailMessageFlags(ctx context.Context, actorEmail string, mailbox string, uid uint32, input mail.MessageMarkRequest) error {
	if input.Seen == nil {
		return nil
	}
	database, errorValue := service.openMailDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, `
UPDATE mail_message_cache
SET is_read = ?, updated_at = ?
WHERE actor_email = ? AND mailbox = ? AND uid = ?`,
		boolInteger(*input.Seen),
		time.Now().UTC().Format(time.RFC3339Nano),
		strings.ToLower(strings.TrimSpace(actorEmail)),
		strings.TrimSpace(mailbox),
		uid,
	)
	return errorValue
}
