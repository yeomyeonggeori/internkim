package admind

import (
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"strings"
)

const (
	googleOAuthDefaultReturnURL      = "/calendar/"
	googleOAuthReturnStatusConnected = "connected"
	googleOAuthReturnStatusFailed    = "failed"
)

type googleOAuthResponseText struct {
	LanguageCode                string
	ErrorTitle                  string
	ClientConfigurationError    string
	StateGenerationError        string
	GoogleReturnedErrorTemplate string
	MissingCallbackParameters   string
	InvalidState                string
	StateRecordTypeError        string
	ExpiredState                string
	OAuthConfigurationError     string
	TokenExchangeError          string
	UserinfoError               string
	TokenSaveError              string
}

func googleOAuthRedirectURIFromRequest(request *http.Request) string {
	scheme := "http"
	if request.TLS != nil {
		scheme = "https"
	}
	if forwarded := strings.TrimSpace(request.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		scheme = forwarded
	}
	return scheme + "://" + request.Host + googleOAuthCallbackPath
}

func googleOAuthReturnURLFromRequest(request *http.Request) string {
	rawReturnURL := strings.TrimSpace(request.URL.Query().Get("returnTo"))
	if rawReturnURL == "" {
		return googleOAuthDefaultReturnURL
	}
	returnURL, errorValue := url.Parse(rawReturnURL)
	if errorValue != nil {
		return googleOAuthDefaultReturnURL
	}
	if !isGoogleOAuthCalendarReturnPath(returnURL.Path) {
		return googleOAuthDefaultReturnURL
	}
	if returnURL.Host != "" && !isAllowedGoogleOAuthAbsoluteReturnURL(request, returnURL) {
		return googleOAuthDefaultReturnURL
	}
	return returnURL.String()
}

func isGoogleOAuthCalendarReturnPath(path string) bool {
	return path == "/calendar" || strings.HasPrefix(path, "/calendar/")
}

func isAllowedGoogleOAuthAbsoluteReturnURL(request *http.Request, returnURL *url.URL) bool {
	if returnURL.Scheme != "http" && returnURL.Scheme != "https" {
		return false
	}
	return isAllowedGoogleOAuthReturnHost(request, returnURL.Host)
}

func isAllowedGoogleOAuthReturnHost(request *http.Request, host string) bool {
	if strings.EqualFold(host, request.Host) {
		return true
	}
	if forwardedHost := strings.TrimSpace(request.Header.Get("X-Forwarded-Host")); strings.EqualFold(host, forwardedHost) {
		return true
	}
	return isLoopbackHost(host) && isLoopbackHost(request.Host)
}

func isLoopbackHost(host string) bool {
	hostname := host
	if parsedHostname, _, errorValue := net.SplitHostPort(host); errorValue == nil {
		hostname = parsedHostname
	}
	if strings.EqualFold(hostname, "localhost") {
		return true
	}
	ipAddress := net.ParseIP(strings.Trim(hostname, "[]"))
	return ipAddress != nil && ipAddress.IsLoopback()
}

func googleOAuthResponseTextForRequest(request *http.Request) googleOAuthResponseText {
	if acceptLanguagePrefersEnglish(request.Header.Get("Accept-Language")) {
		return googleOAuthResponseText{
			LanguageCode:                "en",
			ErrorTitle:                  "Google Calendar connection failed",
			ClientConfigurationError:    "Could not load OAuth settings. Check client.json.",
			StateGenerationError:        "Internal error while generating state.",
			GoogleReturnedErrorTemplate: "Google returned an error: %s",
			MissingCallbackParameters:   "Missing state or code parameter.",
			InvalidState:                "The state is invalid or already used. Start again.",
			StateRecordTypeError:        "State record type error.",
			ExpiredState:                "The state expired. Start again.",
			OAuthConfigurationError:     "Could not load OAuth settings.",
			TokenExchangeError:          "Could not exchange the Google token.",
			UserinfoError:               "Could not fetch Google user information.",
			TokenSaveError:              "Could not save the token.",
		}
	}
	return googleOAuthResponseText{
		LanguageCode:                "ko",
		ErrorTitle:                  "Google 캘린더 연결 실패",
		ClientConfigurationError:    "OAuth 설정을 불러올 수 없습니다. client.json 을 확인해주세요.",
		StateGenerationError:        "내부 오류로 state 생성에 실패했습니다.",
		GoogleReturnedErrorTemplate: "Google이 오류를 반환했습니다: %s",
		MissingCallbackParameters:   "state 또는 code 파라미터가 누락되었습니다.",
		InvalidState:                "잘못되었거나 이미 사용된 state 입니다. 처음부터 다시 시도해주세요.",
		StateRecordTypeError:        "state 레코드 타입 오류입니다.",
		ExpiredState:                "state 가 만료되었습니다. 다시 시작해주세요.",
		OAuthConfigurationError:     "OAuth 설정 로드에 실패했습니다.",
		TokenExchangeError:          "Google 토큰 교환에 실패했습니다.",
		UserinfoError:               "Google 사용자 정보 조회에 실패했습니다.",
		TokenSaveError:              "토큰 저장에 실패했습니다.",
	}
}

func acceptLanguagePrefersEnglish(header string) bool {
	for _, item := range strings.Split(header, ",") {
		language := strings.ToLower(strings.TrimSpace(strings.Split(item, ";")[0]))
		if strings.HasPrefix(language, "en") {
			return true
		}
	}
	return false
}

func redirectGoogleOAuthResult(writer http.ResponseWriter, request *http.Request, returnURL string, status string) {
	resultURL, errorValue := googleOAuthResultURL(returnURL, status)
	if errorValue != nil {
		resultURL = googleOAuthDefaultReturnURL + "?googleOAuth=" + url.QueryEscape(status)
	}
	http.Redirect(writer, request, resultURL, http.StatusSeeOther)
}

func googleOAuthResultURL(returnURL string, status string) (string, error) {
	resultURL, errorValue := url.Parse(returnURL)
	if errorValue != nil {
		return "", errorValue
	}
	query := resultURL.Query()
	query.Set("googleOAuth", status)
	resultURL.RawQuery = query.Encode()
	return resultURL.String(), nil
}

func respondGoogleOAuthErrorHTML(writer http.ResponseWriter, request *http.Request, status int, message string) {
	text := googleOAuthResponseTextForRequest(request)
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.WriteHeader(status)
	fmt.Fprintf(writer, `<!doctype html>
<html lang="%s">
<head><meta charset="utf-8"><title>%s</title>
<style>body{font-family:system-ui,sans-serif;margin:4rem auto;max-width:32rem;padding:0 1rem;line-height:1.6}h1{color:#dc2626}</style>
</head>
<body>
<h1>%s</h1>
<p>%s</p>
</body>
</html>`,
		text.LanguageCode,
		html.EscapeString(text.ErrorTitle),
		html.EscapeString(text.ErrorTitle),
		html.EscapeString(message),
	)
}
