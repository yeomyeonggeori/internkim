package companion

import "testing"

func TestHandoffCriteriaSatisfied(t *testing.T) {
	criteria := HandoffSuccessCriteria{
		URLIncludesAny:  []string{"app"},
		URLExcludesAny:  []string{"login"},
		TextIncludesAny: []string{"dashboard"},
		TextExcludesAny: []string{"sign in"},
	}

	if !HandoffCriteriaSatisfied(criteria, "https://example.com/app", "Dashboard ready") {
		t.Fatal("expected criteria to pass")
	}
	if HandoffCriteriaSatisfied(criteria, "https://example.com/login", "Dashboard ready") {
		t.Fatal("expected excluded url to fail")
	}
	if HandoffCriteriaSatisfied(criteria, "https://example.com/app", "Sign in") {
		t.Fatal("expected excluded text to fail")
	}
}

func TestHandoffStoreRejectsWrongCompletion(t *testing.T) {
	store := NewBrowserHandoffStore()
	snapshot, errorValue := store.Begin(BrowserHandoffRequest{URL: "https://example.com/login"}, "internkim")
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := store.Complete(HandoffCompletion{HandoffID: "wrong", SessionID: snapshot.SessionID, URL: "https://example.com/login"}); errorValue == nil {
		t.Fatal("expected wrong handoff id to fail")
	}
	if errorValue := store.Complete(HandoffCompletion{HandoffID: snapshot.HandoffID, SessionID: "other", URL: "https://example.com/login"}); errorValue == nil {
		t.Fatal("expected wrong session to fail")
	}
	if errorValue := store.Complete(HandoffCompletion{HandoffID: snapshot.HandoffID, SessionID: snapshot.SessionID, URL: "https://other.example/login"}); errorValue == nil {
		t.Fatal("expected wrong origin to fail")
	}
}
