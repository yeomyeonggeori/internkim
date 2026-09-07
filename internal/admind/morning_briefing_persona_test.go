package admind

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUserMorningBriefingDefaultsAndCanonicalRoundTrip(t *testing.T) {
	user, errorValue := parseUserDocument([]byte(`{"schemaVersion":1}`))
	if errorValue != nil {
		t.Fatalf("expected a legacy user document to parse: %v", errorValue)
	}
	if user.MorningBriefing == nil || !user.MorningBriefing.Enabled || user.MorningBriefing.Time != "08:00" {
		t.Fatalf("expected morning briefing defaults, got %+v", user.MorningBriefing)
	}
	canonical, errorValue := canonicalUserDocument(user)
	if errorValue != nil {
		t.Fatalf("expected canonical user document: %v", errorValue)
	}
	if !strings.Contains(string(canonical), `"morningBriefing": {
    "enabled": true,
    "time": "08:00"`) {
		t.Fatalf("expected canonical defaults, got %s", canonical)
	}
	reparsed, errorValue := parseUserDocument(canonical)
	if errorValue != nil || reparsed.MorningBriefing == nil || *reparsed.MorningBriefing != *user.MorningBriefing {
		t.Fatalf("expected canonical round trip, got %+v (%v)", reparsed.MorningBriefing, errorValue)
	}
}

func TestUserMorningBriefingPreservesFalseAndTime(t *testing.T) {
	user, errorValue := parseUserDocument([]byte(`{"schemaVersion":1,"morningBriefing":{"enabled":false,"time":"09:30"}}`))
	if errorValue != nil {
		t.Fatalf("expected configured user document to parse: %v", errorValue)
	}
	if user.MorningBriefing == nil || user.MorningBriefing.Enabled || user.MorningBriefing.Time != "09:30" {
		t.Fatalf("expected configured morning briefing, got %+v", user.MorningBriefing)
	}
	partial, errorValue := parseUserDocument([]byte(`{"schemaVersion":1,"morningBriefing":{"time":"07:15"}}`))
	if errorValue != nil || partial.MorningBriefing == nil || !partial.MorningBriefing.Enabled || partial.MorningBriefing.Time != "07:15" {
		t.Fatalf("expected partial morning briefing defaults, got %+v (%v)", partial.MorningBriefing, errorValue)
	}
}

func TestUserMorningBriefingRejectsInvalidBoundaryValues(t *testing.T) {
	for name, document := range map[string]string{
		"invalid time": `{"schemaVersion":1,"morningBriefing":{"time":"9:00"}}`,
		"null object":  `{"schemaVersion":1,"morningBriefing":null}`,
	} {
		if _, errorValue := parseUserDocument([]byte(document)); errorValue == nil {
			t.Fatalf("expected %s to be rejected", name)
		}
	}
}

func TestSeedUserDocumentIncludesMorningBriefingDefaults(t *testing.T) {
	var seeded []byte
	blueclaw := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		seeded, _ = io.ReadAll(request.Body)
		responseWriter.WriteHeader(http.StatusCreated)
	}))
	defer blueclaw.Close()
	service := personaTestService(t)
	service.Configuration.BlueclawBaseURL = blueclaw.URL

	service.seedUserDocument(context.Background(), "person-1", "샘플 님")

	var document userDocument
	if errorValue := json.Unmarshal(seeded, &document); errorValue != nil {
		t.Fatalf("expected seeded user document JSON: %v", errorValue)
	}
	if document.MorningBriefing == nil || !document.MorningBriefing.Enabled || document.MorningBriefing.Time != "08:00" {
		t.Fatalf("expected seeded morning briefing defaults, got %+v", document.MorningBriefing)
	}
}
