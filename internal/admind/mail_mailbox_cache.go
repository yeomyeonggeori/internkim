package admind

import (
	"context"
	"gitlab.com/eastriver/internkim/internal/mail"
	"sort"
	"strings"
	"time"
)

func (service *Service) readCachedMailboxes(ctx context.Context, actorEmail string) ([]mail.MailboxResponse, bool, error) {
	database, errorValue := service.openMailDatabase(ctx)
	if errorValue != nil {
		return nil, false, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT name, display_name, unseen, total
FROM mail_mailbox_cache
WHERE actor_email = ?
ORDER BY name`, strings.ToLower(strings.TrimSpace(actorEmail)))
	if errorValue != nil {
		return nil, false, errorValue
	}
	defer rows.Close()
	mailboxes := []mail.MailboxResponse{}
	for rows.Next() {
		var mailbox mail.MailboxResponse
		if errorValue := rows.Scan(&mailbox.Name, &mailbox.DisplayName, &mailbox.Unseen, &mailbox.Total); errorValue != nil {
			return nil, false, errorValue
		}
		mailboxes = append(mailboxes, mailbox)
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, false, errorValue
	}
	sort.SliceStable(mailboxes, func(firstIndex int, secondIndex int) bool {
		return mail.MailboxSortKey(mailboxes[firstIndex].Name) < mail.MailboxSortKey(mailboxes[secondIndex].Name)
	})
	return mailboxes, len(mailboxes) > 0, nil
}

func (service *Service) saveCachedMailboxes(ctx context.Context, actorEmail string, mailboxes []mail.MailboxResponse) error {
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
	if _, errorValue = transaction.ExecContext(ctx, `DELETE FROM mail_mailbox_cache WHERE actor_email = ?`, normalizedActorEmail); errorValue != nil {
		return rollbackMailCacheTransaction(transaction, errorValue)
	}
	updatedAt := time.Now().UTC().Format(time.RFC3339Nano)
	for _, mailbox := range mailboxes {
		if strings.TrimSpace(mailbox.Name) == "" {
			continue
		}
		_, errorValue = transaction.ExecContext(ctx, `
INSERT INTO mail_mailbox_cache(actor_email, name, display_name, unseen, total, updated_at)
VALUES(?, ?, ?, ?, ?, ?)`,
			normalizedActorEmail,
			strings.TrimSpace(mailbox.Name),
			firstNonEmpty(strings.TrimSpace(mailbox.DisplayName), strings.TrimSpace(mailbox.Name)),
			mailbox.Unseen,
			mailbox.Total,
			updatedAt,
		)
		if errorValue != nil {
			return rollbackMailCacheTransaction(transaction, errorValue)
		}
	}
	return transaction.Commit()
}
