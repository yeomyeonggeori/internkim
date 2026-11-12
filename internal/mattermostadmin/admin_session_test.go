package mattermostadmin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type loginCounter struct {
	mutex  sync.Mutex
	logins int
}

func (counter *loginCounter) count() int {
	counter.mutex.Lock()
	defer counter.mutex.Unlock()
	return counter.logins
}

func aMattermostThatCountsLogins(t *testing.T) (*Client, *loginCounter) {
	t.Helper()
	counter := &loginCounter{}
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/api/v4/users/login" {
			counter.mutex.Lock()
			counter.logins++
			token := "session-" + string(rune('a'+counter.logins-1))
			counter.mutex.Unlock()
			responseWriter.Header().Set("Token", token)
			responseWriter.WriteHeader(http.StatusOK)
			return
		}
		responseWriter.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	passwordPath := filepath.Join(t.TempDir(), "mm-admin-pass")
	if errorValue := os.WriteFile(passwordPath, []byte("sample-password"), 0o600); errorValue != nil {
		t.Fatalf("write the admin password: %v", errorValue)
	}
	client := New(Settings{
		BaseURL:           server.URL,
		AdminPasswordPath: passwordPath,
		ReadTrimmedFile: func(path string) string {
			document, errorValue := os.ReadFile(path)
			if errorValue != nil {
				return ""
			}
			return string(document)
		},
	})
	return client, counter
}

func TestAStaleSessionIsNotKeptForever(t *testing.T) {
	client, counter := aMattermostThatCountsLogins(t)

	if _, errorValue := client.AdminToken(context.Background()); errorValue != nil {
		t.Fatalf("first token: %v", errorValue)
	}
	client.session.issuedAt = time.Now().Add(-adminSessionLifetime - time.Minute)
	if _, errorValue := client.AdminToken(context.Background()); errorValue != nil {
		t.Fatalf("token after the lifetime: %v", errorValue)
	}

	if counter.count() != 2 {
		t.Fatalf("a session older than its lifetime was reused; logins: %d", counter.count())
	}
}
