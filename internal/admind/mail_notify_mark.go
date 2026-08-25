package admind

import (
	"context"
	"database/sql"
)

// The newest message this device has already told somebody about. Without it a
// poller announces the whole inbox every time it wakes.
func ensureMailNotifyMarkTable(ctx context.Context, database *sql.DB) error {
	_, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS mail_notify_mark (
	actor_email TEXT PRIMARY KEY,
	seen_up_to INTEGER NOT NULL
)`)
	return errorValue
}

func (service *Service) openMailNotifyDatabase(ctx context.Context) (*sql.DB, error) {
	options := sqliteDatabaseOptions{transactionLock: "immediate"}
	return service.openStateDatabase(ctx, "mail-notify", ensureMailNotifyMarkTable, options)
}

func (service *Service) readMailNotifyMark(ctx context.Context, actorEmail string) (uint32, bool, error) {
	database, errorValue := service.openMailNotifyDatabase(ctx)
	if errorValue != nil {
		return 0, false, errorValue
	}
	defer database.Close()

	seenUpTo := uint32(0)
	errorValue = database.QueryRowContext(ctx,
		"SELECT seen_up_to FROM mail_notify_mark WHERE actor_email = ?", actorEmail).Scan(&seenUpTo)
	if errorValue == sql.ErrNoRows {
		return 0, false, nil
	}
	return seenUpTo, errorValue == nil, errorValue
}

func (service *Service) writeMailNotifyMark(ctx context.Context, actorEmail string, seenUpTo uint32) error {
	database, errorValue := service.openMailNotifyDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()

	_, errorValue = database.ExecContext(ctx, `
INSERT INTO mail_notify_mark (actor_email, seen_up_to) VALUES (?, ?)
ON CONFLICT (actor_email) DO UPDATE SET seen_up_to = excluded.seen_up_to`, actorEmail, seenUpTo)
	return errorValue
}

func (service *Service) mailNotifyActorEmails(ctx context.Context) ([]string, error) {
	database, errorValue := service.openMailDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()

	rows, errorValue := database.QueryContext(ctx,
		"SELECT actor_email FROM mail_accounts WHERE imap_host <> '' ORDER BY actor_email")
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()

	actorEmails := []string{}
	for rows.Next() {
		actorEmail := ""
		if errorValue := rows.Scan(&actorEmail); errorValue != nil {
			return nil, errorValue
		}
		actorEmails = append(actorEmails, actorEmail)
	}
	return actorEmails, rows.Err()
}
