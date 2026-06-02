package admind

import (
	"fmt"
	"html"
	"net/http"
	"strings"
)

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

func respondGoogleOAuthSuccessHTML(writer http.ResponseWriter, email string) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(writer, `<!doctype html>
<html lang="ko">
<head><meta charset="utf-8"><title>Google 캘린더 연결 완료</title>
<style>body{font-family:system-ui,sans-serif;margin:4rem auto;max-width:32rem;padding:0 1rem;line-height:1.6}h1{color:#2563eb}</style>
</head>
<body>
<h1>Google 캘린더 연결 완료</h1>
<p><strong>%s</strong> 계정으로 연결되었습니다. 이 탭은 닫아도 됩니다.</p>
</body>
</html>`, html.EscapeString(email))
}

func respondGoogleOAuthErrorHTML(writer http.ResponseWriter, status int, message string) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.WriteHeader(status)
	fmt.Fprintf(writer, `<!doctype html>
<html lang="ko">
<head><meta charset="utf-8"><title>Google 캘린더 연결 실패</title>
<style>body{font-family:system-ui,sans-serif;margin:4rem auto;max-width:32rem;padding:0 1rem;line-height:1.6}h1{color:#dc2626}</style>
</head>
<body>
<h1>Google 캘린더 연결 실패</h1>
<p>%s</p>
</body>
</html>`, html.EscapeString(message))
}
