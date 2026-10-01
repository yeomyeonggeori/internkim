package admind

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/mail"
)

const mailAccountCarryRecoveryAction = "mail-account-carry-into-the-record"
const mailAccountCarryTimeout = 30 * time.Second

// A mail account a device recorded before the record held one.
type heldMailAccount struct {
	ActorEmail string
	Account    mail.Account
}

func heldMailAccounts(ctx context.Context, database *sql.DB) ([]heldMailAccount, error) {
	if _, held := countRowsInTable(ctx, database, "mail_accounts"); !held {
		return nil, nil
	}
	rows, errorValue := database.QueryContext(ctx, `
SELECT actor_email, email, from_address, display_name, imap_host, imap_port, imap_security,
	imap_username, imap_password, smtp_host, smtp_port, smtp_security, smtp_username,
	smtp_password, default_mailbox, sent_mailbox
FROM mail_accounts ORDER BY actor_email`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	accounts := []heldMailAccount{}
	for rows.Next() {
		var held heldMailAccount
		if errorValue := rows.Scan(&held.ActorEmail, &held.Account.Email, &held.Account.FromAddress,
			&held.Account.DisplayName, &held.Account.IMAPHost, &held.Account.IMAPPort,
			&held.Account.IMAPSecurity, &held.Account.IMAPUsername, &held.Account.IMAPPassword,
			&held.Account.SMTPHost, &held.Account.SMTPPort, &held.Account.SMTPSecurity,
			&held.Account.SMTPUsername, &held.Account.SMTPPassword, &held.Account.DefaultMailbox,
			&held.Account.SentMailbox); errorValue != nil {
			return nil, errorValue
		}
		held.Account.ActorEmail = held.ActorEmail
		accounts = append(accounts, held)
	}
	return accounts, rows.Err()
}

type mailAccountCarryReport struct {
	Accounts int      `json:"accounts"`
	Refused  []string `json:"refused"`
}

// A row the record refuses is reported, not reshaped: it stays here, it is
// named, and the table stays with it.
func (service *Service) carryMailAccountsIntoTheRecord(ctx context.Context) (mailAccountCarryReport, error) {
	report := mailAccountCarryReport{Refused: []string{}}
	client := service.centralPlane()
	if client == nil {
		return report, fmt.Errorf("this device names no company to carry its mail accounts into")
	}
	database, errorValue := service.openMailDatabase(ctx)
	if errorValue != nil {
		return report, errorValue
	}
	defer database.Close()

	accounts, errorValue := heldMailAccounts(ctx, database)
	if errorValue != nil {
		return report, errorValue
	}
	for _, held := range accounts {
		actorEmail := strings.ToLower(strings.TrimSpace(held.ActorEmail))
		if actorEmail == "" {
			report.Refused = append(report.Refused, "a mail account names nobody it belongs to")
			continue
		}
		carryContext, cancel := context.WithTimeout(ctx, mailAccountCarryTimeout)
		errorValue := client.WriteMailAccount(carryContext, actorEmail,
			recordAccountOf(mail.NormalizeAccount(held.Account)))
		cancel()
		if errorValue != nil {
			report.Refused = append(report.Refused, actorEmail+": "+errorValue.Error())
			continue
		}
		report.Accounts++
	}
	return report, nil
}

// A device that names no company covers nothing, so it keeps what it holds.
// Reading this the other way round is how somebody's mail password gets
// dropped by a device the record has never heard of.
func (service *Service) sweepTheMailAccountsTheRecordNowHolds(ctx context.Context) {
	database, errorValue := service.openMailDatabase(ctx)
	if errorValue != nil {
		return
	}
	defer database.Close()

	accounts, errorValue := heldMailAccounts(ctx, database)
	if errorValue != nil {
		slog.WarnContext(ctx, "the mail accounts this device still holds could not be counted, so none were let go",
			"error", errorValue)
		return
	}
	if len(accounts) == 0 {
		return
	}
	if service.centralPlane() == nil {
		slog.WarnContext(ctx, "this device names no company, so it keeps the mail accounts nobody else holds",
			"accounts", len(accounts), "recovery_action", mailAccountCarryRecoveryAction)
		return
	}
	report, errorValue := service.carryMailAccountsIntoTheRecord(ctx)
	if errorValue != nil || len(report.Refused) > 0 {
		slog.WarnContext(ctx, "the mail accounts this device holds were not all taken, so the table stays",
			"carried", report.Accounts, "refused", strings.Join(report.Refused, "; "),
			"error", errorValue, "recovery_action", mailAccountCarryRecoveryAction)
		return
	}
	if _, errorValue := database.ExecContext(ctx, "DROP TABLE IF EXISTS mail_accounts"); errorValue != nil {
		slog.WarnContext(ctx, "the mail accounts the record now holds could not be dropped", "error", errorValue)
		return
	}
	slog.InfoContext(ctx, "the record holds the mail accounts this device was keeping",
		"accounts", report.Accounts)
}

func (service *Service) startMailAccountSweep(ctx context.Context) {
	go service.sweepTheMailAccountsTheRecordNowHolds(ctx)
}

func (service *Service) handleMailAccountCarry(responseWriter http.ResponseWriter, request *http.Request) {
	report, errorValue := service.carryMailAccountsIntoTheRecord(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.sweepTheMailAccountsTheRecordNowHolds(request.Context())
	service.writeJSON(responseWriter, report)
}
