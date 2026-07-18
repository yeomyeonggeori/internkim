package admind

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

type recordingGoogleOAuthLock struct {
	name    string
	actions *[]string
}

func (lock *recordingGoogleOAuthLock) Lock() {
	*lock.actions = append(*lock.actions, lock.name+"-lock")
}

func (lock *recordingGoogleOAuthLock) Unlock() {
	*lock.actions = append(*lock.actions, lock.name+"-unlock")
}

func TestGoogleOAuthAccountStateUsesRemoteThenTokenLockOrder(t *testing.T) {
	actions := []string{}
	remoteLock := &recordingGoogleOAuthLock{name: "remote", actions: &actions}
	tokenLock := &recordingGoogleOAuthLock{name: "token", actions: &actions}

	unlock := lockGoogleOAuthAccountState(remoteLock, tokenLock)
	unlock()

	expected := []string{"remote-lock", "token-lock", "token-unlock", "remote-unlock"}
	if !reflect.DeepEqual(actions, expected) {
		t.Fatalf("lock actions = %#v, expected %#v", actions, expected)
	}
}

func TestPersistingGoogleOAuthTokenSourceDoesNotRecreateRemovedAccountToken(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account, errorValue := service.saveGoogleOAuthTokenAndAccount(ctx, &oauth2.Token{
		AccessToken: "old-access-token",
		TokenType:   "Bearer",
		Expiry:      time.Now().UTC().Add(time.Hour),
	}, "removed@example.com")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.resetGoogleOAuthAccountConnection(ctx); errorValue != nil {
		t.Fatal(errorValue)
	}
	source := &persistingGoogleOAuthTokenSource{
		service: service,
		account: account,
		inner: oauth2.StaticTokenSource(&oauth2.Token{
			AccessToken: "new-access-token",
			TokenType:   "Bearer",
			Expiry:      time.Now().UTC().Add(2 * time.Hour),
		}),
	}

	_, errorValue = source.Token()

	if errorValue == nil || !strings.Contains(errorValue.Error(), "was removed") {
		t.Fatalf("token source error=%v", errorValue)
	}
	if _, errorValue := os.Stat(account.TokenFilePath); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("removed account token file error=%v", errorValue)
	}
}
