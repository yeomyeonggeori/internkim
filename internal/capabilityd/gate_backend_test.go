package capabilityd

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

)

// capabilityd reaches everything it does not do itself through one field of its
// configuration naming an address: admind over HTTP or over the requester
// socket, blueclaw, Mattermost, the companion, OpenRouter. There is no other
// shape. So a gate does not need a setup per tool — it needs one stand-in per
// address, and a case that says which addresses its call reaches.
type gateBackend string

const (
	admindOverTheSocket gateBackend = "admind socket"
	admindOverHTTP      gateBackend = "admind"
	blueclawOverHTTP    gateBackend = "blueclaw"
	mattermostOverHTTP  gateBackend = "mattermost"
	companionOverHTTP   gateBackend = "companion"
	openRouterOverHTTP  gateBackend = "openrouter"
)

// What a backend answers, and what it was asked. A case gives the first; the
// gate collects the second so a test can say what the tool actually sent.
type standingIn struct {
	answers func(*http.Request) (int, string)

	mutex sync.Mutex
	asked []askedOfBackend
}

type askedOfBackend struct {
	Method string
	Path   string
	Body   string
}

func answering(document string) *standingIn {
	return &standingIn{answers: func(*http.Request) (int, string) { return http.StatusOK, document }}
}

func answeringPerCall(answers func(*http.Request) (int, string)) *standingIn {
	return &standingIn{answers: answers}
}

func (backend *standingIn) Asked() []askedOfBackend {
	backend.mutex.Lock()
	defer backend.mutex.Unlock()
	return append([]askedOfBackend(nil), backend.asked...)
}

func (backend *standingIn) handler(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		body := make([]byte, 0)
		if request.Body != nil {
			buffer := make([]byte, 1<<20)
			read, _ := request.Body.Read(buffer)
			body = buffer[:read]
		}
		backend.mutex.Lock()
		backend.asked = append(backend.asked, askedOfBackend{Method: request.Method, Path: request.URL.Path, Body: string(body)})
		answers := backend.answers
		backend.mutex.Unlock()

		status, document := answers(request)
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.WriteHeader(status)
		_, _ = responseWriter.Write([]byte(document))
	})
}

// serviceReaching starts a stand-in for each address the case names and points
// the configuration at it. An address nobody named stays unset, so a tool that
// reaches somewhere the case did not declare fails rather than escaping.
func serviceReaching(t *testing.T, reaches map[gateBackend]*standingIn) Service {
	t.Helper()
	configuration := Configuration{AdmindBaseURL: refusingAddress(t, "admind over TCP")}

	for backend, standIn := range reaches {
		switch backend {
		case admindOverTheSocket:
			configuration.AdmindSocketPath = servedOnASocket(t, standIn.handler(t))
		case admindOverHTTP:
			configuration.AdmindBaseURL = servedOnLoopback(t, standIn.handler(t))
		case blueclawOverHTTP:
			configuration.BlueclawBaseURL = servedOnLoopback(t, standIn.handler(t))
		case mattermostOverHTTP:
			configuration.MattermostBaseURL = servedOnLoopback(t, standIn.handler(t))
			configuration.MattermostTokenPath = keyFileHolding(t, "mattermost-bot-token")
		case companionOverHTTP:
			configuration.CompanionBaseURL = servedOnLoopback(t, standIn.handler(t))
		case openRouterOverHTTP:
			address := servedOnLoopback(t, standIn.handler(t))
			configuration.OpenRouterBaseURL = address
			configuration.OpenRouterWebBaseURL = address
			configuration.OpenRouterKeyPath = keyFileHolding(t, "openrouter-key")
		default:
			t.Fatalf("no stand-in knows how to be %s", backend)
		}
	}
	return Service{Configuration: configuration}
}

func servedOnLoopback(t *testing.T, handler http.Handler) string {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server.URL
}

func servedOnASocket(t *testing.T, handler http.Handler) string {
	t.Helper()
	// A unix socket path is bounded at about a hundred characters, and t.TempDir
	// spends most of that on the subtest's name.
	directory, errorValue := os.MkdirTemp("", "gate")
	if errorValue != nil {
		t.Fatalf("make the socket directory: %v", errorValue)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	socketPath := filepath.Join(directory, "admind.sock")
	listener, listenError := net.Listen("unix", socketPath)
	if listenError != nil {
		t.Fatalf("listen on %s: %v", socketPath, listenError)
	}
	server := httptest.NewUnstartedServer(handler)
	_ = server.Listener.Close()
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)
	return socketPath
}

// A stand-in answers as the thing itself, and reaching the thing itself takes a
// credential, so the credential comes with the address rather than being a
// separate arrangement in every case.
//
// An address the case did not name still has to go somewhere, and going
// somewhere that fails the test is how a tool reaching past its declaration is
// caught rather than quietly answered.
func refusingAddress(t *testing.T, what string) string {
	t.Helper()
	return servedOnLoopback(t, http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		t.Errorf("%s %s reached %s, which this case did not say it reaches", request.Method, request.URL.Path, what)
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte(`{}`))
	}))
}

func keyFileHolding(t *testing.T, secret string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "key")
	if errorValue := os.WriteFile(path, []byte(secret+"\n"), 0o600); errorValue != nil {
		t.Fatalf("write %s: %v", path, errorValue)
	}
	return path
}
