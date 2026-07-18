package admind

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
)

type remoteCalendarAccount struct {
	ID                              string `json:"id"`
	Provider                        string `json:"provider"`
	AccountEmail                    string `json:"accountEmail"`
	PrincipalURL                    string `json:"principalURL"`
	HomeSetURL                      string `json:"homeSetURL"`
	DefaultCalendarURL              string `json:"defaultCalendarURL"`
	DefaultCalendarCTag             string `json:"defaultCalendarCTag"`
	SelectedCalendarID              string `json:"selectedCalendarID,omitempty"`
	SelectedCalendarSummary         string `json:"selectedCalendarSummary,omitempty"`
	SelectedCalendarAccessRole      string `json:"selectedCalendarAccessRole,omitempty"`
	SelectedCalendarURL             string `json:"selectedCalendarURL,omitempty"`
	SelectedCalendarSelectedAt      string `json:"selectedCalendarSelectedAt,omitempty"`
	SelectedCalendarReadinessStatus string `json:"selectedCalendarReadinessStatus,omitempty"`
	InitialSyncCompletedAt          string `json:"initialSyncCompletedAt,omitempty"`
	TokenFilePath                   string `json:"-"`
	LastAuthError                   string `json:"lastAuthError,omitempty"`
	LastAuthErrorAt                 string `json:"lastAuthErrorAt,omitempty"`
	CreatedAt                       string `json:"createdAt"`
	UpdatedAt                       string `json:"updatedAt"`
}

func (service *Service) upsertRemoteCalendarAccount(ctx context.Context, account remoteCalendarAccount) (remoteCalendarAccount, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return remoteCalendarAccount{}, errorValue
	}
	defer database.Close()
	return upsertRemoteCalendarAccountWithRunner(ctx, database, account)
}

func upsertRemoteCalendarAccountWithRunner(ctx context.Context, queryRunner calendarSQLRunner, account remoteCalendarAccount) (remoteCalendarAccount, error) {
	if strings.TrimSpace(account.ID) == "" {
		return remoteCalendarAccount{}, errors.New("remote calendar account id is required")
	}
	if strings.TrimSpace(account.Provider) == "" {
		return remoteCalendarAccount{}, errors.New("remote calendar account provider is required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if account.CreatedAt == "" {
		account.CreatedAt = now
	}
	account.UpdatedAt = now
	_, errorValue := queryRunner.ExecContext(ctx, `
INSERT INTO calendar_remote_accounts (
	id, provider, account_email, principal_url, home_set_url, default_calendar_url, default_calendar_ctag, selected_calendar_id, selected_calendar_summary, selected_calendar_access_role, selected_calendar_url, selected_calendar_selected_at, selected_calendar_readiness_status, initial_sync_completed_at, token_file_path, last_auth_error, last_auth_error_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	provider = excluded.provider,
	account_email = excluded.account_email,
	principal_url = excluded.principal_url,
	home_set_url = excluded.home_set_url,
	default_calendar_url = excluded.default_calendar_url,
	default_calendar_ctag = excluded.default_calendar_ctag,
	selected_calendar_id = excluded.selected_calendar_id,
	selected_calendar_summary = excluded.selected_calendar_summary,
	selected_calendar_access_role = excluded.selected_calendar_access_role,
	selected_calendar_url = excluded.selected_calendar_url,
	selected_calendar_selected_at = excluded.selected_calendar_selected_at,
	selected_calendar_readiness_status = excluded.selected_calendar_readiness_status,
	initial_sync_completed_at = excluded.initial_sync_completed_at,
	token_file_path = excluded.token_file_path,
	last_auth_error = excluded.last_auth_error,
	last_auth_error_at = excluded.last_auth_error_at,
	updated_at = excluded.updated_at`,
		account.ID,
		account.Provider,
		account.AccountEmail,
		account.PrincipalURL,
		account.HomeSetURL,
		account.DefaultCalendarURL,
		account.DefaultCalendarCTag,
		account.SelectedCalendarID,
		account.SelectedCalendarSummary,
		account.SelectedCalendarAccessRole,
		account.SelectedCalendarURL,
		account.SelectedCalendarSelectedAt,
		account.SelectedCalendarReadinessStatus,
		account.InitialSyncCompletedAt,
		account.TokenFilePath,
		account.LastAuthError,
		account.LastAuthErrorAt,
		account.CreatedAt,
		account.UpdatedAt,
	)
	if errorValue != nil {
		return remoteCalendarAccount{}, errorValue
	}
	return account, nil
}

func (service *Service) readRemoteCalendarAccountByProvider(ctx context.Context, provider string) (remoteCalendarAccount, bool, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return remoteCalendarAccount{}, false, errorValue
	}
	defer database.Close()
	return readRemoteCalendarAccountByProviderWithRunner(ctx, database, provider)
}

func readRemoteCalendarAccountByProviderWithRunner(ctx context.Context, queryRunner calendarSQLRunner, provider string) (remoteCalendarAccount, bool, error) {
	rows, errorValue := queryRunner.QueryContext(ctx, `
SELECT id, provider, account_email, principal_url, home_set_url, default_calendar_url, default_calendar_ctag, selected_calendar_id, selected_calendar_summary, selected_calendar_access_role, selected_calendar_url, selected_calendar_selected_at, selected_calendar_readiness_status, initial_sync_completed_at, token_file_path, last_auth_error, last_auth_error_at, created_at, updated_at
FROM calendar_remote_accounts
WHERE provider = ?
ORDER BY updated_at DESC
LIMIT 1`, strings.TrimSpace(provider))
	if errorValue != nil {
		return remoteCalendarAccount{}, false, errorValue
	}
	defer rows.Close()
	if !rows.Next() {
		return remoteCalendarAccount{}, false, rows.Err()
	}
	account, errorValue := scanRemoteCalendarAccount(rows)
	if errorValue != nil {
		return remoteCalendarAccount{}, false, errorValue
	}
	return account, true, nil
}

func (service *Service) listRemoteCalendarAccountsByProvider(ctx context.Context, provider string) ([]remoteCalendarAccount, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, provider, account_email, principal_url, home_set_url, default_calendar_url, default_calendar_ctag, selected_calendar_id, selected_calendar_summary, selected_calendar_access_role, selected_calendar_url, selected_calendar_selected_at, selected_calendar_readiness_status, initial_sync_completed_at, token_file_path, last_auth_error, last_auth_error_at, created_at, updated_at
FROM calendar_remote_accounts
WHERE provider = ?
ORDER BY updated_at DESC`, strings.TrimSpace(provider))
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	accounts := []remoteCalendarAccount{}
	for rows.Next() {
		account, errorValue := scanRemoteCalendarAccount(rows)
		if errorValue != nil {
			return nil, errorValue
		}
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

func (service *Service) markRemoteCalendarAccountAuthError(ctx context.Context, account remoteCalendarAccount, authError error) {
	if authError == nil {
		return
	}
	if errorValue := service.updateRemoteCalendarAccountAuthState(ctx, account.ID, authError.Error(), time.Now().UTC().Format(time.RFC3339Nano)); errorValue != nil {
		slog.WarnContext(ctx, "calendar auth error update failed", "account_id", account.ID, "error", errorValue)
	}
}

func (service *Service) clearRemoteCalendarAccountAuthError(ctx context.Context, account remoteCalendarAccount) {
	if account.LastAuthError == "" {
		return
	}
	if errorValue := service.clearRemoteCalendarAccountAuthStateIfUnchanged(ctx, account); errorValue != nil {
		slog.WarnContext(ctx, "calendar auth error clear failed", "account_id", account.ID, "error", errorValue)
	}
}

func (service *Service) deleteRemoteCalendarAccount(ctx context.Context, accountID string) error {
	service.calendarRemoteMutex.Lock()
	defer service.calendarRemoteMutex.Unlock()
	return service.deleteRemoteCalendarAccountLocked(ctx, accountID)
}

func (service *Service) deleteRemoteCalendarAccountLocked(ctx context.Context, accountID string) error {
	service.calendarStoreWriteMutex.Lock()
	defer service.calendarStoreWriteMutex.Unlock()
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := deleteRemoteCalendarAccountWithRunner(ctx, transaction, accountID); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	return transaction.Commit()
}

func deleteRemoteCalendarAccountWithRunner(ctx context.Context, queryRunner calendarSQLRunner, accountID string) error {
	trimmedAccountID := strings.TrimSpace(accountID)
	if _, errorValue := queryRunner.ExecContext(ctx, "DELETE FROM calendar_outbox WHERE account_id = ?", trimmedAccountID); errorValue != nil {
		return errorValue
	}
	if _, errorValue := queryRunner.ExecContext(ctx, "DELETE FROM calendar_sync_state WHERE account_id = ?", trimmedAccountID); errorValue != nil {
		return errorValue
	}
	if _, errorValue := queryRunner.ExecContext(ctx, "DELETE FROM calendar_remote_event_sync_state WHERE account_id = ?", trimmedAccountID); errorValue != nil {
		return errorValue
	}
	if _, errorValue := queryRunner.ExecContext(ctx, "DELETE FROM calendar_target_field_acknowledgements WHERE account_id = ?", trimmedAccountID); errorValue != nil {
		return errorValue
	}
	if _, errorValue := queryRunner.ExecContext(ctx, "DELETE FROM calendar_push_observation_fences WHERE account_id = ?", trimmedAccountID); errorValue != nil {
		return errorValue
	}
	if _, errorValue := queryRunner.ExecContext(ctx, "DELETE FROM calendar_remote_accounts WHERE id = ?", trimmedAccountID); errorValue != nil {
		return errorValue
	}
	return nil
}

func scanRemoteCalendarAccount(scanner calendarEventScanner) (remoteCalendarAccount, error) {
	var account remoteCalendarAccount
	errorValue := scanner.Scan(
		&account.ID,
		&account.Provider,
		&account.AccountEmail,
		&account.PrincipalURL,
		&account.HomeSetURL,
		&account.DefaultCalendarURL,
		&account.DefaultCalendarCTag,
		&account.SelectedCalendarID,
		&account.SelectedCalendarSummary,
		&account.SelectedCalendarAccessRole,
		&account.SelectedCalendarURL,
		&account.SelectedCalendarSelectedAt,
		&account.SelectedCalendarReadinessStatus,
		&account.InitialSyncCompletedAt,
		&account.TokenFilePath,
		&account.LastAuthError,
		&account.LastAuthErrorAt,
		&account.CreatedAt,
		&account.UpdatedAt,
	)
	return account, errorValue
}
