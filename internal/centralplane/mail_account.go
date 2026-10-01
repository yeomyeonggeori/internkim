package centralplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"gitlab.com/eastriver/internkim/internal/box"
)

// One person's mail account, in the shape /api/agent/mail-account answers it.
type MailAccount struct {
	ActorEmail     string `json:"ActorEmail"`
	Email          string `json:"Email"`
	FromAddress    string `json:"FromAddress"`
	DisplayName    string `json:"DisplayName"`
	IMAPHost       string `json:"IMAPHost"`
	IMAPPort       int    `json:"IMAPPort"`
	IMAPSecurity   string `json:"IMAPSecurity"`
	IMAPUsername   string `json:"IMAPUsername"`
	IMAPPassword   string `json:"IMAPPassword"`
	SMTPHost       string `json:"SMTPHost"`
	SMTPPort       int    `json:"SMTPPort"`
	SMTPSecurity   string `json:"SMTPSecurity"`
	SMTPUsername   string `json:"SMTPUsername"`
	SMTPPassword   string `json:"SMTPPassword"`
	DefaultMailbox string `json:"DefaultMailbox"`
	SentMailbox    string `json:"SentMailbox"`

	MemberID           string            `json:"MemberID"`
	SealedIMAPPassword *box.SealedSecret `json:"SealedIMAPPassword"`
	SealedSMTPPassword *box.SealedSecret `json:"SealedSMTPPassword"`
}

func (client *Client) MailAccount(ctx context.Context, actorEmail string) (MailAccount, bool, error) {
	member, known, errorValue := client.MemberByEmail(ctx, actorEmail)
	if errorValue != nil {
		return MailAccount{}, false, errorValue
	}
	if !known {
		return MailAccount{}, false, nil
	}
	var answered struct {
		Account *MailAccount `json:"account"`
	}
	if errorValue := client.readAsAgent(ctx, "/api/agent/mail-account?memberID="+url.QueryEscape(member.MemberID), &answered); errorValue != nil {
		return MailAccount{}, false, errorValue
	}
	if answered.Account == nil {
		return MailAccount{}, false, nil
	}
	held := *answered.Account
	held.MemberID = member.MemberID
	return held, true, nil
}

func (client *Client) MailAccounts(ctx context.Context) ([]MailAccount, error) {
	var answered struct {
		Accounts []MailAccount `json:"accounts"`
	}
	if errorValue := client.readAsAgent(ctx, "/api/agent/mail-accounts", &answered); errorValue != nil {
		return nil, errorValue
	}
	return answered.Accounts, nil
}

// /api/member/mail-account resolves the caller from their own session and
// writes only their account, so a carry signs as the person whose mail it is.
func (client *Client) WriteMailAccount(ctx context.Context, actorEmail string, account MailAccount) error {
	session, errorValue := client.sessionFor(ctx, "email", actorEmail)
	if errorValue != nil {
		return errorValue
	}
	payload, errorValue := json.Marshal(mailAccountAsWritten(account))
	if errorValue != nil {
		return errorValue
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPut,
		strings.TrimSuffix(client.settings.AppURL, "/")+"/api/member/mail-account", bytes.NewReader(payload))
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Authorization", "Bearer "+session.accessToken)
	request.Header.Set("Content-Type", "application/json")

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return fmt.Errorf("the record refused %s's mail account: %s", actorEmail, response.Status)
	}
	return nil
}

// That route takes the shape the settings screen submits, spelled its way.
func mailAccountAsWritten(account MailAccount) map[string]any {
	return map[string]any{
		"email":          account.Email,
		"fromAddress":    account.FromAddress,
		"displayName":    account.DisplayName,
		"imapHost":       account.IMAPHost,
		"imapPort":       account.IMAPPort,
		"imapSecurity":   account.IMAPSecurity,
		"imapUsername":   account.IMAPUsername,
		"imapPassword":   account.IMAPPassword,
		"smtpHost":       account.SMTPHost,
		"smtpPort":       account.SMTPPort,
		"smtpSecurity":   account.SMTPSecurity,
		"smtpUsername":   account.SMTPUsername,
		"smtpPassword":   account.SMTPPassword,
		"defaultMailbox": account.DefaultMailbox,
		"sentMailbox":    account.SentMailbox,
	}
}

func (client *Client) readAsAgent(ctx context.Context, path string, answer any) error {
	if client == nil || !client.settings.Configured() {
		return fmt.Errorf("central plane is not configured")
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimSuffix(client.settings.AppURL, "/")+path, nil)
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Authorization", "Bearer "+client.settings.HostCredential())

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return fmt.Errorf("the record refused %s: %s", path, response.Status)
	}
	return json.NewDecoder(response.Body).Decode(answer)
}
