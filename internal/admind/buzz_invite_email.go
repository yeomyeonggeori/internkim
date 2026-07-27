package admind

import (
	"encoding/json"
	"net/http"
	"strings"
)

type buzzInviteEmailRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (service *Service) messengerPublicURL() string {
	base := strings.TrimRight(strings.TrimSpace(service.Configuration.FlowPublicURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(readTrimmedFile(service.Configuration.DeviceURLPath)), "/")
	}
	if base == "" {
		return ""
	}
	return base + "/messenger"
}

// handleBuzzInviteEmail sends a person their onboarding invitation from the
// staff member's own connected mailbox (the mail-client SMTP account), so it
// arrives from a real address with good deliverability and needs no dedicated
// system mail sender. The invite carries only the messenger link and guidance —
// Cloudflare Access is the gate, so there is no secret invite token.
func (service *Service) handleBuzzInviteEmail(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.NotFound(responseWriter, request)
		return
	}
	if !service.authorizeInternalOrWebStaffRequest(request) {
		http.Error(responseWriter, "staff access required", http.StatusForbidden)
		return
	}
	var payload buzzInviteEmailRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, "invalid request", http.StatusBadRequest)
		return
	}
	inviteeEmail := strings.ToLower(strings.TrimSpace(payload.Email))
	if inviteeEmail == "" || !strings.Contains(inviteeEmail, "@") {
		http.Error(responseWriter, "a valid email is required", http.StatusBadRequest)
		return
	}
	account, found, errorValue := service.readMailAccountForRequest(request)
	if errorValue != nil || !found {
		http.Error(responseWriter, "connect your mailbox before sending invites", http.StatusBadRequest)
		return
	}
	link := service.messengerPublicURL()
	if link == "" {
		http.Error(responseWriter, "messenger public URL is not configured", http.StatusNotImplemented)
		return
	}
	message := buildBuzzInviteMessage(account.FromAddress, inviteeEmail, strings.TrimSpace(payload.Name), link)
	if errorValue := (standardMailBackend{}).sendSMTPMessage(account, []string{inviteeEmail}, message); errorValue != nil {
		http.Error(responseWriter, "invite_send_failed", http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, map[string]bool{"sent": true})
}

func buildBuzzInviteMessage(fromAddress string, toAddress string, name string, link string) []byte {
	greeting := "Hello"
	if name != "" {
		greeting = "Hi " + name
	}
	headers := "From: " + fromAddress + "\r\n" +
		"To: " + toAddress + "\r\n" +
		"Subject: You're invited to InternKim\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n"
	body := greeting + ",\r\n\r\n" +
		"You've been invited to InternKim.\r\n\r\n" +
		"Open " + link + " and sign in with your work account.\r\n" +
		"Your messenger identity is set up automatically on first sign-in; you'll be asked to protect it with a passkey or password.\r\n"
	return []byte(headers + body)
}
