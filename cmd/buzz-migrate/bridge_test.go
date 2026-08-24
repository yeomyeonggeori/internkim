package main

import (
	"context"
	"encoding/json"
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
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		asked = request.URL.Query().Get("email")
		responseWriter.Write([]byte(`{"secretHex":"` + strings.Repeat("c", 64) + `"}`))
	}))
	defer server.Close()

	secretHex, errorValue := newBridge(server.URL).secretFor(context.Background(), "이샘플@example.com")

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
	if _, errorValue := newBridge("  ").secretFor(context.Background(), "a@example.com"); errorValue == nil {
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
		server := httptest.NewServer(handler)
		if _, errorValue := newBridge(server.URL).secretFor(context.Background(), "a@example.com"); errorValue == nil {
			t.Errorf("%s: the import carried on and would have signed with nothing", name)
		}
		server.Close()
	}
}

// The mirror carries a Buzz message back to Mattermost only through what the
// import recorded: without the channel it drops the message, and without the
// post it delivers a reply as a new message.
func TestTheImportTellsTheBridgeWhatItMade(t *testing.T) {
	posted := map[string]map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		var body map[string]string
		json.NewDecoder(request.Body).Decode(&body)
		posted[request.URL.Path] = body
	}))
	defer server.Close()
	client := newBridge(server.URL)

	if errorValue := client.recordChannel(context.Background(), "buzz-channel", "mm-channel"); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := client.recordMessage(context.Background(), "buzz-event", "mm-post", "mm-channel"); errorValue != nil {
		t.Fatal(errorValue)
	}

	channel := posted["/channel"]
	if channel["buzzChannelId"] != "buzz-channel" || channel["externalChannelId"] != "mm-channel" || channel["platform"] != "mattermost" {
		t.Errorf("channel = %v", channel)
	}
	message := posted["/message"]
	if message["buzzEventId"] != "buzz-event" || message["externalId"] != "mm-post" || message["externalChannelId"] != "mm-channel" {
		t.Errorf("message = %v", message)
	}
}

func TestARefusedRecordIsNotSilent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		http.Error(responseWriter, "no", http.StatusInternalServerError)
	}))
	defer server.Close()
	client := newBridge(server.URL)

	if errorValue := client.recordChannel(context.Background(), "buzz-channel", "mm-channel"); errorValue == nil {
		t.Error("a channel nobody recorded was reported as recorded")
	}
	if errorValue := client.recordMessage(context.Background(), "buzz-event", "mm-post", "mm-channel"); errorValue == nil {
		t.Error("a post nobody recorded was reported as recorded")
	}
}

// The messenger keeps everyone who ever posted; the directory keeps the people
// who work here. Reading the first as the second is how thirty-two probe
// accounts became colleagues in a company of eight.
func TestOnlyTheDirectorysPeopleAreOurs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/people" {
			t.Errorf("asked %s", request.URL.Path)
		}
		responseWriter.Write([]byte(`{"emails":["이샘플@example.com","Bot@Example.com"]}`))
	}))
	defer server.Close()

	companyPeople, errorValue := newBridge(server.URL).people(context.Background())
	if errorValue != nil {
		t.Fatalf("people: %v", errorValue)
	}
	if !companyPeople["이샘플@example.com"] || !companyPeople["bot@example.com"] {
		t.Errorf("the directory's people were not all taken: %v", companyPeople)
	}
	if companyPeople["probemm1780337241@internkim.test"] {
		t.Error("someone the directory never named is one of ours")
	}
}

func TestADirectoryThatNamesNobodyIsNotAnAnswer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		responseWriter.Write([]byte(`{"emails":[]}`))
	}))
	defer server.Close()

	if _, errorValue := newBridge(server.URL).people(context.Background()); errorValue == nil {
		t.Error("an empty directory read as a company with no people, and every profile would go out unpublished")
	}
}

func TestAnImportWithNoBridgeDoesNotDecideWhoWorksHere(t *testing.T) {
	if _, errorValue := newBridge("  ").people(context.Background()); errorValue == nil {
		t.Error("an import with nobody to ask named the people itself")
	}
}

// A channel the company closed is gone from the messenger's list, so the copy
// made while it was open stays open unless something goes looking for it.
func TestAChannelResolvesToItsBuzzCounterpart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/channel/resolve" {
			t.Errorf("asked %s", request.URL.Path)
		}
		var payload map[string]string
		json.NewDecoder(request.Body).Decode(&payload)
		if payload["externalChannelId"] != "mm-channel" || payload["platform"] != mattermostPlatform {
			t.Errorf("resolved %v", payload)
		}
		responseWriter.Write([]byte(`{"buzzChannelId":"11111111-1111-1111-1111-111111111111"}`))
	}))
	defer server.Close()

	buzzChannelID, errorValue := newBridge(server.URL).resolveChannel(context.Background(), "mm-channel")
	if errorValue != nil {
		t.Fatalf("resolve: %v", errorValue)
	}
	if buzzChannelID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("resolved to %q", buzzChannelID)
	}
}

func TestAChannelTheBridgeCannotResolveIsNotClosedByGuess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		responseWriter.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	if _, errorValue := newBridge(server.URL).resolveChannel(context.Background(), "mm-channel"); errorValue == nil {
		t.Error("a refused resolve read as a channel id, and something else would have been closed")
	}
}
