package admind

import (
	"encoding/json"
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

type googleOAuthPopupMessage struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

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
	PopupConnectedBody          string
	PopupFailedBody             string
}

func googleOAuthRedirectURIFromRequest(request *http.Request) string {
	scheme := "http"
	if request.TLS != nil {
		scheme = "https"
	}
	if forwarded := strings.TrimSpace(request.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		scheme = forwarded
	}
	host := request.Host
	if forwardedHost := strings.TrimSpace(request.Header.Get("X-Forwarded-Host")); forwardedHost != "" {
		host = forwardedHost
	}
	return scheme + "://" + host + googleOAuthCallbackPath
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
			PopupConnectedBody:          "You can close this window if it does not close automatically.",
			PopupFailedBody:             "Close this window and try again if it does not close automatically.",
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
		PopupConnectedBody:          "이 창이 자동으로 닫히지 않으면 닫아도 됩니다.",
		PopupFailedBody:             "이 창이 자동으로 닫히지 않으면 닫고 다시 시도해주세요.",
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

func respondGoogleOAuthPopupResult(writer http.ResponseWriter, request *http.Request, returnURL string, status string) {
	text := googleOAuthResponseTextForRequest(request)
	title := googleOAuthPopupTitle(text, status)
	targetOrigin := googleOAuthPopupTargetOrigin(request, returnURL)
	messageJSON := mustMarshalGoogleOAuthPopupJSON(googleOAuthPopupMessage{
		Type:   googleOAuthReturnSignal,
		Status: status,
	})
	targetOriginJSON := mustMarshalGoogleOAuthPopupJSON(targetOrigin)
	storageKeyJSON := mustMarshalGoogleOAuthPopupJSON(googleOAuthReturnSignal)
	statusJSON := mustMarshalGoogleOAuthPopupJSON(status)
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.WriteHeader(http.StatusOK)
	fmt.Fprintf(writer, `<!doctype html>
<html lang="%s">
<head><meta charset="utf-8"><title>%s</title>
<style>body{font-family:system-ui,sans-serif;margin:4rem auto;max-width:32rem;padding:0 1rem;line-height:1.6}</style>
</head>
<body hidden>
<h1>%s</h1>
<p>%s</p>
<script>
(function() {
	var message = %s;
	try {
		if (window.opener && !window.opener.closed) {
			window.opener.postMessage(message, %s);
		}
	} catch (errorValue) {
		document.documentElement.dataset.messageError = String(errorValue);
	}
	try {
		window.localStorage.setItem(%s, JSON.stringify({status: %s, issuedAt: Date.now()}));
	} catch (errorValue) {
		document.documentElement.dataset.storageError = String(errorValue);
	}
	window.close();
	window.setTimeout(function() {
		document.body.hidden = false;
	}, 300);
})();
</script>
</body>
</html>`,
		text.LanguageCode,
		html.EscapeString(title),
		html.EscapeString(title),
		html.EscapeString(googleOAuthPopupBody(text, status)),
		messageJSON,
		targetOriginJSON,
		storageKeyJSON,
		statusJSON,
	)
}

func googleOAuthPopupTitle(text googleOAuthResponseText, status string) string {
	if status == googleOAuthReturnStatusConnected {
		if text.LanguageCode == "en" {
			return "Google Calendar connected"
		}
		return "Google 캘린더 연결 완료"
	}
	return text.ErrorTitle
}

func googleOAuthPopupBody(text googleOAuthResponseText, status string) string {
	if status == googleOAuthReturnStatusConnected {
		return text.PopupConnectedBody
	}
	return text.PopupFailedBody
}

func googleOAuthPopupTargetOrigin(request *http.Request, returnURL string) string {
	parsedReturnURL, errorValue := url.Parse(returnURL)
	if errorValue == nil && parsedReturnURL.Scheme != "" && parsedReturnURL.Host != "" {
		return parsedReturnURL.Scheme + "://" + parsedReturnURL.Host
	}
	redirectURI, errorValue := url.Parse(googleOAuthRedirectURIFromRequest(request))
	if errorValue == nil && redirectURI.Scheme != "" && redirectURI.Host != "" {
		return redirectURI.Scheme + "://" + redirectURI.Host
	}
	return "null"
}

func mustMarshalGoogleOAuthPopupJSON(value any) string {
	encoded, errorValue := json.Marshal(value)
	if errorValue != nil {
		return "null"
	}
	return string(encoded)
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
