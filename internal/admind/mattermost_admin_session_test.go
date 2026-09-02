package admind

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type mattermostLoginCounter struct {
	mutex   sync.Mutex
	logins  int
	tokens  []string
	refuses bool
}

func (counter *mattermostLoginCounter) count() int {
	counter.mutex.Lock()
	defer counter.mutex.Unlock()
	return counter.logins
}

func (counter *mattermostLoginCounter) refuseEverything(refuses bool) {
	counter.mutex.Lock()
	defer counter.mutex.Unlock()
	counter.refuses = refuses
}

func aMattermostThatCountsLogins(t *testing.T) (*Service, *mattermostLoginCounter) {
	t.Helper()
	counter := &mattermostLoginCounter{}
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/api/v4/users/login" {
			counter.mutex.Lock()
			counter.logins++
			token := "session-" + string(rune('a'+counter.logins-1))
			counter.tokens = append(counter.tokens, token)
			counter.mutex.Unlock()
			responseWriter.Header().Set("Token", token)
			responseWriter.WriteHeader(http.StatusOK)
			return
		}
		counter.mutex.Lock()
		refuses := counter.refuses
		counter.mutex.Unlock()
		if refuses {
			responseWriter.WriteHeader(http.StatusUnauthorized)
			return
		}
		responseWriter.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	passwordPath := filepath.Join(t.TempDir(), "mm-admin-pass")
	if errorValue := os.WriteFile(passwordPath, []byte("sample-password"), 0o600); errorValue != nil {
		t.Fatalf("write the admin password: %v", errorValue)
	}
	service := &Service{Configuration: Configuration{
		MattermostBaseURL:           server.URL,
		MattermostAdminPasswordPath: passwordPath,
	}}
	return service, counter
}

func TestTheAdminSessionIsReusedAcrossCalls(t *testing.T) {
	service, counter := aMattermostThatCountsLogins(t)

	for attempt := 0; attempt < 50; attempt++ {
		if _, errorValue := service.mattermostAdmin().AdminToken(context.Background()); errorValue != nil {
			t.Fatalf("attempt %d: %v", attempt, errorValue)
		}
	}

	if counter.count() != 1 {
		t.Fatalf("50 calls signed in %d times; every login evicts an older Mattermost session", counter.count())
	}
}

func TestConcurrentCallersShareOneAdminSession(t *testing.T) {
	service, counter := aMattermostThatCountsLogins(t)

	var waiting sync.WaitGroup
	for caller := 0; caller < 20; caller++ {
		waiting.Add(1)
		go func() {
			defer waiting.Done()
			_, _ = service.mattermostAdmin().AdminToken(context.Background())
		}()
	}
	waiting.Wait()

	if counter.count() != 1 {
		t.Fatalf("20 concurrent callers signed in %d times", counter.count())
	}
}

func TestARefusedSessionIsReplaced(t *testing.T) {
	service, counter := aMattermostThatCountsLogins(t)

	firstToken, errorValue := service.mattermostAdmin().AdminToken(context.Background())
	if errorValue != nil {
		t.Fatalf("first token: %v", errorValue)
	}

	counter.refuseEverything(true)
	if errorValue := service.mattermostAdmin().Request(context.Background(), http.MethodGet, "/api/v4/users/me", firstToken, nil, nil); errorValue == nil {
		t.Fatal("the refused request reported success")
	}
	counter.refuseEverything(false)

	secondToken, errorValue := service.mattermostAdmin().AdminToken(context.Background())
	if errorValue != nil {
		t.Fatalf("second token: %v", errorValue)
	}
	if secondToken == firstToken {
		t.Fatal("the token Mattermost refused was handed out again")
	}
	if counter.count() != 2 {
		t.Fatalf("expected exactly one replacement login, got %d in total", counter.count())
	}
}
