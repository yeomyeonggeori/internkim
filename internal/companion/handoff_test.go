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
	if errorValue := store.Complete(HandoffCompletion{HandoffID: snapshot.HandoffID, SessionID: snapshot.SessionID, URL: "not a url"}); errorValue == nil {
		t.Fatal("expected invalid url to fail")
	}
}

func TestHandoffStoreCompletionIsIdempotent(t *testing.T) {
	store := NewBrowserHandoffStore()
	snapshot, errorValue := store.Begin(BrowserHandoffRequest{URL: "https://example.com/login"}, "internkim")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	completion := HandoffCompletion{HandoffID: snapshot.HandoffID, SessionID: snapshot.SessionID, URL: "https://example.com/app"}
	if errorValue := store.Complete(completion); errorValue != nil {
		t.Fatalf("expected first completion to succeed: %v", errorValue)
	}
	if store.Snapshot().Active {
		t.Fatal("expected completed handoff overlay to close")
	}
	if errorValue := store.Complete(completion); errorValue != nil {
		t.Fatalf("expected duplicate completion to be ignored: %v", errorValue)
	}
}

func TestPersistentHandoffStoreRestoresActiveHandoff(t *testing.T) {
	statePath := t.TempDir() + "/browser-handoff.json"
	store := NewPersistentBrowserHandoffStore(statePath)
	handoff, errorValue := store.Begin(BrowserHandoffRequest{
		URL:          "https://example.com/login",
		Message:      "로그인 후 완료를 눌러주세요.",
		ResumePrompt: "계속 진행하세요.",
	}, "internkim")
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	reloadedStore := NewPersistentBrowserHandoffStore(statePath)
	snapshot := reloadedStore.Snapshot()

	if !snapshot.Active || snapshot.HandoffID != handoff.HandoffID || snapshot.ResumePrompt != "계속 진행하세요." {
		t.Fatalf("expected active handoff to restore, got %+v", snapshot)
	}
	if errorValue := reloadedStore.Complete(HandoffCompletion{HandoffID: handoff.HandoffID, SessionID: handoff.SessionID, URL: "https://example.com/app"}); errorValue != nil {
		t.Fatalf("expected restored handoff completion: %v", errorValue)
	}
	if NewPersistentBrowserHandoffStore(statePath).Snapshot().Active {
		t.Fatal("expected completed handoff state file to clear")
	}
}
