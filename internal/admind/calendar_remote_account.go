package admind

import (
	"context"
	"database/sql"
	"errors"
	"log"
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
	row := database.QueryRowContext(ctx, `
SELECT id, provider, account_email, principal_url, home_set_url, default_calendar_url, default_calendar_ctag, selected_calendar_id, selected_calendar_summary, selected_calendar_access_role, selected_calendar_url, selected_calendar_selected_at, selected_calendar_readiness_status, initial_sync_completed_at, token_file_path, last_auth_error, last_auth_error_at, created_at, updated_at
FROM calendar_remote_accounts
WHERE provider = ?
ORDER BY updated_at DESC
LIMIT 1`, strings.TrimSpace(provider))
	account, errorValue := scanRemoteCalendarAccount(row)
	if errorValue == nil {
		return account, true, nil
	}
	if errors.Is(errorValue, sql.ErrNoRows) {
		return remoteCalendarAccount{}, false, nil
	}
	return remoteCalendarAccount{}, false, errorValue
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
	account.LastAuthError = authError.Error()
	account.LastAuthErrorAt = time.Now().UTC().Format(time.RFC3339Nano)
	if _, errorValue := service.upsertRemoteCalendarAccount(ctx, account); errorValue != nil {
		log.Printf("mark calendar auth error: %v", errorValue)
	}
}

func (service *Service) clearRemoteCalendarAccountAuthError(ctx context.Context, account remoteCalendarAccount) {
	if account.LastAuthError == "" {
		return
	}
	account.LastAuthError = ""
	account.LastAuthErrorAt = ""
	if _, errorValue := service.upsertRemoteCalendarAccount(ctx, account); errorValue != nil {
		log.Printf("clear calendar auth error: %v", errorValue)
	}
}

func (service *Service) deleteRemoteCalendarAccount(ctx context.Context, accountID string) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx,
		"DELETE FROM calendar_remote_accounts WHERE id = ?", strings.TrimSpace(accountID))
	return errorValue
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
