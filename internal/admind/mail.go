package admind

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

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
	HasIMAPPassword bool   `json:"hasIMAPPassword"`
	HasSMTPPassword bool   `json:"hasSMTPPassword"`
}

type mailMailboxResponse struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Unseen      int    `json:"unseen"`
	Total       int    `json:"total"`
}

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
		http.Error(responseWriter, "mail account settings are not available yet", http.StatusNotImplemented)
	case request.Method == http.MethodPost && path == "/account/test":
		http.Error(responseWriter, "mail account testing is not available yet", http.StatusNotImplemented)
	case request.Method == http.MethodGet && path == "/mailboxes":
		service.writeMailboxes(responseWriter)
	case request.Method == http.MethodGet && path == "/messages":
		service.writeMailMessages(responseWriter)
	case request.Method == http.MethodGet && strings.HasPrefix(path, "/messages/"):
		service.writeMailMessage(responseWriter, request)
	case request.Method == http.MethodPost && path == "/messages/send":
		http.Error(responseWriter, "mail sending is not available yet", http.StatusNotImplemented)
	case request.Method == http.MethodPost && strings.HasSuffix(path, "/move"):
		http.Error(responseWriter, "mail move is not available yet", http.StatusNotImplemented)
	case request.Method == http.MethodPost && strings.HasSuffix(path, "/flags"):
		http.Error(responseWriter, "mail flags are not available yet", http.StatusNotImplemented)
	default:
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) authorizeMailRequest(request *http.Request) bool {
	actorEmail := authenticatedCallerEmail(request)
	if actorEmail == "" {
		actorEmail = service.claimedAdminEmail()
	}
	if actorEmail == "" {
		actorEmail = service.seedAdminEmail()
	}
	return isLocalRequest(request) || service.isFlowStaffActor(request.Context(), actorEmail)
}

func (service *Service) writeMailAccount(responseWriter http.ResponseWriter, request *http.Request) {
	email := firstNonEmpty(authenticatedCallerEmail(request), service.claimedAdminEmail(), service.seedAdminEmail())
	service.writeJSON(responseWriter, mailAccountResponse{
		Email:          strings.ToLower(strings.TrimSpace(email)),
		FromAddress:    strings.ToLower(strings.TrimSpace(email)),
		DefaultMailbox: "INBOX",
		IMAPPort:       993,
		IMAPSecurity:   "tls",
		SMTPPort:       587,
		SMTPSecurity:   "starttls",
	})
}

func (service *Service) writeMailboxes(responseWriter http.ResponseWriter) {
	service.writeJSON(responseWriter, map[string]any{"mailboxes": []mailMailboxResponse{
		{Name: "INBOX", DisplayName: "Inbox"},
		{Name: "Sent", DisplayName: "Sent"},
		{Name: "Drafts", DisplayName: "Drafts"},
		{Name: "Archive", DisplayName: "Archive"},
		{Name: "Trash", DisplayName: "Trash"},
	}})
}

func (service *Service) writeMailMessages(responseWriter http.ResponseWriter) {
	service.writeJSON(responseWriter, map[string]any{
		"messages": []any{},
		"cursor":   "",
	})
}

func (service *Service) writeMailMessage(responseWriter http.ResponseWriter, request *http.Request) {
	path := strings.TrimPrefix(request.URL.Path, "/mail/api/messages/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		http.NotFound(responseWriter, request)
		return
	}
	uid, errorValue := strconv.ParseUint(parts[1], 10, 64)
	if errorValue != nil || uid == 0 {
		http.Error(responseWriter, "message uid is required", http.StatusBadRequest)
		return
	}
	service.writeJSON(responseWriter, map[string]any{
		"uid":     uid,
		"mailbox": parts[0],
		"body":    "",
	})
}
