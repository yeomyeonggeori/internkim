package admind

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	webLogoutMarkerCookieName = "internkim_logged_out"
	webLogoutMarkerDuration   = 365 * 24 * time.Hour
	webSessionCookieName      = "internkim_session"
	webSessionDuration        = 30 * 24 * time.Hour
	webSessionRenewalWindow   = 7 * 24 * time.Hour
	webSessionSecretName      = "web-session-secret"
)

type webSessionPayload struct {
	Email            string `json:"email"`
	MattermostUserID string `json:"mattermostUserID"`
	IssuedAt         int64  `json:"issuedAt"`
	ExpiresAt        int64  `json:"expiresAt"`
	PolicyVersion    string `json:"policyVersion"`
}

type webSessionResponse struct {
	Authenticated      bool   `json:"authenticated"`
	Email              string `json:"email,omitempty"`
	IdentityEmail      string `json:"identityEmail,omitempty"`
	NotInvited         bool   `json:"notInvited,omitempty"`
	Image              string `json:"image,omitempty"`
	CloudflareLoginURL string `json:"cloudflareLoginURL,omitempty"`
	IsAdmin            bool   `json:"isAdmin"`
	CanViewTasks       bool   `json:"canViewTasks"`
	IsPoCSuperAdmin    bool   `json:"isPocSuperAdmin"`
}

type webLogoutResponse struct {
	OK          bool   `json:"ok"`
	RedirectURL string `json:"redirectURL"`
}

func (service *Service) handleWebSession(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.NotFound(responseWriter, request)
		return
	}
	returnPath := loginReturnPathForRequest(request)
	email := service.webActorEmail(request)
	if email == "" {
		service.writeJSON(responseWriter, webSessionResponse{
			Authenticated:      false,
			CloudflareLoginURL: service.cloudflareLoginURLForReturnPath(returnPath),
		})
		return
	}
	if !service.canAuthenticateWebReturnPath(request.Context(), email, returnPath) {
		service.writeJSON(responseWriter, webSessionResponse{
			Authenticated:      false,
			IdentityEmail:      email,
			NotInvited:         true,
			CloudflareLoginURL: service.cloudflareLoginURLForReturnPath(returnPath),
		})
		return
	}
	service.renewWebSessionCookieIfExpiringSoon(responseWriter, request)
	isTaskRunAdmin := service.canManageTaskRuns(request.Context(), email)
	service.writeJSON(responseWriter, webSessionResponse{
		Authenticated:   true,
		Email:           email,
		Image:           profileImagePathForEmail(email),
		IsAdmin:         isTaskRunAdmin || service.isFlowAdminEmail(request.Context(), email),
		CanViewTasks:    service.canViewTaskRuns(request.Context(), email),
		IsPoCSuperAdmin: service.isFlowAdminEmail(request.Context(), email),
	})
}

func (service *Service) handleWebLogout(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.NotFound(responseWriter, request)
		return
	}
	http.SetCookie(responseWriter, expiredWebSessionCookie())
	http.SetCookie(responseWriter, webLogoutMarkerCookie())
	logAuditEvent("web session logout")
	service.writeJSON(responseWriter, webLogoutResponse{OK: true, RedirectURL: service.logoutRedirectURL(request)})
}

// logoutRedirectURL sends the browser to Cloudflare Access's logout endpoint when
// Access fronts the app, so the CF_Authorization session is cleared too and the
// user is not silently re-authenticated. Without Access it returns to the app.
func (service *Service) logoutRedirectURL(request *http.Request) string {
	teamDomain := strings.TrimSuffix(strings.TrimSpace(service.Configuration.CloudflareAccessTeamDomain), "/")
	if teamDomain != "" {
		return "https://" + teamDomain + "/cdn-cgi/access/logout"
	}
	return logoutRedirectURLForRequest(request)
}

func (service *Service) handleCloudflareAuthStart(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.NotFound(responseWriter, request)
		return
	}
	returnPath := safeWebReturnPath(request.URL.Query().Get("return"))
	if returnPath == "" {
		respondWebAuthError(responseWriter, http.StatusBadRequest, "잘못된 이동 경로입니다.")
		logAuditEvent("cloudflare auth start denied: unsafe return")
		return
	}
	http.Redirect(responseWriter, request, "/auth/cloudflare/callback?return="+url.QueryEscape(returnPath), http.StatusFound)
	logAuditEvent("cloudflare auth start")
}

func (service *Service) handleCloudflareAuthCallback(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.NotFound(responseWriter, request)
		return
	}
	returnPath := safeWebReturnPath(request.URL.Query().Get("return"))
	if returnPath == "" {
		respondWebAuthError(responseWriter, http.StatusBadRequest, "잘못된 이동 경로입니다.")
		logAuditEvent("cloudflare auth callback denied: unsafe return")
		return
	}
	email := service.authenticatedCallerEmail(request)
	if email == "" {
		respondWebAuthError(responseWriter, http.StatusUnauthorized, "Cloudflare Access 인증 정보가 없습니다.")
		logAuditEvent("cloudflare auth callback denied: missing identity")
		return
	}
	if !service.canAuthenticateWebReturnPath(request.Context(), email, returnPath) {
		respondWebAuthError(responseWriter, http.StatusForbidden, "InternKim 사용 권한이 없습니다.")
		logAuditEvent("cloudflare auth callback denied: non_staff")
		return
	}
	userRecord := mattermostUserRecord{Email: email}
	if errorValue := service.issueWebSessionCookie(responseWriter, request, userRecord); errorValue != nil {
		respondWebAuthError(responseWriter, http.StatusInternalServerError, "웹 세션을 만들지 못했습니다.")
		logAuditEvent("cloudflare auth callback failed: session")
		return
	}
	http.Redirect(responseWriter, request, returnPath, http.StatusFound)
	logAuditEvent("cloudflare auth callback success")
}

func (service *Service) renewWebSessionCookieIfExpiringSoon(responseWriter http.ResponseWriter, request *http.Request) {
	cookie, errorValue := request.Cookie(webSessionCookieName)
	if errorValue != nil {
		return
	}
	now := time.Now().UTC()
	payload, errorValue := service.verifyWebSessionPayload(request.Context(), cookie.Value, now)
	if errorValue != nil {
		return
	}
	if time.Unix(payload.ExpiresAt, 0).UTC().Sub(now) > webSessionRenewalWindow {
		return
	}
	userRecord := mattermostUserRecord{Email: payload.Email, ID: payload.MattermostUserID}
	if errorValue := service.issueWebSessionCookie(responseWriter, request, userRecord); errorValue != nil {
		logAuditEvent("web session renewal failed: " + errorValue.Error())
		return
	}
	logAuditEvent("web session renewed")
}

func (service *Service) webSessionActorEmail(request *http.Request) string {
	cookie, errorValue := request.Cookie(webSessionCookieName)
	if errorValue != nil {
		return ""
	}
	payload, errorValue := service.verifyWebSessionPayload(request.Context(), cookie.Value, time.Now().UTC())
	if errorValue != nil {
		logAuditEvent("web session invalid: " + errorValue.Error())
		return ""
	}
	return payload.Email
}

func (service *Service) issueWebSessionCookie(responseWriter http.ResponseWriter, request *http.Request, userRecord mattermostUserRecord) error {
	now := time.Now().UTC()
	policyVersion, errorValue := service.currentWebPolicyVersion(request.Context())
	if errorValue != nil {
		return errorValue
	}
	payload := webSessionPayload{
		Email:            strings.ToLower(strings.TrimSpace(userRecord.Email)),
		MattermostUserID: strings.TrimSpace(userRecord.ID),
		IssuedAt:         now.Unix(),
		ExpiresAt:        now.Add(webSessionDuration).Unix(),
		PolicyVersion:    policyVersion,
	}
	cookieValue, errorValue := service.signWebSessionPayload(payload)
	if errorValue != nil {
		return errorValue
	}
	http.SetCookie(responseWriter, &http.Cookie{
		Name:     webSessionCookieName,
		Value:    cookieValue,
		Path:     "/",
		Expires:  time.Unix(payload.ExpiresAt, 0).UTC(),
		MaxAge:   int(webSessionDuration.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(responseWriter, expiredWebLogoutMarkerCookie())
	return nil
}

func expiredWebSessionCookie() *http.Cookie {
	return &http.Cookie{
		Name:     webSessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
}

func webLogoutMarkerCookie() *http.Cookie {
	return &http.Cookie{
		Name:     webLogoutMarkerCookieName,
		Value:    "1",
		Path:     "/",
		Expires:  time.Now().UTC().Add(webLogoutMarkerDuration),
		MaxAge:   int(webLogoutMarkerDuration.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
}

func expiredWebLogoutMarkerCookie() *http.Cookie {
	return &http.Cookie{
		Name:     webLogoutMarkerCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
}

func (service *Service) signWebSessionPayload(payload webSessionPayload) (string, error) {
	key, errorValue := service.webSessionSigningKey()
	if errorValue != nil {
		return "", errorValue
	}
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		return "", errorValue
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(document)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(encodedPayload))
	signature := mac.Sum(nil)
	return encodedPayload + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (service *Service) verifyWebSessionPayload(ctx context.Context, cookieValue string, now time.Time) (webSessionPayload, error) {
	key, errorValue := service.webSessionSigningKey()
	if errorValue != nil {
		return webSessionPayload{}, errorValue
	}
	encodedPayload, encodedSignature, found := strings.Cut(strings.TrimSpace(cookieValue), ".")
	if !found || encodedPayload == "" || encodedSignature == "" {
		return webSessionPayload{}, errors.New("malformed")
	}
	signature, errorValue := base64.RawURLEncoding.DecodeString(encodedSignature)
	if errorValue != nil {
		return webSessionPayload{}, errors.New("bad_signature_encoding")
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(encodedPayload))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return webSessionPayload{}, errors.New("bad_signature")
	}
	document, errorValue := base64.RawURLEncoding.DecodeString(encodedPayload)
	if errorValue != nil {
		return webSessionPayload{}, errors.New("bad_payload_encoding")
	}
	var payload webSessionPayload
	if errorValue := json.Unmarshal(document, &payload); errorValue != nil {
		return webSessionPayload{}, errors.New("bad_payload")
	}
	if strings.TrimSpace(payload.Email) == "" || payload.ExpiresAt <= now.Unix() {
		return webSessionPayload{}, errors.New("expired")
	}
	currentVersion, errorValue := service.currentWebPolicyVersion(ctx)
	if errorValue != nil {
		return webSessionPayload{}, errors.New("policy_unavailable")
	}
	if payload.PolicyVersion != currentVersion {
		return webSessionPayload{}, errors.New("policy_changed")
	}
	return payload, nil
}

func (service *Service) webSessionSigningKey() ([]byte, error) {
	path := filepath.Join(service.Configuration.StateDirectory, webSessionSecretName)
	document, errorValue := os.ReadFile(path)
	if errorValue == nil && len(strings.TrimSpace(string(document))) >= 64 {
		key, errorValue := hex.DecodeString(strings.TrimSpace(string(document)))
		if errorValue != nil {
			return nil, errorValue
		}
		if len(key) != 32 {
			return nil, errors.New("web session secret must be 32 bytes")
		}
		return key, nil
	}
	if errorValue != nil && !errors.Is(errorValue, os.ErrNotExist) {
		return nil, errorValue
	}
	key := make([]byte, 32)
	if _, errorValue := rand.Read(key); errorValue != nil {
		return nil, errorValue
	}
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return nil, errorValue
	}
	if errorValue := os.WriteFile(path, []byte(hex.EncodeToString(key)), 0o600); errorValue != nil {
		return nil, errorValue
	}
	return key, nil
}

func (service *Service) currentWebPolicyVersion(ctx context.Context) (string, error) {
	if !service.hasDeviceAuth() {
		adminEmail := service.seedAdminEmail()
		claimedEmail := service.claimedAdminEmail()
		return hashWebPolicyRecords([]string{"local", adminEmail, claimedEmail}), nil
	}
	records, errorValue := service.currentUserRecords(ctx)
	if errorValue != nil {
		return "", errorValue
	}
	values := make([]string, 0, len(records))
	for _, record := range records {
		values = append(values, strings.Join([]string{
			strings.ToLower(strings.TrimSpace(record.Email)),
			strings.TrimSpace(record.UserID),
			strings.ToLower(strings.TrimSpace(record.Role)),
			strings.ToLower(strings.TrimSpace(record.Status)),
			strings.TrimSpace(record.MattermostUserID),
		}, "\x00"))
	}
	return hashWebPolicyRecords(values), nil
}

func hashWebPolicyRecords(values []string) string {
	sort.Strings(values)
	digest := sha256.Sum256([]byte(strings.Join(values, "\x1f")))
	return hex.EncodeToString(digest[:])
}

func (service *Service) cloudflareLoginURLForReturnPath(returnPath string) string {
	return "/auth/cloudflare/start?return=" + url.QueryEscape(returnPath)
}

func loginReturnPathForRequest(request *http.Request) string {
	if returnPath := safeWebReturnPath(request.URL.Query().Get("return")); returnPath != "" {
		return returnPath
	}
	return returnPathForRequest(request)
}

func logoutRedirectURLForRequest(request *http.Request) string {
	if returnPath := safeWebReturnPath(request.URL.Query().Get("return")); returnPath != "" {
		return returnPath
	}
	return "/flow/"
}

func returnPathForRequest(request *http.Request) string {
	path := request.URL.Path
	if request.URL.RawQuery != "" {
		path += "?" + request.URL.RawQuery
	}
	if safePath := safeWebReturnPath(path); safePath != "" {
		return safePath
	}
	return "/flow/"
}

func safeWebReturnPath(value string) string {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" || !strings.HasPrefix(trimmedValue, "/") || strings.HasPrefix(trimmedValue, "//") {
		return ""
	}
	parsedURL, errorValue := url.Parse(trimmedValue)
	if errorValue != nil || parsedURL.IsAbs() || parsedURL.Host != "" {
		return ""
	}
	if strings.Contains(parsedURL.Path, "/api/") || strings.HasSuffix(parsedURL.Path, "/api") {
		return ""
	}
	for _, prefix := range []string{"/flow/", "/memory/", "/calendar/", "/mail/", "/attendance/", "/files/", "/tasks/", "/poc-admin/"} {
		if parsedURL.Path == strings.TrimSuffix(prefix, "/") || strings.HasPrefix(parsedURL.Path, prefix) {
			return parsedURL.RequestURI()
		}
	}
	return ""
}

func webAuthBaseURLFromRequest(request *http.Request) string {
	scheme := firstNonEmpty(request.Header.Get("X-Forwarded-Proto"), "https")
	if isLocalRequest(request) {
		scheme = "http"
	}
	return scheme + "://" + request.Host
}

func logAuditEvent(message string) {
	log.Printf("auth audit: %s", message)
}

func respondWebAuthError(responseWriter http.ResponseWriter, status int, message string) {
	http.Error(responseWriter, message, status)
}
