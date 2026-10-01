package admind

import (
	"context"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/mail"
)

func (service *Service) saveCachedMailMessages(ctx context.Context, actorEmail string, input mail.MessageListRequest, result mail.MessageListResponse) error {
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
	mailbox := strings.TrimSpace(input.Mailbox)
	query := strings.TrimSpace(input.Query)
	updatedAt := time.Now().UTC().Format(time.RFC3339Nano)
	_, errorValue = transaction.ExecContext(ctx, `
	INSERT INTO mail_message_list_cache(actor_email, mailbox, query, before_uid, page_limit, next_cursor, updated_at)
	VALUES(?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(actor_email, mailbox, query, before_uid, page_limit) DO UPDATE SET
		next_cursor = excluded.next_cursor,
		updated_at = excluded.updated_at`,
		normalizedActorEmail,
		mailbox,
		query,
		input.BeforeUID,
		input.Limit,
		result.NextCursor,
		updatedAt,
	)
	if errorValue != nil {
		return rollbackMailCacheTransaction(transaction, errorValue)
	}
	if _, errorValue = transaction.ExecContext(ctx, `
	DELETE FROM mail_message_list_cache_item
	WHERE actor_email = ? AND mailbox = ? AND query = ? AND before_uid = ? AND page_limit = ?`, normalizedActorEmail, mailbox, query, input.BeforeUID, input.Limit); errorValue != nil {
		return rollbackMailCacheTransaction(transaction, errorValue)
	}
	position := 0
	for _, message := range result.Messages {
		if strings.TrimSpace(message.Mailbox) == "" || message.UID == 0 {
			continue
		}
		_, errorValue = transaction.ExecContext(ctx, `
INSERT INTO mail_message_cache(actor_email, mailbox, uid, subject, from_address, to_addresses, cc_addresses, date, preview, body, body_html, is_read, updated_at)
VALUES(?, ?, ?, ?, ?, '', '', ?, ?, '', '', ?, ?)
ON CONFLICT(actor_email, mailbox, uid) DO UPDATE SET
	subject = excluded.subject,
	from_address = excluded.from_address,
	date = excluded.date,
	preview = excluded.preview,
	is_read = excluded.is_read,
	updated_at = excluded.updated_at`,
			normalizedActorEmail,
			strings.TrimSpace(message.Mailbox),
			message.UID,
			message.Subject,
			message.From,
			message.Date,
			message.Preview,
			boolInteger(message.IsRead),
			updatedAt,
		)
		if errorValue != nil {
			return rollbackMailCacheTransaction(transaction, errorValue)
		}
		_, errorValue = transaction.ExecContext(ctx, `
		INSERT INTO mail_message_list_cache_item(actor_email, mailbox, query, before_uid, page_limit, uid, position, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
			normalizedActorEmail,
			mailbox,
			query,
			input.BeforeUID,
			input.Limit,
			message.UID,
			position,
			updatedAt,
		)
		if errorValue != nil {
			return rollbackMailCacheTransaction(transaction, errorValue)
		}
		position++
	}
	return transaction.Commit()
}
