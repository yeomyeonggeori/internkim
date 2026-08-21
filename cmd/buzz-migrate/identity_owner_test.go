package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// admind versions a person's subject, keeps the agent apart from its bot
// address, and pins the result. None of that is knowable from the seed, so an
// import that derives its own writes history signed by people nothing else can
// act as.
func TestAPersonsKeyComesFromWhoeverOwnsIt(t *testing.T) {
	asked := ""
	bridge := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		asked = request.URL.Query().Get("email")
		responseWriter.Write([]byte(`{"secretHex":"` + strings.Repeat("c", 64) + `"}`))
	}))
	defer bridge.Close()

	secretHex, errorValue := newIdentityOwner(bridge.URL).secretFor(context.Background(), "이샘플@example.com")

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if asked != "이샘플@example.com" {
		t.Errorf("asked for %q", asked)
	}
	if secretHex != strings.Repeat("c", 64) {
		t.Errorf("secret = %q", secretHex)
	}
}

func TestAnImportWithNoOwnerToAskRefusesToInventOne(t *testing.T) {
	if _, errorValue := newIdentityOwner("  ").secretFor(context.Background(), "a@example.com"); errorValue == nil {
		t.Error("the import made up a key rather than stopping")
	}
}

func TestAnUnansweredKeyStopsTheImport(t *testing.T) {
	for name, handler := range map[string]http.HandlerFunc{
		"refused": func(responseWriter http.ResponseWriter, _ *http.Request) {
			http.Error(responseWriter, "no", http.StatusInternalServerError)
		},
		"empty": func(responseWriter http.ResponseWriter, _ *http.Request) {
			responseWriter.Write([]byte(`{"secretHex":""}`))
		},
	} {
		bridge := httptest.NewServer(handler)
		if _, errorValue := newIdentityOwner(bridge.URL).secretFor(context.Background(), "a@example.com"); errorValue == nil {
			t.Errorf("%s: the import carried on and would have signed with nothing", name)
		}
		bridge.Close()
	}
}
