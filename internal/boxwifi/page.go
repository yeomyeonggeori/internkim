package boxwifi

import (
	"html/template"
	"net/http"
	"strings"
	"sync"
)

var formTemplate = template.Must(template.New("form").Parse(formTemplateSource))
var joiningTemplate = template.Must(template.New("joining").Parse(joiningTemplateSource))

const formTemplateSource = `<!doctype html>
<html lang="{{.Text.Language}}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Text.FormTitle}}</title>
<style>
body { font-family: -apple-system, BlinkMacSystemFont, sans-serif; max-width: 420px; margin: 2rem auto; padding: 0 1rem; color: #1a1a1a; }
h1 { font-size: 1.25rem; }
label { display: block; margin-top: 1rem; font-weight: 600; }
select, input, button { width: 100%; padding: 0.6rem; margin-top: 0.25rem; font-size: 1rem; box-sizing: border-box; }
button { margin-top: 1.5rem; background: #1a73e8; color: #fff; border: none; border-radius: 6px; padding: 0.8rem; font-weight: 600; }
.notice { background: #fdecea; color: #a30000; padding: 0.75rem; border-radius: 6px; margin-bottom: 1rem; }
</style>
</head>
<body>
<h1>{{.Text.FormHeading}}</h1>
{{if .HasJoinFailed}}<p class="notice">{{.Text.JoinFailure}}</p>{{end}}
<form method="post" action="/join">
<label for="ssid">{{.Text.NetworkLabel}}</label>
<select id="ssid" name="ssid">
<option value="">{{.Text.EnterManually}}</option>
{{range .Networks}}<option value="{{.SSID}}">{{.SSID}}{{if .IsSecured}} {{$.Text.Secured}}{{end}}</option>{{end}}
</select>
<label for="customSSID">{{.Text.CustomNetworkLabel}}</label>
<input id="customSSID" name="customSSID" type="text" autocomplete="off">
<label for="password">{{.Text.PasswordLabel}}</label>
<input id="password" name="password" type="password" autocomplete="off">
<button type="submit">{{.Text.Connect}}</button>
</form>
</body>
</html>`

const joiningTemplateSource = `<!doctype html>
<html lang="{{.Text.Language}}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Text.JoiningTitle}}</title>
<style>
body { font-family: -apple-system, BlinkMacSystemFont, sans-serif; max-width: 420px; margin: 2rem auto; padding: 0 1rem; color: #1a1a1a; }
</style>
</head>
<body>
<h1>{{.Text.JoiningHeading}}</h1>
<p>{{.Text.JoiningInstruction}}</p>
</body>
</html>`

type pageText struct {
	Language           string
	FormTitle          string
	FormHeading        string
	JoinFailure        string
	NetworkLabel       string
	EnterManually      string
	Secured            string
	CustomNetworkLabel string
	PasswordLabel      string
	Connect            string
	JoiningTitle       string
	JoiningHeading     string
	JoiningInstruction string
	BadRequest         string
	NetworkRequired    string
	AlreadySubmitted   string
}

const (
	koreanLanguage  = "ko"
	englishLanguage = "en"
)

var pageTexts = map[string]pageText{
	koreanLanguage: {
		Language:           koreanLanguage,
		FormTitle:          "Kim mini 설정",
		FormHeading:        "사무실 Wi-Fi에 연결하세요",
		JoinFailure:        "연결하지 못했습니다. 비밀번호를 확인하고 다시 시도하세요.",
		NetworkLabel:       "네트워크 선택",
		EnterManually:      "직접 입력",
		Secured:            "(보안)",
		CustomNetworkLabel: "다른 네트워크 이름 (선택 사항)",
		PasswordLabel:      "비밀번호",
		Connect:            "연결",
		JoiningTitle:       "Kim mini 연결 중",
		JoiningHeading:     "연결을 시도하고 있습니다",
		JoiningInstruction: "이제 사무실 Wi-Fi로 돌아가 intern.kim 을 여세요.",
		BadRequest:         "잘못된 요청입니다.",
		NetworkRequired:    "네트워크를 선택하거나 입력하세요.",
		AlreadySubmitted:   "이미 접수되었습니다.",
	},
	englishLanguage: {
		Language:           englishLanguage,
		FormTitle:          "Kim mini setup",
		FormHeading:        "Connect to your office Wi-Fi",
		JoinFailure:        "Could not connect. Check the password and try again.",
		NetworkLabel:       "Choose a network",
		EnterManually:      "Enter manually",
		Secured:            "(secured)",
		CustomNetworkLabel: "Another network name (optional)",
		PasswordLabel:      "Password",
		Connect:            "Connect",
		JoiningTitle:       "Kim mini is connecting",
		JoiningHeading:     "Trying to connect",
		JoiningInstruction: "Switch back to your office Wi-Fi and open intern.kim.",
		BadRequest:         "Invalid request.",
		NetworkRequired:    "Choose or enter a network.",
		AlreadySubmitted:   "Already received.",
	},
}

func textFor(request *http.Request) pageText {
	first, _, _ := strings.Cut(request.Header.Get("Accept-Language"), ",")
	tag, _, _ := strings.Cut(first, ";")
	tag = strings.ToLower(strings.TrimSpace(tag))
	if tag == koreanLanguage || strings.HasPrefix(tag, koreanLanguage+"-") {
		return pageTexts[koreanLanguage]
	}
	return pageTexts[englishLanguage]
}

type formData struct {
	Text          pageText
	Networks      []Network
	HasJoinFailed bool
}

type joiningData struct {
	Text pageText
}

type page struct {
	networks      []Network
	hasJoinFailed bool
	submitted     chan<- submission

	mutex        sync.Mutex
	hasSubmitted bool
}

func newPage(networks []Network, hasJoinFailed bool, submitted chan<- submission) http.Handler {
	return &page{networks: networks, hasJoinFailed: hasJoinFailed, submitted: submitted}
}

func (handler *page) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Cache-Control", "no-store")
	if request.URL.Path == "/" && request.Method == http.MethodGet {
		handler.form(responseWriter, request)
		return
	}
	if request.URL.Path == "/join" && request.Method == http.MethodPost {
		handler.join(responseWriter, request)
		return
	}
	http.Redirect(responseWriter, request, "/", http.StatusFound)
}

func (handler *page) form(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := formData{Text: textFor(request), Networks: handler.networks, HasJoinFailed: handler.hasJoinFailed}
	if errorValue := formTemplate.Execute(responseWriter, data); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
	}
}

func (handler *page) join(responseWriter http.ResponseWriter, request *http.Request) {
	if errorValue := request.ParseForm(); errorValue != nil {
		http.Error(responseWriter, textFor(request).BadRequest, http.StatusBadRequest)
		return
	}
	ssid := strings.TrimSpace(request.PostForm.Get("customSSID"))
	if ssid == "" {
		ssid = request.PostForm.Get("ssid")
	}
	if strings.TrimSpace(ssid) == "" {
		http.Error(responseWriter, textFor(request).NetworkRequired, http.StatusBadRequest)
		return
	}
	password := request.PostForm.Get("password")

	if !handler.claimSubmission() {
		http.Error(responseWriter, textFor(request).AlreadySubmitted, http.StatusConflict)
		return
	}
	handler.submitted <- submission{ssid: ssid, password: password}
	handler.joining(responseWriter, request)
}

func (handler *page) claimSubmission() bool {
	handler.mutex.Lock()
	defer handler.mutex.Unlock()
	if handler.hasSubmitted {
		return false
	}
	handler.hasSubmitted = true
	return true
}

func (handler *page) joining(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "text/html; charset=utf-8")
	if errorValue := joiningTemplate.Execute(responseWriter, joiningData{Text: textFor(request)}); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
	}
}
