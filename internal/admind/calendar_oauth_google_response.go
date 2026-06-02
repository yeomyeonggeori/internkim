package admind

import (
	"fmt"
	"html"
	"net/http"
	"strings"
)

type googleOAuthResponseText struct {
	LanguageCode                string
	SuccessTitle                string
	SuccessMessageTemplate      string
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

func googleOAuthResponseTextForRequest(request *http.Request) googleOAuthResponseText {
	if acceptLanguagePrefersEnglish(request.Header.Get("Accept-Language")) {
		return googleOAuthResponseText{
			LanguageCode:                "en",
			SuccessTitle:                "Google Calendar connected",
			SuccessMessageTemplate:      "%s account is connected. You can close this tab.",
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
		SuccessTitle:                "Google 캘린더 연결 완료",
		SuccessMessageTemplate:      "%s 계정으로 연결되었습니다. 이 탭은 닫아도 됩니다.",
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

func respondGoogleOAuthSuccessHTML(writer http.ResponseWriter, request *http.Request, email string) {
	text := googleOAuthResponseTextForRequest(request)
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(writer, `<!doctype html>
<html lang="%s">
<head><meta charset="utf-8"><title>%s</title>
<style>body{font-family:system-ui,sans-serif;margin:4rem auto;max-width:32rem;padding:0 1rem;line-height:1.6}h1{color:#2563eb}</style>
</head>
<body>
<h1>%s</h1>
<p>%s</p>
</body>
</html>`,
		text.LanguageCode,
		html.EscapeString(text.SuccessTitle),
		html.EscapeString(text.SuccessTitle),
		fmt.Sprintf(html.EscapeString(text.SuccessMessageTemplate), "<strong>"+html.EscapeString(email)+"</strong>"),
	)
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
