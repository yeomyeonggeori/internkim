package centralplane

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func TestSettingsNeedEveryPart(t *testing.T) {
	complete := Settings{AppURL: "https://app", HostCredential: func() string { return "key" }, ProjectURL: "https://plane", PublishableKey: "publishable"}
	if !complete.Configured() {
		t.Fatal("a complete setting should be usable")
	}
	for _, missing := range []Settings{
		{HostCredential: func() string { return "key" }, ProjectURL: "https://plane", PublishableKey: "publishable"},
		{AppURL: "https://app", ProjectURL: "https://plane", PublishableKey: "publishable"},
		{AppURL: "https://app", HostCredential: func() string { return "key" }, PublishableKey: "publishable"},
		{AppURL: "https://app", HostCredential: func() string { return "key" }, ProjectURL: "https://plane"},
	} {
		if missing.Configured() {
			t.Fatalf("a setting missing a part must not be usable: %+v", missing)
		}
	}
}

func TestEveryRequestCarriesTheCredentialHeldAtThatMoment(t *testing.T) {
	presented := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		presented = append(presented, request.Header.Get("Authorization"))
		writer.Write([]byte(`{"company":{"name":"이샘플"}}`))
	}))
	defer server.Close()
	held := "first-session"
	client := New(Settings{AppURL: server.URL, HostCredential: func() string { return held }, ProjectURL: server.URL, PublishableKey: "publishable"})

	for _, next := range []string{"second-session", ""} {
		if _, _, errorValue := client.Company(context.Background()); errorValue != nil {
			t.Fatal(errorValue)
		}
		held = next
	}

	if want := []string{"Bearer first-session", "Bearer second-session"}; !slices.Equal(presented, want) {
		t.Fatalf("presented %v, want %v", presented, want)
	}
}
