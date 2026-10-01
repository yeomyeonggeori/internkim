package admind

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/mail"
)

func (service *Service) readCachedMailMessages(ctx context.Context, actorEmail string, input mail.MessageListRequest) (mail.MessageListResponse, bool, error) {
	database, errorValue := service.openMailDatabase(ctx)
	if errorValue != nil {
		return mail.MessageListResponse{}, false, errorValue
	}
	defer database.Close()
	normalizedActorEmail := strings.ToLower(strings.TrimSpace(actorEmail))
	nextCursor, hasCachedList, errorValue := readCachedMailMessageListCursor(ctx, database, normalizedActorEmail, input)
	if errorValue != nil {
		return mail.MessageListResponse{}, false, errorValue
	}
	if !hasCachedList {
		return mail.MessageListResponse{}, false, nil
	}
	messages, errorValue := queryCachedMailMessagePageSnapshot(ctx, database, normalizedActorEmail, input)
	if errorValue != nil {
		return mail.MessageListResponse{}, true, errorValue
	}
	return mail.MessageListResponse{
		Messages:   messages,
		NextCursor: nextCursor,
	}, true, nil
}

func readCachedMailMessageListCursor(ctx context.Context, database *sql.DB, actorEmail string, input mail.MessageListRequest) (string, bool, error) {
	row := database.QueryRowContext(ctx, `
	SELECT next_cursor
	FROM mail_message_list_cache
	WHERE actor_email = ? AND mailbox = ? AND query = ? AND before_uid = ? AND page_limit = ?`, actorEmail, strings.TrimSpace(input.Mailbox), strings.TrimSpace(input.Query), input.BeforeUID, input.Limit)
	var nextCursor string
	errorValue := row.Scan(&nextCursor)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return "", false, nil
	}
	if errorValue != nil {
		return "", false, errorValue
	}
	return nextCursor, true, nil
}

func queryCachedMailMessagePageSnapshot(ctx context.Context, database *sql.DB, actorEmail string, input mail.MessageListRequest) ([]mail.MessageResponse, error) {
	rows, errorValue := database.QueryContext(ctx, `
SELECT message.uid, message.mailbox, message.subject, message.from_address, message.date, message.preview, message.is_read
	FROM mail_message_list_cache_item item
	JOIN mail_message_cache message ON message.actor_email = item.actor_email AND message.mailbox = item.mailbox AND message.uid = item.uid
	WHERE item.actor_email = ? AND item.mailbox = ? AND item.query = ? AND item.before_uid = ? AND item.page_limit = ?
	ORDER BY item.position`, actorEmail, strings.TrimSpace(input.Mailbox), strings.TrimSpace(input.Query), input.BeforeUID, input.Limit)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	messages := []mail.MessageResponse{}
	for rows.Next() {
		var message mail.MessageResponse
		var isRead int
		if errorValue := rows.Scan(&message.UID, &message.Mailbox, &message.Subject, &message.From, &message.Date, &message.Preview, &isRead); errorValue != nil {
			return nil, errorValue
		}
		message.IsRead = isRead != 0
		messages = append(messages, message)
	}
	return messages, rows.Err()
}
