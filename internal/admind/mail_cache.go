package admind

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

func (service *Service) clearMailCache(ctx context.Context, actorEmail string) error {
	database, errorValue := service.openMailDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	normalizedActorEmail := strings.ToLower(strings.TrimSpace(actorEmail))
	if _, errorValue := database.ExecContext(ctx, `DELETE FROM mail_mailbox_cache WHERE actor_email = ?`, normalizedActorEmail); errorValue != nil {
		return errorValue
	}
	if _, errorValue := database.ExecContext(ctx, `DELETE FROM mail_message_list_cache_item WHERE actor_email = ?`, normalizedActorEmail); errorValue != nil {
		return errorValue
	}
	if _, errorValue := database.ExecContext(ctx, `DELETE FROM mail_message_list_cache WHERE actor_email = ?`, normalizedActorEmail); errorValue != nil {
		return errorValue
	}
	_, errorValue = database.ExecContext(ctx, `DELETE FROM mail_message_cache WHERE actor_email = ?`, normalizedActorEmail)
	return errorValue
}

func boolInteger(value bool) int {
	if value {
		return 1
	}
	return 0
}

func rollbackMailCacheTransaction(transaction *sql.Tx, errorValue error) error {
	if rollbackError := transaction.Rollback(); rollbackError != nil && !errors.Is(rollbackError, sql.ErrTxDone) {
		return rollbackError
	}
	return errorValue
}
