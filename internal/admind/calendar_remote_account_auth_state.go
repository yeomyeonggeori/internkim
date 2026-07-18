package admind

import (
	"context"
	"strings"
	"time"
)

func (service *Service) updateRemoteCalendarAccountAuthState(ctx context.Context, accountID string, authError string, authErrorAt string) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, `
UPDATE calendar_remote_accounts
SET last_auth_error = ?, last_auth_error_at = ?, updated_at = ?
WHERE id = ?`,
		strings.TrimSpace(authError),
		strings.TrimSpace(authErrorAt),
		time.Now().UTC().Format(time.RFC3339Nano),
		strings.TrimSpace(accountID),
	)
	return errorValue
}

func (service *Service) clearRemoteCalendarAccountAuthStateIfUnchanged(ctx context.Context, account remoteCalendarAccount) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, `
UPDATE calendar_remote_accounts
SET last_auth_error = '', last_auth_error_at = '', updated_at = ?
WHERE id = ?
	AND last_auth_error = ?
	AND last_auth_error_at = ?`,
		time.Now().UTC().Format(time.RFC3339Nano),
		strings.TrimSpace(account.ID),
		strings.TrimSpace(account.LastAuthError),
		strings.TrimSpace(account.LastAuthErrorAt),
	)
	return errorValue
}
