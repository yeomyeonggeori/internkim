package admind

import (
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"mime"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/charset"
	messagemail "github.com/emersion/go-message/mail"
)

const (
	mailSecurityTLS      = "tls"
	mailSecurityStartTLS = "starttls"
	mailSecurityNone     = "none"
)

type mailAccount struct {
	ActorEmail     string
	Email          string
	FromAddress    string
	DisplayName    string
	IMAPHost       string
	IMAPPort       int
	IMAPSecurity   string
	IMAPUsername   string
	IMAPPassword   string
	SMTPHost       string
	SMTPPort       int
	SMTPSecurity   string
	SMTPUsername   string
	SMTPPassword   string
	DefaultMailbox string
	SentMailbox    string
	UpdatedAt      string
}

type mailAccountResponse struct {
	Email           string `json:"email"`
	FromAddress     string `json:"fromAddress"`
	DisplayName     string `json:"displayName"`
	IMAPHost        string `json:"imapHost"`
	IMAPPort        int    `json:"imapPort"`
	IMAPSecurity    string `json:"imapSecurity"`
	IMAPUsername    string `json:"imapUsername"`
	SMTPHost        string `json:"smtpHost"`
	SMTPPort        int    `json:"smtpPort"`
	SMTPSecurity    string `json:"smtpSecurity"`
	SMTPUsername    string `json:"smtpUsername"`
	DefaultMailbox  string `json:"defaultMailbox"`
	SentMailbox     string `json:"sentMailbox"`
	IsConfigured    bool   `json:"isConfigured"`
	HasIMAPPassword bool   `json:"hasIMAPPassword"`
	HasSMTPPassword bool   `json:"hasSMTPPassword"`
}

type mailAccountWriteRequest struct {
	Email          string `json:"email"`
	FromAddress    string `json:"fromAddress"`
	DisplayName    string `json:"displayName"`
	IMAPHost       string `json:"imapHost"`
	IMAPPort       int    `json:"imapPort"`
	IMAPSecurity   string `json:"imapSecurity"`
	IMAPUsername   string `json:"imapUsername"`
	IMAPPassword   string `json:"imapPassword"`
	SMTPHost       string `json:"smtpHost"`
	SMTPPort       int    `json:"smtpPort"`
	SMTPSecurity   string `json:"smtpSecurity"`
	SMTPUsername   string `json:"smtpUsername"`
	SMTPPassword   string `json:"smtpPassword"`
	DefaultMailbox string `json:"defaultMailbox"`
	SentMailbox    string `json:"sentMailbox"`
}

type mailMailboxResponse struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Unseen      int    `json:"unseen"`
	Total       int    `json:"total"`
}

type mailMessageListRequest struct {
	Mailbox string
	Query   string
	Limit   int
}

type mailMessageResponse struct {
	UID     uint32 `json:"uid"`
	Mailbox string `json:"mailbox"`
	Subject string `json:"subject"`
	From    string `json:"from"`
	Date    string `json:"date"`
	Preview string `json:"preview"`
	IsRead  bool   `json:"isRead"`
}

type mailMessageDetailResponse struct {
	UID      uint32 `json:"uid"`
	Mailbox  string `json:"mailbox"`
	Subject  string `json:"subject"`
	From     string `json:"from"`
	To       string `json:"to"`
	CC       string `json:"cc"`
	Date     string `json:"date"`
	Body     string `json:"body"`
	BodyHTML string `json:"bodyHTML,omitempty"`
	IsRead   bool   `json:"isRead"`
}

type parsedMailDocument struct {
	PlainText string
	HTML      string
}

type mailMessageSendRequest struct {
	To      []string `json:"to"`
	CC      []string `json:"cc"`
	BCC     []string `json:"bcc"`
	Subject string   `json:"subject"`
	Body    string   `json:"body"`
}

type mailMessageMoveRequest struct {
	TargetMailbox string `json:"targetMailbox"`
}

type mailMessageMarkRequest struct {
	Seen    *bool `json:"seen"`
	Flagged *bool `json:"flagged"`
}

type mailSendResult struct {
	Sent          bool   `json:"sent"`
	AppendedTo    string `json:"appendedTo,omitempty"`
	AppendWarning string `json:"appendWarning,omitempty"`
}

type mailBackend interface {
	TestAccount(ctx context.Context, account mailAccount) error
	ListMailboxes(ctx context.Context, account mailAccount) ([]mailMailboxResponse, error)
	ListMessages(ctx context.Context, account mailAccount, input mailMessageListRequest) ([]mailMessageResponse, error)
	ReadMessage(ctx context.Context, account mailAccount, mailbox string, uid uint32) (mailMessageDetailResponse, error)
	SendMessage(ctx context.Context, account mailAccount, input mailMessageSendRequest) (mailSendResult, error)
	MoveMessage(ctx context.Context, account mailAccount, mailbox string, uid uint32, targetMailbox string) error
	MarkMessage(ctx context.Context, account mailAccount, mailbox string, uid uint32, input mailMessageMarkRequest) error
}

type standardMailBackend struct{}

func (service *Service) serveMailPage(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/mail" {
		http.Redirect(responseWriter, request, "/mail/", http.StatusFound)
		return
	}
	if service.serveMailStaticFile(responseWriter, request) {
		return
	}
	service.serveMailIndex(responseWriter, request)
}

func (service *Service) serveMailStaticFile(responseWriter http.ResponseWriter, request *http.Request) bool {
	relativePath := strings.TrimPrefix(request.URL.Path, "/mail/")
	if relativePath == "" {
		return false
	}
	filePath := filepath.Join(service.Configuration.AdminUIPath, "mail", relativePath)
	fileInformation, errorValue := os.Stat(filePath)
	if errorValue != nil || fileInformation.IsDir() {
		return false
	}
	http.ServeFile(responseWriter, request, filePath)
	return true
}

func (service *Service) serveMailIndex(responseWriter http.ResponseWriter, request *http.Request) {
	mailIndexPath := filepath.Join(service.Configuration.AdminUIPath, "mail", "index.html")
	if fileInformation, errorValue := os.Stat(mailIndexPath); errorValue == nil && !fileInformation.IsDir() {
		http.ServeFile(responseWriter, request, mailIndexPath)
		return
	}
	http.ServeFile(responseWriter, request, filepath.Join(service.Configuration.AdminUIPath, "index.html"))
}

func (service *Service) handleMail(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.authorizeMailRequest(request) {
		http.Error(responseWriter, "mail access required", http.StatusForbidden)
		return
	}
	path := strings.TrimPrefix(request.URL.Path, "/mail/api")
	switch {
	case request.Method == http.MethodGet && path == "/account":
		service.writeMailAccount(responseWriter, request)
	case request.Method == http.MethodPut && path == "/account":
		service.saveMailAccount(responseWriter, request)
	case request.Method == http.MethodPost && path == "/account/test":
		service.testMailAccount(responseWriter, request)
	case request.Method == http.MethodGet && path == "/mailboxes":
		service.writeMailboxes(responseWriter, request)
	case request.Method == http.MethodGet && path == "/messages":
		service.writeMailMessages(responseWriter, request)
	case request.Method == http.MethodGet && strings.HasPrefix(path, "/messages/"):
		service.writeMailMessage(responseWriter, request)
	case request.Method == http.MethodPost && path == "/messages/send":
		service.sendMailMessage(responseWriter, request)
	case request.Method == http.MethodPost && strings.HasSuffix(path, "/move"):
		service.moveMailMessage(responseWriter, request)
	case request.Method == http.MethodPost && strings.HasSuffix(path, "/flags"):
		service.markMailMessage(responseWriter, request)
	default:
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) authorizeMailRequest(request *http.Request) bool {
	actorEmail := service.mailActorEmail(request)
	return isLocalRequest(request) || service.isFlowStaffActor(request.Context(), actorEmail)
}

func (service *Service) writeMailAccount(responseWriter http.ResponseWriter, request *http.Request) {
	account, _, errorValue := service.readMailAccountForRequest(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, mailAccountToResponse(account))
}

func (service *Service) saveMailAccount(responseWriter http.ResponseWriter, request *http.Request) {
	existingAccount, _, errorValue := service.readMailAccountForRequest(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	var payload mailAccountWriteRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	account, errorValue := mergeMailAccountWriteRequest(existingAccount, payload)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if errorValue := validateMailAccountForSave(account); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if errorValue := service.saveMailAccountRecord(request.Context(), account); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, mailAccountToResponse(account))
}

func (service *Service) testMailAccount(responseWriter http.ResponseWriter, request *http.Request) {
	account, _, errorValue := service.readMailAccountForRequest(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if request.Body != nil && request.ContentLength != 0 {
		var payload mailAccountWriteRequest
		if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
			return
		}
		account, errorValue = mergeMailAccountWriteRequest(account, payload)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
			return
		}
	}
	if errorValue := validateMailAccountForSave(account); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if errorValue := service.mailBackend.TestAccount(request.Context(), account); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, map[string]bool{"ok": true})
}

func (service *Service) writeMailboxes(responseWriter http.ResponseWriter, request *http.Request) {
	account, _, errorValue := service.readConfiguredMailAccount(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	mailboxes, errorValue := service.mailBackend.ListMailboxes(request.Context(), account)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, map[string]any{"mailboxes": mailboxes})
}

func (service *Service) writeMailMessages(responseWriter http.ResponseWriter, request *http.Request) {
	account, _, errorValue := service.readConfiguredMailAccount(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	input, errorValue := mailMessageListRequestFromURL(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	messages, errorValue := service.mailBackend.ListMessages(request.Context(), account, input)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, map[string]any{"messages": messages, "cursor": ""})
}

func (service *Service) writeMailMessage(responseWriter http.ResponseWriter, request *http.Request) {
	account, _, errorValue := service.readConfiguredMailAccount(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	mailbox, uid, errorValue := mailMessagePathParts(request, "")
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	message, errorValue := service.mailBackend.ReadMessage(request.Context(), account, mailbox, uid)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, message)
}

func (service *Service) sendMailMessage(responseWriter http.ResponseWriter, request *http.Request) {
	account, _, errorValue := service.readConfiguredMailAccount(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	var payload mailMessageSendRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	payload, errorValue = validateMailMessageSendRequest(payload)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	result, errorValue := service.mailBackend.SendMessage(request.Context(), account, payload)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, result)
}

func (service *Service) moveMailMessage(responseWriter http.ResponseWriter, request *http.Request) {
	account, _, errorValue := service.readConfiguredMailAccount(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	mailbox, uid, errorValue := mailMessagePathParts(request, "/move")
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	var payload mailMessageMoveRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	payload.TargetMailbox = strings.TrimSpace(payload.TargetMailbox)
	if payload.TargetMailbox == "" {
		http.Error(responseWriter, "targetMailbox is required", http.StatusBadRequest)
		return
	}
	if errorValue := service.mailBackend.MoveMessage(request.Context(), account, mailbox, uid, payload.TargetMailbox); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, map[string]bool{"moved": true})
}

func (service *Service) markMailMessage(responseWriter http.ResponseWriter, request *http.Request) {
	account, _, errorValue := service.readConfiguredMailAccount(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	mailbox, uid, errorValue := mailMessagePathParts(request, "/flags")
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	var payload mailMessageMarkRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if payload.Seen == nil && payload.Flagged == nil {
		http.Error(responseWriter, "seen or flagged is required", http.StatusBadRequest)
		return
	}
	if errorValue := service.mailBackend.MarkMessage(request.Context(), account, mailbox, uid, payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, map[string]bool{"marked": true})
}

func (service *Service) readConfiguredMailAccount(request *http.Request) (mailAccount, bool, error) {
	account, found, errorValue := service.readMailAccountForRequest(request)
	if errorValue != nil {
		return mailAccount{}, false, errorValue
	}
	if !found || !account.isConfigured() {
		return mailAccount{}, found, errors.New("mail account is not configured")
	}
	return account, found, nil
}

func (service *Service) readMailAccountForRequest(request *http.Request) (mailAccount, bool, error) {
	actorEmail := service.mailActorEmail(request)
	if actorEmail == "" {
		return mailAccount{}, false, errors.New("mail actor email is required")
	}
	return service.readMailAccount(request.Context(), actorEmail)
}

func (service *Service) mailActorEmail(request *http.Request) string {
	return strings.ToLower(strings.TrimSpace(firstNonEmpty(authenticatedCallerEmail(request), service.claimedAdminEmail(), service.seedAdminEmail())))
}

func (service *Service) openMailDatabase(ctx context.Context) (*sql.DB, error) {
	if errorValue := os.MkdirAll(filepath.Dir(service.Configuration.MailDatabasePath), 0o700); errorValue != nil {
		return nil, errorValue
	}
	database, errorValue := sql.Open("sqlite", service.Configuration.MailDatabasePath)
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := ensureMailSchema(ctx, database); errorValue != nil {
		_ = database.Close()
		return nil, errorValue
	}
	return database, nil
}

func ensureMailSchema(ctx context.Context, database *sql.DB) error {
	_, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS mail_accounts (
	actor_email TEXT PRIMARY KEY,
	email TEXT NOT NULL,
	from_address TEXT NOT NULL,
	display_name TEXT NOT NULL,
	imap_host TEXT NOT NULL,
	imap_port INTEGER NOT NULL,
	imap_security TEXT NOT NULL,
	imap_username TEXT NOT NULL,
	imap_password TEXT NOT NULL,
	smtp_host TEXT NOT NULL,
	smtp_port INTEGER NOT NULL,
	smtp_security TEXT NOT NULL,
	smtp_username TEXT NOT NULL,
	smtp_password TEXT NOT NULL,
	default_mailbox TEXT NOT NULL,
	sent_mailbox TEXT NOT NULL,
	updated_at TEXT NOT NULL
)`)
	return errorValue
}

func (service *Service) readMailAccount(ctx context.Context, actorEmail string) (mailAccount, bool, error) {
	account := defaultMailAccount(actorEmail)
	database, errorValue := service.openMailDatabase(ctx)
	if errorValue != nil {
		return mailAccount{}, false, errorValue
	}
	defer database.Close()
	row := database.QueryRowContext(ctx, `
SELECT actor_email, email, from_address, display_name, imap_host, imap_port, imap_security, imap_username, imap_password, smtp_host, smtp_port, smtp_security, smtp_username, smtp_password, default_mailbox, sent_mailbox, updated_at
FROM mail_accounts
WHERE actor_email = ?`, actorEmail)
	errorValue = row.Scan(
		&account.ActorEmail,
		&account.Email,
		&account.FromAddress,
		&account.DisplayName,
		&account.IMAPHost,
		&account.IMAPPort,
		&account.IMAPSecurity,
		&account.IMAPUsername,
		&account.IMAPPassword,
		&account.SMTPHost,
		&account.SMTPPort,
		&account.SMTPSecurity,
		&account.SMTPUsername,
		&account.SMTPPassword,
		&account.DefaultMailbox,
		&account.SentMailbox,
		&account.UpdatedAt,
	)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return account, false, nil
	}
	if errorValue != nil {
		return mailAccount{}, false, errorValue
	}
	return normalizeMailAccount(account), true, nil
}

func (service *Service) saveMailAccountRecord(ctx context.Context, account mailAccount) error {
	database, errorValue := service.openMailDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	account = normalizeMailAccount(account)
	account.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	_, errorValue = database.ExecContext(ctx, `
INSERT INTO mail_accounts(actor_email, email, from_address, display_name, imap_host, imap_port, imap_security, imap_username, imap_password, smtp_host, smtp_port, smtp_security, smtp_username, smtp_password, default_mailbox, sent_mailbox, updated_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(actor_email) DO UPDATE SET
	email = excluded.email,
	from_address = excluded.from_address,
	display_name = excluded.display_name,
	imap_host = excluded.imap_host,
	imap_port = excluded.imap_port,
	imap_security = excluded.imap_security,
	imap_username = excluded.imap_username,
	imap_password = excluded.imap_password,
	smtp_host = excluded.smtp_host,
	smtp_port = excluded.smtp_port,
	smtp_security = excluded.smtp_security,
	smtp_username = excluded.smtp_username,
	smtp_password = excluded.smtp_password,
	default_mailbox = excluded.default_mailbox,
	sent_mailbox = excluded.sent_mailbox,
	updated_at = excluded.updated_at`,
		account.ActorEmail,
		account.Email,
		account.FromAddress,
		account.DisplayName,
		account.IMAPHost,
		account.IMAPPort,
		account.IMAPSecurity,
		account.IMAPUsername,
		account.IMAPPassword,
		account.SMTPHost,
		account.SMTPPort,
		account.SMTPSecurity,
		account.SMTPUsername,
		account.SMTPPassword,
		account.DefaultMailbox,
		account.SentMailbox,
		account.UpdatedAt,
	)
	return errorValue
}

func defaultMailAccount(actorEmail string) mailAccount {
	return mailAccount{
		ActorEmail:     strings.ToLower(strings.TrimSpace(actorEmail)),
		Email:          strings.ToLower(strings.TrimSpace(actorEmail)),
		FromAddress:    strings.ToLower(strings.TrimSpace(actorEmail)),
		IMAPPort:       993,
		IMAPSecurity:   mailSecurityTLS,
		SMTPPort:       587,
		SMTPSecurity:   mailSecurityStartTLS,
		DefaultMailbox: "INBOX",
		SentMailbox:    "Sent",
	}
}

func mergeMailAccountWriteRequest(account mailAccount, payload mailAccountWriteRequest) (mailAccount, error) {
	account.Email = strings.ToLower(strings.TrimSpace(firstNonEmpty(payload.Email, account.Email, account.ActorEmail)))
	account.FromAddress = strings.TrimSpace(firstNonEmpty(payload.FromAddress, account.FromAddress, account.Email))
	account.DisplayName = strings.TrimSpace(payload.DisplayName)
	account.IMAPHost = strings.ToLower(strings.TrimSpace(payload.IMAPHost))
	account.IMAPPort = firstPositiveInteger(payload.IMAPPort, account.IMAPPort, 993)
	account.IMAPSecurity = normalizeMailSecurity(firstNonEmpty(payload.IMAPSecurity, account.IMAPSecurity, mailSecurityTLS))
	account.IMAPUsername = strings.TrimSpace(firstNonEmpty(payload.IMAPUsername, account.IMAPUsername, account.Email))
	account.SMTPHost = strings.ToLower(strings.TrimSpace(payload.SMTPHost))
	account.SMTPPort = firstPositiveInteger(payload.SMTPPort, account.SMTPPort, 587)
	account.SMTPSecurity = normalizeMailSecurity(firstNonEmpty(payload.SMTPSecurity, account.SMTPSecurity, mailSecurityStartTLS))
	account.SMTPUsername = strings.TrimSpace(firstNonEmpty(payload.SMTPUsername, account.SMTPUsername, account.Email))
	account.DefaultMailbox = strings.TrimSpace(firstNonEmpty(payload.DefaultMailbox, account.DefaultMailbox, "INBOX"))
	account.SentMailbox = strings.TrimSpace(firstNonEmpty(payload.SentMailbox, account.SentMailbox, "Sent"))
	if strings.TrimSpace(payload.IMAPPassword) != "" {
		account.IMAPPassword = strings.TrimSpace(payload.IMAPPassword)
	}
	if strings.TrimSpace(payload.SMTPPassword) != "" {
		account.SMTPPassword = strings.TrimSpace(payload.SMTPPassword)
	}
	if account.IMAPSecurity == "" || account.SMTPSecurity == "" {
		return mailAccount{}, errors.New("mail security must be tls, starttls, or none")
	}
	return normalizeMailAccount(account), nil
}

func normalizeMailAccount(account mailAccount) mailAccount {
	account.ActorEmail = strings.ToLower(strings.TrimSpace(account.ActorEmail))
	account.Email = strings.ToLower(strings.TrimSpace(account.Email))
	account.FromAddress = strings.TrimSpace(account.FromAddress)
	account.DisplayName = strings.TrimSpace(account.DisplayName)
	account.IMAPHost = strings.ToLower(strings.TrimSpace(account.IMAPHost))
	account.IMAPSecurity = normalizeMailSecurity(account.IMAPSecurity)
	account.IMAPUsername = strings.TrimSpace(account.IMAPUsername)
	account.SMTPHost = strings.ToLower(strings.TrimSpace(account.SMTPHost))
	account.SMTPSecurity = normalizeMailSecurity(account.SMTPSecurity)
	account.SMTPUsername = strings.TrimSpace(account.SMTPUsername)
	account.DefaultMailbox = strings.TrimSpace(firstNonEmpty(account.DefaultMailbox, "INBOX"))
	account.SentMailbox = strings.TrimSpace(firstNonEmpty(account.SentMailbox, "Sent"))
	if account.IMAPPort == 0 {
		account.IMAPPort = 993
	}
	if account.SMTPPort == 0 {
		account.SMTPPort = 587
	}
	return account
}

func validateMailAccountForSave(account mailAccount) error {
	if _, errorValue := messagemail.ParseAddress(account.Email); errorValue != nil {
		return fmt.Errorf("email is invalid: %w", errorValue)
	}
	if _, errorValue := messagemail.ParseAddress(account.FromAddress); errorValue != nil {
		return fmt.Errorf("fromAddress is invalid: %w", errorValue)
	}
	if strings.TrimSpace(account.IMAPHost) == "" {
		return errors.New("imapHost is required")
	}
	if strings.TrimSpace(account.IMAPUsername) == "" {
		return errors.New("imapUsername is required")
	}
	if strings.TrimSpace(account.IMAPPassword) == "" {
		return errors.New("imapPassword is required")
	}
	if strings.TrimSpace(account.SMTPHost) == "" {
		return errors.New("smtpHost is required")
	}
	if strings.TrimSpace(account.SMTPUsername) == "" {
		return errors.New("smtpUsername is required")
	}
	if strings.TrimSpace(account.SMTPPassword) == "" {
		return errors.New("smtpPassword is required")
	}
	if !isValidMailPort(account.IMAPPort) {
		return errors.New("imapPort must be between 1 and 65535")
	}
	if !isValidMailPort(account.SMTPPort) {
		return errors.New("smtpPort must be between 1 and 65535")
	}
	if normalizeMailSecurity(account.IMAPSecurity) == "" || normalizeMailSecurity(account.SMTPSecurity) == "" {
		return errors.New("mail security must be tls, starttls, or none")
	}
	return nil
}

func mailAccountToResponse(account mailAccount) mailAccountResponse {
	return mailAccountResponse{
		Email:           account.Email,
		FromAddress:     account.FromAddress,
		DisplayName:     account.DisplayName,
		IMAPHost:        account.IMAPHost,
		IMAPPort:        account.IMAPPort,
		IMAPSecurity:    account.IMAPSecurity,
		IMAPUsername:    account.IMAPUsername,
		SMTPHost:        account.SMTPHost,
		SMTPPort:        account.SMTPPort,
		SMTPSecurity:    account.SMTPSecurity,
		SMTPUsername:    account.SMTPUsername,
		DefaultMailbox:  account.DefaultMailbox,
		SentMailbox:     account.SentMailbox,
		IsConfigured:    account.isConfigured(),
		HasIMAPPassword: strings.TrimSpace(account.IMAPPassword) != "",
		HasSMTPPassword: strings.TrimSpace(account.SMTPPassword) != "",
	}
}

func (account mailAccount) isConfigured() bool {
	return strings.TrimSpace(account.IMAPHost) != "" &&
		strings.TrimSpace(account.IMAPUsername) != "" &&
		strings.TrimSpace(account.IMAPPassword) != "" &&
		strings.TrimSpace(account.SMTPHost) != "" &&
		strings.TrimSpace(account.SMTPUsername) != "" &&
		strings.TrimSpace(account.SMTPPassword) != ""
}

func normalizeMailSecurity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", mailSecurityTLS:
		return mailSecurityTLS
	case mailSecurityStartTLS:
		return mailSecurityStartTLS
	case mailSecurityNone:
		return mailSecurityNone
	default:
		return ""
	}
}

func isValidMailPort(port int) bool {
	return port > 0 && port <= 65535
}

func firstPositiveInteger(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func mailMessageListRequestFromURL(request *http.Request) (mailMessageListRequest, error) {
	limit := 50
	if value := strings.TrimSpace(request.URL.Query().Get("limit")); value != "" {
		parsedLimit, errorValue := strconv.Atoi(value)
		if errorValue != nil {
			return mailMessageListRequest{}, errors.New("limit must be a number")
		}
		limit = parsedLimit
	}
	if limit < 1 || limit > 100 {
		return mailMessageListRequest{}, errors.New("limit must be between 1 and 100")
	}
	return mailMessageListRequest{
		Mailbox: strings.TrimSpace(firstNonEmpty(request.URL.Query().Get("mailbox"), "INBOX")),
		Query:   strings.TrimSpace(request.URL.Query().Get("query")),
		Limit:   limit,
	}, nil
}

func mailMessagePathParts(request *http.Request, suffix string) (string, uint32, error) {
	trimmedPath := strings.TrimSuffix(strings.TrimPrefix(request.URL.EscapedPath(), "/mail/api/messages/"), suffix)
	parts := strings.Split(strings.Trim(trimmedPath, "/"), "/")
	if len(parts) != 2 {
		return "", 0, errors.New("mailbox and uid are required")
	}
	mailbox, errorValue := url.PathUnescape(strings.TrimSpace(parts[0]))
	if errorValue != nil {
		return "", 0, errors.New("mailbox is invalid")
	}
	uid, errorValue := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 32)
	if mailbox == "" || errorValue != nil || uid == 0 {
		return "", 0, errors.New("mailbox and uid are required")
	}
	return mailbox, uint32(uid), nil
}

func validateMailMessageSendRequest(input mailMessageSendRequest) (mailMessageSendRequest, error) {
	var errorValue error
	input.To, errorValue = validateMailAddressList(input.To, "to")
	if errorValue != nil {
		return mailMessageSendRequest{}, errorValue
	}
	input.CC, errorValue = validateMailAddressList(input.CC, "cc")
	if errorValue != nil {
		return mailMessageSendRequest{}, errorValue
	}
	input.BCC, errorValue = validateMailAddressList(input.BCC, "bcc")
	if errorValue != nil {
		return mailMessageSendRequest{}, errorValue
	}
	input.Subject = strings.TrimSpace(input.Subject)
	input.Body = strings.TrimSpace(input.Body)
	if len(input.To) == 0 {
		return mailMessageSendRequest{}, errors.New("to is required")
	}
	if input.Subject == "" && input.Body == "" {
		return mailMessageSendRequest{}, errors.New("subject or body is required")
	}
	return input, nil
}

func validateMailAddressList(values []string, fieldName string) ([]string, error) {
	addresses := []string{}
	seenAddresses := map[string]bool{}
	for _, value := range values {
		address := strings.TrimSpace(value)
		if address == "" {
			continue
		}
		parsedAddress, errorValue := messagemail.ParseAddress(address)
		if errorValue != nil {
			return nil, fmt.Errorf("%s contains an invalid address: %w", fieldName, errorValue)
		}
		normalizedAddress := strings.ToLower(parsedAddress.Address)
		if seenAddresses[normalizedAddress] {
			continue
		}
		seenAddresses[normalizedAddress] = true
		addresses = append(addresses, parsedAddress.String())
	}
	return addresses, nil
}

func (backend standardMailBackend) TestAccount(ctx context.Context, account mailAccount) error {
	imapClient, errorValue := backend.openIMAPClient(account)
	if errorValue != nil {
		return fmt.Errorf("imap connection failed: %w", errorValue)
	}
	closeIMAPClient(imapClient)
	smtpClient, errorValue := backend.openSMTPClient(account)
	if errorValue != nil {
		return fmt.Errorf("smtp connection failed: %w", errorValue)
	}
	return smtpClient.Quit()
}

func (backend standardMailBackend) ListMailboxes(ctx context.Context, account mailAccount) ([]mailMailboxResponse, error) {
	imapClient, errorValue := backend.openIMAPClient(account)
	if errorValue != nil {
		return nil, errorValue
	}
	defer closeIMAPClient(imapClient)
	listData, errorValue := imapClient.List("", "*", nil).Collect()
	if errorValue != nil {
		return nil, errorValue
	}
	mailboxes := mailMailboxResponsesFromListData(listData)
	addMailMailboxStatuses(imapClient, mailboxes)
	sort.SliceStable(mailboxes, func(firstIndex int, secondIndex int) bool {
		return mailMailboxSortKey(mailboxes[firstIndex].Name) < mailMailboxSortKey(mailboxes[secondIndex].Name)
	})
	return mailboxes, nil
}

func mailMailboxResponsesFromListData(listData []*imap.ListData) []mailMailboxResponse {
	mailboxes := make([]mailMailboxResponse, 0, len(listData))
	for _, mailboxData := range listData {
		if mailboxData == nil || mailboxData.Mailbox == "" || containsMailMailboxAttribute(mailboxData.Attrs, imap.MailboxAttrNoSelect) {
			continue
		}
		mailboxes = append(mailboxes, mailMailboxResponse{
			Name:        mailboxData.Mailbox,
			DisplayName: displayMailMailboxName(mailboxData),
			Unseen:      statusInteger(mailboxData.Status, "unseen"),
			Total:       statusInteger(mailboxData.Status, "total"),
		})
	}
	return mailboxes
}

func addMailMailboxStatuses(imapClient *imapclient.Client, mailboxes []mailMailboxResponse) {
	for index := range mailboxes {
		status, errorValue := imapClient.Status(mailboxes[index].Name, &imap.StatusOptions{NumMessages: true, NumUnseen: true}).Wait()
		if errorValue != nil {
			continue
		}
		mailboxes[index].Unseen = statusInteger(status, "unseen")
		mailboxes[index].Total = statusInteger(status, "total")
	}
}

func (backend standardMailBackend) ListMessages(ctx context.Context, account mailAccount, input mailMessageListRequest) ([]mailMessageResponse, error) {
	imapClient, errorValue := backend.openIMAPClient(account)
	if errorValue != nil {
		return nil, errorValue
	}
	defer closeIMAPClient(imapClient)
	selectedMailbox, errorValue := imapClient.Select(input.Mailbox, &imap.SelectOptions{ReadOnly: true}).Wait()
	if errorValue != nil {
		return nil, errorValue
	}
	numberSet, isUIDSet, errorValue := messageListNumberSet(imapClient, selectedMailbox, input)
	if errorValue != nil {
		return nil, errorValue
	}
	if numberSet == nil {
		return []mailMessageResponse{}, nil
	}
	fetchOptions := &imap.FetchOptions{
		UID:          true,
		Envelope:     true,
		Flags:        true,
		InternalDate: true,
	}
	messages, errorValue := imapClient.Fetch(numberSet, fetchOptions).Collect()
	if errorValue != nil {
		return nil, errorValue
	}
	responses := make([]mailMessageResponse, 0, len(messages))
	for _, message := range messages {
		response := mailMessageResponseFromBuffer(input.Mailbox, message, nil)
		if response.UID != 0 {
			responses = append(responses, response)
		}
	}
	sort.SliceStable(responses, func(firstIndex int, secondIndex int) bool {
		if isUIDSet {
			return responses[firstIndex].UID > responses[secondIndex].UID
		}
		return firstIndex > secondIndex
	})
	if len(responses) > input.Limit {
		responses = responses[:input.Limit]
	}
	return responses, nil
}

func (backend standardMailBackend) ReadMessage(ctx context.Context, account mailAccount, mailbox string, uid uint32) (mailMessageDetailResponse, error) {
	imapClient, errorValue := backend.openIMAPClient(account)
	if errorValue != nil {
		return mailMessageDetailResponse{}, errorValue
	}
	defer closeIMAPClient(imapClient)
	if _, errorValue := imapClient.Select(mailbox, &imap.SelectOptions{ReadOnly: true}).Wait(); errorValue != nil {
		return mailMessageDetailResponse{}, errorValue
	}
	bodySection := &imap.FetchItemBodySection{Peek: true}
	messages, errorValue := imapClient.Fetch(imap.UIDSetNum(imap.UID(uid)), &imap.FetchOptions{
		UID:          true,
		Envelope:     true,
		Flags:        true,
		InternalDate: true,
		BodySection:  []*imap.FetchItemBodySection{bodySection},
	}).Collect()
	if errorValue != nil {
		return mailMessageDetailResponse{}, errorValue
	}
	if len(messages) == 0 {
		return mailMessageDetailResponse{}, errors.New("message not found")
	}
	return mailMessageDetailFromBuffer(mailbox, messages[0], bodySection), nil
}

func (backend standardMailBackend) SendMessage(ctx context.Context, account mailAccount, input mailMessageSendRequest) (mailSendResult, error) {
	messageDocument, recipients, errorValue := createMailMessageDocument(account, input)
	if errorValue != nil {
		return mailSendResult{}, errorValue
	}
	if errorValue := backend.sendSMTPMessage(account, recipients, messageDocument); errorValue != nil {
		return mailSendResult{}, errorValue
	}
	result := mailSendResult{Sent: true}
	if strings.TrimSpace(account.SentMailbox) == "" {
		return result, nil
	}
	if errorValue := backend.appendSentMessage(account, messageDocument); errorValue != nil {
		result.AppendWarning = errorValue.Error()
		return result, nil
	}
	result.AppendedTo = account.SentMailbox
	return result, nil
}

func (backend standardMailBackend) MoveMessage(ctx context.Context, account mailAccount, mailbox string, uid uint32, targetMailbox string) error {
	imapClient, errorValue := backend.openIMAPClient(account)
	if errorValue != nil {
		return errorValue
	}
	defer closeIMAPClient(imapClient)
	if _, errorValue := imapClient.Select(mailbox, nil).Wait(); errorValue != nil {
		return errorValue
	}
	_, errorValue = imapClient.Move(imap.UIDSetNum(imap.UID(uid)), targetMailbox).Wait()
	return errorValue
}

func (backend standardMailBackend) MarkMessage(ctx context.Context, account mailAccount, mailbox string, uid uint32, input mailMessageMarkRequest) error {
	imapClient, errorValue := backend.openIMAPClient(account)
	if errorValue != nil {
		return errorValue
	}
	defer closeIMAPClient(imapClient)
	if _, errorValue := imapClient.Select(mailbox, nil).Wait(); errorValue != nil {
		return errorValue
	}
	if input.Seen != nil {
		if errorValue := storeMailFlag(imapClient, uid, imap.FlagSeen, *input.Seen); errorValue != nil {
			return errorValue
		}
	}
	if input.Flagged != nil {
		return storeMailFlag(imapClient, uid, imap.FlagFlagged, *input.Flagged)
	}
	return nil
}

func (backend standardMailBackend) openIMAPClient(account mailAccount) (*imapclient.Client, error) {
	address := net.JoinHostPort(account.IMAPHost, strconv.Itoa(account.IMAPPort))
	options := &imapclient.Options{
		TLSConfig: &tls.Config{ServerName: account.IMAPHost},
		WordDecoder: &mime.WordDecoder{
			CharsetReader: charset.Reader,
		},
		Dialer: &net.Dialer{Timeout: 30 * time.Second},
	}
	var client *imapclient.Client
	var errorValue error
	switch account.IMAPSecurity {
	case mailSecurityTLS:
		client, errorValue = imapclient.DialTLS(address, options)
	case mailSecurityStartTLS:
		client, errorValue = imapclient.DialStartTLS(address, options)
	case mailSecurityNone:
		client, errorValue = imapclient.DialInsecure(address, options)
	default:
		errorValue = errors.New("imapSecurity must be tls, starttls, or none")
	}
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := client.Login(account.IMAPUsername, account.IMAPPassword).Wait(); errorValue != nil {
		client.Close()
		return nil, errorValue
	}
	return client, nil
}

func (backend standardMailBackend) openSMTPClient(account mailAccount) (*smtp.Client, error) {
	address := net.JoinHostPort(account.SMTPHost, strconv.Itoa(account.SMTPPort))
	dialer := net.Dialer{Timeout: 30 * time.Second}
	var client *smtp.Client
	var errorValue error
	switch account.SMTPSecurity {
	case mailSecurityTLS:
		connection, dialError := tls.DialWithDialer(&dialer, "tcp", address, &tls.Config{ServerName: account.SMTPHost})
		if dialError != nil {
			return nil, dialError
		}
		client, errorValue = smtp.NewClient(connection, account.SMTPHost)
	case mailSecurityStartTLS:
		connection, dialError := dialer.Dial("tcp", address)
		if dialError != nil {
			return nil, dialError
		}
		client, errorValue = smtp.NewClient(connection, account.SMTPHost)
		if errorValue == nil {
			errorValue = client.StartTLS(&tls.Config{ServerName: account.SMTPHost})
		}
	case mailSecurityNone:
		connection, dialError := dialer.Dial("tcp", address)
		if dialError != nil {
			return nil, dialError
		}
		client, errorValue = smtp.NewClient(connection, account.SMTPHost)
	default:
		errorValue = errors.New("smtpSecurity must be tls, starttls, or none")
	}
	if errorValue != nil {
		return nil, errorValue
	}
	auth := smtp.PlainAuth("", account.SMTPUsername, account.SMTPPassword, account.SMTPHost)
	if errorValue := client.Auth(auth); errorValue != nil {
		_ = client.Close()
		return nil, errorValue
	}
	return client, nil
}

func (backend standardMailBackend) sendSMTPMessage(account mailAccount, recipients []string, messageDocument []byte) error {
	client, errorValue := backend.openSMTPClient(account)
	if errorValue != nil {
		return errorValue
	}
	defer client.Close()
	fromAddress, errorValue := messagemail.ParseAddress(account.FromAddress)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := client.Mail(fromAddress.Address); errorValue != nil {
		return errorValue
	}
	for _, recipient := range recipients {
		recipientAddress, errorValue := messagemail.ParseAddress(recipient)
		if errorValue != nil {
			return errorValue
		}
		if errorValue := client.Rcpt(recipientAddress.Address); errorValue != nil {
			return errorValue
		}
	}
	writer, errorValue := client.Data()
	if errorValue != nil {
		return errorValue
	}
	if _, errorValue := writer.Write(messageDocument); errorValue != nil {
		_ = writer.Close()
		return errorValue
	}
	if errorValue := writer.Close(); errorValue != nil {
		return errorValue
	}
	return client.Quit()
}

func (backend standardMailBackend) appendSentMessage(account mailAccount, messageDocument []byte) error {
	imapClient, errorValue := backend.openIMAPClient(account)
	if errorValue != nil {
		return errorValue
	}
	defer closeIMAPClient(imapClient)
	appendCommand := imapClient.Append(account.SentMailbox, int64(len(messageDocument)), &imap.AppendOptions{
		Flags: []imap.Flag{imap.FlagSeen},
		Time:  time.Now(),
	})
	if _, errorValue := appendCommand.Write(messageDocument); errorValue != nil {
		_ = appendCommand.Close()
		return errorValue
	}
	if errorValue := appendCommand.Close(); errorValue != nil {
		return errorValue
	}
	_, errorValue = appendCommand.Wait()
	return errorValue
}

func closeIMAPClient(client *imapclient.Client) {
	if client == nil {
		return
	}
	_ = client.Logout().Wait()
	client.Close()
}

func messageListNumberSet(client *imapclient.Client, selectedMailbox *imap.SelectData, input mailMessageListRequest) (imap.NumSet, bool, error) {
	if strings.TrimSpace(input.Query) != "" {
		searchData, errorValue := client.UIDSearch(&imap.SearchCriteria{Text: []string{input.Query}}, nil).Wait()
		if errorValue != nil {
			return nil, false, errorValue
		}
		uids := searchData.AllUIDs()
		if len(uids) == 0 {
			return nil, true, nil
		}
		sort.Slice(uids, func(firstIndex int, secondIndex int) bool {
			return uids[firstIndex] > uids[secondIndex]
		})
		if len(uids) > input.Limit {
			uids = uids[:input.Limit]
		}
		return imap.UIDSetNum(uids...), true, nil
	}
	if selectedMailbox == nil || selectedMailbox.NumMessages == 0 {
		return nil, false, nil
	}
	stop := selectedMailbox.NumMessages
	start := uint32(1)
	if stop > uint32(input.Limit) {
		start = stop - uint32(input.Limit) + 1
	}
	sequenceSet := imap.SeqSet{}
	sequenceSet.AddRange(start, stop)
	return sequenceSet, false, nil
}

func mailMessageResponseFromBuffer(mailbox string, message *imapclient.FetchMessageBuffer, bodySection *imap.FetchItemBodySection) mailMessageResponse {
	if message == nil {
		return mailMessageResponse{}
	}
	document := bodySectionBytes(message, bodySection)
	parsedDocument := parseMailDocument(document)
	date := message.InternalDate
	subject := ""
	from := ""
	if message.Envelope != nil {
		subject = decodeMailHeader(message.Envelope.Subject)
		from = imapAddressListString(message.Envelope.From)
		if !message.Envelope.Date.IsZero() {
			date = message.Envelope.Date
		}
	}
	return mailMessageResponse{
		UID:     uint32(message.UID),
		Mailbox: mailbox,
		Subject: subject,
		From:    from,
		Date:    formatMailDate(date),
		Preview: mailPreview(parsedDocument.previewText()),
		IsRead:  containsMailFlag(message.Flags, imap.FlagSeen),
	}
}

func mailMessageDetailFromBuffer(mailbox string, message *imapclient.FetchMessageBuffer, bodySection *imap.FetchItemBodySection) mailMessageDetailResponse {
	response := mailMessageResponseFromBuffer(mailbox, message, bodySection)
	parsedDocument := parseMailDocument(bodySectionBytes(message, bodySection))
	to := ""
	cc := ""
	if message != nil && message.Envelope != nil {
		to = imapAddressListString(message.Envelope.To)
		cc = imapAddressListString(message.Envelope.Cc)
	}
	return mailMessageDetailResponse{
		UID:      response.UID,
		Mailbox:  response.Mailbox,
		Subject:  response.Subject,
		From:     response.From,
		To:       to,
		CC:       cc,
		Date:     response.Date,
		Body:     parsedDocument.PlainText,
		BodyHTML: parsedDocument.HTML,
		IsRead:   response.IsRead,
	}
}

func bodySectionBytes(message *imapclient.FetchMessageBuffer, bodySection *imap.FetchItemBodySection) []byte {
	if message == nil {
		return nil
	}
	for _, section := range message.BodySection {
		if bodySection == nil || section.Section == nil || section.Section.Specifier == bodySection.Specifier {
			return section.Bytes
		}
	}
	return nil
}

func plainTextFromMailDocument(document []byte) string {
	return parseMailDocument(document).PlainText
}

func parseMailDocument(document []byte) parsedMailDocument {
	if len(bytes.TrimSpace(document)) == 0 {
		return parsedMailDocument{}
	}
	reader, errorValue := messagemail.CreateReader(bytes.NewReader(document))
	if errorValue != nil && reader == nil {
		return parsedMailDocument{PlainText: strings.TrimSpace(string(document))}
	}
	defer reader.Close()
	result := parsedMailDocument{}
	for {
		part, errorValue := reader.NextPart()
		if errors.Is(errorValue, io.EOF) {
			break
		}
		if errorValue != nil || part == nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(part.Body, 1024*1024))
		contentType := mailPartContentType(part)
		if result.PlainText == "" && contentType == "text/plain" {
			result.PlainText = strings.TrimSpace(string(body))
		}
		if result.HTML == "" && contentType == "text/html" {
			result.HTML = strings.TrimSpace(string(body))
		}
	}
	if result.PlainText == "" && result.HTML != "" {
		result.PlainText = stripHTML(result.HTML)
	}
	if result.PlainText == "" {
		result.PlainText = strings.TrimSpace(string(document))
	}
	return result
}

func mailPartContentType(part *messagemail.Part) string {
	if part == nil || part.Header == nil {
		return ""
	}
	if header, ok := part.Header.(interface {
		ContentType() (string, map[string]string, error)
	}); ok {
		contentType, _, errorValue := header.ContentType()
		if errorValue == nil {
			return strings.ToLower(contentType)
		}
	}
	contentType, _, errorValue := mime.ParseMediaType(part.Header.Get("Content-Type"))
	if errorValue == nil {
		return strings.ToLower(contentType)
	}
	return strings.ToLower(strings.TrimSpace(part.Header.Get("Content-Type")))
}

func (document parsedMailDocument) previewText() string {
	if strings.TrimSpace(document.PlainText) != "" {
		return document.PlainText
	}
	return stripHTML(document.HTML)
}

func decodeMailHeader(value string) string {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return ""
	}
	decodedValue, errorValue := mailWordDecoder().DecodeHeader(trimmedValue)
	if errorValue != nil {
		return trimmedValue
	}
	return strings.TrimSpace(decodedValue)
}

func mailWordDecoder() *mime.WordDecoder {
	return &mime.WordDecoder{CharsetReader: charset.Reader}
}

func stripHTML(document string) string {
	builder := strings.Builder{}
	isTag := false
	for _, value := range document {
		switch value {
		case '<':
			isTag = true
		case '>':
			isTag = false
			builder.WriteRune(' ')
		default:
			if !isTag {
				builder.WriteRune(value)
			}
		}
	}
	return strings.Join(strings.Fields(html.UnescapeString(builder.String())), " ")
}

func mailPreview(body string) string {
	preview := strings.Join(strings.Fields(body), " ")
	if len(preview) > 220 {
		return preview[:220]
	}
	return preview
}

func imapAddressListString(addresses []imap.Address) string {
	values := []string{}
	for _, address := range addresses {
		emailAddress := address.Addr()
		if emailAddress == "" {
			continue
		}
		if strings.TrimSpace(address.Name) != "" {
			values = append(values, displayMailAddress(decodeMailHeader(address.Name), emailAddress))
		} else {
			values = append(values, emailAddress)
		}
	}
	return strings.Join(values, ", ")
}

func displayMailAddress(name string, address string) string {
	displayName := strings.TrimSpace(name)
	emailAddress := strings.TrimSpace(address)
	if displayName == "" {
		return emailAddress
	}
	return displayName + " <" + emailAddress + ">"
}

func containsMailFlag(flags []imap.Flag, flag imap.Flag) bool {
	for _, value := range flags {
		if value == flag {
			return true
		}
	}
	return false
}

func containsMailMailboxAttribute(attributes []imap.MailboxAttr, attribute imap.MailboxAttr) bool {
	for _, value := range attributes {
		if value == attribute {
			return true
		}
	}
	return false
}

func storeMailFlag(client *imapclient.Client, uid uint32, flag imap.Flag, enabled bool) error {
	operation := imap.StoreFlagsDel
	if enabled {
		operation = imap.StoreFlagsAdd
	}
	return client.Store(imap.UIDSetNum(imap.UID(uid)), &imap.StoreFlags{
		Op:     operation,
		Silent: true,
		Flags:  []imap.Flag{flag},
	}, nil).Close()
}

func createMailMessageDocument(account mailAccount, input mailMessageSendRequest) ([]byte, []string, error) {
	from, errorValue := messagemail.ParseAddress(account.FromAddress)
	if errorValue != nil {
		return nil, nil, errorValue
	}
	if strings.TrimSpace(account.DisplayName) != "" {
		from.Name = account.DisplayName
	}
	to, errorValue := parseMailAddresses(input.To)
	if errorValue != nil {
		return nil, nil, errorValue
	}
	cc, errorValue := parseMailAddresses(input.CC)
	if errorValue != nil {
		return nil, nil, errorValue
	}
	bcc, errorValue := parseMailAddresses(input.BCC)
	if errorValue != nil {
		return nil, nil, errorValue
	}
	var header messagemail.Header
	header.SetAddressList("From", []*messagemail.Address{from})
	header.SetAddressList("To", to)
	header.SetAddressList("Cc", cc)
	header.SetSubject(input.Subject)
	header.SetDate(time.Now())
	_ = header.GenerateMessageID()
	var document bytes.Buffer
	writer, errorValue := messagemail.CreateSingleInlineWriter(&document, header)
	if errorValue != nil {
		return nil, nil, errorValue
	}
	if _, errorValue := io.WriteString(writer, input.Body); errorValue != nil {
		_ = writer.Close()
		return nil, nil, errorValue
	}
	if errorValue := writer.Close(); errorValue != nil {
		return nil, nil, errorValue
	}
	recipients := append(mailAddressStrings(to), mailAddressStrings(cc)...)
	recipients = append(recipients, mailAddressStrings(bcc)...)
	return document.Bytes(), recipients, nil
}

func parseMailAddresses(values []string) ([]*messagemail.Address, error) {
	addresses := make([]*messagemail.Address, 0, len(values))
	for _, value := range values {
		address, errorValue := messagemail.ParseAddress(value)
		if errorValue != nil {
			return nil, errorValue
		}
		addresses = append(addresses, address)
	}
	return addresses, nil
}

func mailAddressStrings(addresses []*messagemail.Address) []string {
	values := make([]string, 0, len(addresses))
	for _, address := range addresses {
		values = append(values, address.String())
	}
	return values
}

func displayMailMailboxName(mailboxData *imap.ListData) string {
	if mailboxData == nil {
		return ""
	}
	for _, attribute := range mailboxData.Attrs {
		switch attribute {
		case imap.MailboxAttrSent:
			return "Sent"
		case imap.MailboxAttrDrafts:
			return "Drafts"
		case imap.MailboxAttrArchive:
			return "Archive"
		case imap.MailboxAttrTrash:
			return "Trash"
		case imap.MailboxAttrJunk:
			return "Junk"
		}
	}
	return mailboxData.Mailbox
}

func mailMailboxSortKey(mailbox string) string {
	normalizedMailbox := strings.ToLower(mailbox)
	switch {
	case normalizedMailbox == "inbox":
		return "00-" + normalizedMailbox
	case strings.Contains(normalizedMailbox, "sent"):
		return "10-" + normalizedMailbox
	case strings.Contains(normalizedMailbox, "draft"):
		return "20-" + normalizedMailbox
	case strings.Contains(normalizedMailbox, "archive"):
		return "30-" + normalizedMailbox
	case strings.Contains(normalizedMailbox, "trash"):
		return "40-" + normalizedMailbox
	default:
		return "50-" + normalizedMailbox
	}
}

func statusInteger(status *imap.StatusData, key string) int {
	if status == nil {
		return 0
	}
	switch key {
	case "unseen":
		if status.NumUnseen == nil {
			return 0
		}
		return int(*status.NumUnseen)
	default:
		if status.NumMessages == nil {
			return 0
		}
		return int(*status.NumMessages)
	}
}

func formatMailDate(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
