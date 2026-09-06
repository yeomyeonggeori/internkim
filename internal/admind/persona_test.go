package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPersonaSchemasAreTheOnesBlueclawOwns(t *testing.T) {
	for fileName, embedded := range map[string][]byte{
		"identity.schema.json": identitySchemaDocument,
		"soul.schema.json":     soulSchemaDocument,
		"user.schema.json":     userSchemaDocument,
	} {
		canonical, errorValue := os.ReadFile(filepath.Join("..", "..", ".dependency", "blueclaw", "internal", "persona", "schema", fileName))
		if errorValue != nil {
			t.Fatalf("expected blueclaw's %s: %v", fileName, errorValue)
		}
		if !bytes.Equal(canonical, embedded) {
			t.Fatalf("admind's copy of %s drifted from blueclaw's; copy it verbatim", fileName)
		}
	}
}

func personaTestService(t *testing.T) *Service {
	t.Helper()
	configurationDirectory := t.TempDir()
	blueclaw := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = io.Copy(response, request.Body)
	}))
	t.Cleanup(blueclaw.Close)
	return NewService(Configuration{
		BlueclawBaseURL:          blueclaw.URL,
		IdentityDocumentPath:     filepath.Join(configurationDirectory, "identity.json"),
		SoulDocumentPath:         filepath.Join(configurationDirectory, "soul.json"),
		BlueclawWorkspacePath:    t.TempDir(),
		AdminEmailPath:           writeTestFile(t, "admin@example.com"),
		CentralPlaneAgentKeyPath: writeTestFile(t, "synthetic-persona-key"),
	})
}

func stubPersonaActor(t *testing.T, actor personaActor, found bool) {
	t.Helper()
	previous := resolvePersonaActor
	resolvePersonaActor = func(*Service, *http.Request) (personaActor, bool, error) { return actor, found, nil }
	t.Cleanup(func() { resolvePersonaActor = previous })
}

func TestPersonaSeedsBothConfigurationDocuments(t *testing.T) {
	service := personaTestService(t)

	identity, soul, errorValue := service.loadOrSeedPersona()

	if errorValue != nil {
		t.Fatalf("expected the persona to seed: %v", errorValue)
	}
	if len(identity.Names) < 2 || identity.Names[0] != "김인턴" || identity.Names[1] != "Intern Kim" || identity.Handle != "internkim" {
		t.Fatalf("expected the default identity, got %+v", identity)
	}
	if soul.Language == nil || soul.Language.Default != "ko" || !soul.Language.MatchRequester || len(soul.Values) == 0 {
		t.Fatalf("expected the default soul, got %+v", soul)
	}
	for _, fileName := range []string{service.Configuration.IdentityDocumentPath, service.Configuration.SoulDocumentPath} {
		document, errorValue := os.ReadFile(fileName)
		if errorValue != nil || !strings.Contains(string(document), `"schemaVersion": 1`) {
			t.Fatalf("expected the workspace %s in canonical form: %v %s", fileName, errorValue, document)
		}
	}
}

func TestPersonaMigratesTheLegacyBotProfileButKeepsTheFirstName(t *testing.T) {
	service := personaTestService(t)
	writeFile(t, filepath.Join(filepath.Dir(service.Configuration.IdentityDocumentPath), "bot-profile.yaml"), `username: "internkim"
displayName: "김비서"
englishDisplayName: "Kim Secretary"
aliases:
  - "비서"
  - "비서"
publicDescription: "회사 일을 돕는 인턴입니다."
identityExtension: "Be crisp."
`)
	for _, fileName := range []string{"BOT_PROFILE.yaml", "BOT_PROFILE.md", "IDENTITY.md", "SOUL.md"} {
		writeFile(t, filepath.Join(service.Configuration.BlueclawWorkspacePath, fileName), "retired persona\n")
	}

	identity, _, errorValue := service.loadOrSeedPersona()

	if errorValue != nil {
		t.Fatalf("expected the migration to run: %v", errorValue)
	}
	if strings.Join(identity.Names, "|") != "김인턴|Kim Secretary|비서" {
		t.Fatalf("expected the first name by rule and the legacy names after it, got %v", identity.Names)
	}
	if identity.Introduction != "회사 일을 돕는 인턴입니다." {
		t.Fatalf("expected the public description to become the introduction, got %+v", identity)
	}
	for _, fileName := range []string{"BOT_PROFILE.yaml", "BOT_PROFILE.md", "IDENTITY.md", "SOUL.md"} {
		if _, errorValue := os.Stat(filepath.Join(service.Configuration.BlueclawWorkspacePath, fileName)); !os.IsNotExist(errorValue) {
			t.Fatalf("retired persona remains: %s", fileName)
		}
	}
}

func TestIdentityRulesRefuseAnotherFirstNameOrHandle(t *testing.T) {
	for name, document := range map[string]string{
		"another first name": `{"schemaVersion": 1, "names": ["김비서", "김인턴"]}`,
		"another handle":     `{"schemaVersion": 1, "names": ["김인턴"], "handle": "kimbot"}`,
		"an unknown field":   `{"schemaVersion": 1, "names": ["김인턴"], "nickname": "kim"}`,
		"a secret":           `{"schemaVersion": 1, "names": ["김인턴"], "introduction": "my key is sk-abc"}`,
	} {
		if _, errorValue := parseIdentityDocument([]byte(document)); errorValue == nil {
			t.Fatalf("expected %s to be refused", name)
		}
	}
	identity, errorValue := parseIdentityDocument([]byte(`{"schemaVersion": 1, "names": ["김인턴", " 인턴킴 "]}`))
	if errorValue != nil || identity.Handle != "" || identity.Names[1] != "인턴킴" {
		t.Fatalf("expected a handle-less identity with trimmed names, got %+v (%v)", identity, errorValue)
	}
}

func TestSoulUpdateIsReadOnly(t *testing.T) {
	service := personaTestService(t)
	for name, body := range map[string]string{"valid": `{"schemaVersion": 1, "values": ["Lead with the result."]}`, "invalid": `{"schemaVersion": 1, "mood": "happy"}`} {
		recorder := httptest.NewRecorder()
		service.updateSoul(recorder, httptest.NewRequest(http.MethodPut, "/soul", strings.NewReader(body)))
		if recorder.Code != http.StatusConflict {
			t.Fatalf("expected %s to be refused as read-only, got %d %s", name, recorder.Code, recorder.Body.String())
		}
	}
	if _, errorValue := os.Stat(service.Configuration.SoulDocumentPath); !os.IsNotExist(errorValue) {
		t.Fatalf("read-only update changed the local soul: %v", errorValue)
	}
}

func TestPersonaAPILetsAMemberChangeTheirOwnDocumentButNotTheSoul(t *testing.T) {
	var blueclawRequests []string
	var storedDocument []byte
	blueclaw := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		blueclawRequests = append(blueclawRequests, request.Method+" "+request.URL.RequestURI())
		responseWriter.Header().Set("Content-Type", "application/json")
		switch request.Method {
		case http.MethodPut:
			storedDocument, _ = io.ReadAll(request.Body)
			_, _ = responseWriter.Write(storedDocument)
		default:
			if storedDocument == nil {
				_, _ = responseWriter.Write([]byte(`{"schemaVersion":1}`))
				return
			}
			_, _ = responseWriter.Write(storedDocument)
		}
	}))
	defer blueclaw.Close()
	service := personaTestService(t)
	service.Configuration.BlueclawBaseURL = blueclaw.URL
	stubPersonaActor(t, personaActor{email: "sample@example.com", personID: "person-1", isAdmin: false}, true)

	recorder := httptest.NewRecorder()
	service.handlePersona(recorder, httptest.NewRequest(http.MethodPost, "/persona/api/soul", strings.NewReader(`{"schemaVersion": 1}`)))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected a member to be refused the soul, got %d", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	service.handlePersona(recorder, httptest.NewRequest(http.MethodPost, "/persona/api/user", strings.NewReader(`{"schemaVersion": 1, "callMe": " 샘플님 ", "preferences": ["Give me the command first."], "language": {"default": "en"}}`)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected the member's document to save, got %d %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(string(storedDocument), `"callMe": "샘플님"`) {
		t.Fatalf("expected the canonical document to reach blueclaw, got %s", storedDocument)
	}

	recorder = httptest.NewRecorder()
	service.handlePersona(recorder, httptest.NewRequest(http.MethodGet, "/persona/api/user", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "Give me the command first.") {
		t.Fatalf("expected the member to read their document back, got %d %s", recorder.Code, recorder.Body.String())
	}
	for _, requested := range blueclawRequests {
		if !strings.Contains(requested, "/admin/api/persona/user?personID=person-1") {
			t.Fatalf("expected every call to name the member's own person, got %v", blueclawRequests)
		}
	}

	recorder = httptest.NewRecorder()
	service.handlePersona(recorder, httptest.NewRequest(http.MethodPost, "/persona/api/user", strings.NewReader(`{"schemaVersion": 1, "language": {"default": "ko", "matchRequester": true}}`)))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected a field the user schema does not name to be refused, got %d", recorder.Code)
	}
	if len(blueclawRequests) != 2 {
		t.Fatalf("expected a refused document never to reach blueclaw, got %v", blueclawRequests)
	}
}

func TestPersonaAPIDoesNotLetAnAdministratorChangeTheSoul(t *testing.T) {
	service := personaTestService(t)
	blueclaw := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodPut {
			_, _ = io.Copy(responseWriter, request.Body)
			return
		}
		_, _ = responseWriter.Write([]byte(`{"identity":{"schemaVersion":1,"names":["김인턴"],"handle":"internkim"},"soul":{"schemaVersion":1,"values":["Say what you did not do."]}}`))
	}))
	defer blueclaw.Close()
	service.Configuration.BlueclawBaseURL = blueclaw.URL
	stubPersonaActor(t, personaActor{email: "admin@example.com", personID: "person-admin", isAdmin: true}, true)

	recorder := httptest.NewRecorder()
	service.handlePersona(recorder, httptest.NewRequest(http.MethodPost, "/persona/api/soul", strings.NewReader(`{"schemaVersion": 1, "values": ["Say what you did not do."]}`)))
	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected the administrator soul update to be read-only, got %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestPersonaAPIRefusesSomeoneTheWorkspaceDoesNotKnow(t *testing.T) {
	service := personaTestService(t)
	stubPersonaActor(t, personaActor{}, false)

	recorder := httptest.NewRecorder()
	service.handlePersona(recorder, httptest.NewRequest(http.MethodGet, "/persona/api/user", nil))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected a stranger to be refused, got %d", recorder.Code)
	}
}

func TestPersonaIdentityUpdateRequiresAdministratorAndPreservesSoul(t *testing.T) {
	service := personaTestService(t)
	if errorValue := service.saveSoul(defaultSoulDocument()); errorValue != nil {
		t.Fatal(errorValue)
	}
	blueclaw := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(`{"identity":{"schemaVersion":1,"names":["김인턴"],"handle":"internkim"},"soul":{"schemaVersion":1,"values":["Keep current guidance."]}}`))
	}))
	defer blueclaw.Close()
	service.Configuration.BlueclawBaseURL = blueclaw.URL
	stubPersonaActor(t, personaActor{email: "member@example.com", personID: "person-1"}, true)
	recorder := httptest.NewRecorder()
	service.handlePersona(recorder, httptest.NewRequest(http.MethodPost, "/persona/api/identity", strings.NewReader(`{"schemaVersion":1,"names":["김인턴"],"handle":"internkim"}`)))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("member identity update status = %d", recorder.Code)
	}
	stubPersonaActor(t, personaActor{email: "admin@example.com", personID: "person-admin", isAdmin: true}, true)
	recorder = httptest.NewRecorder()
	service.handlePersona(recorder, httptest.NewRequest(http.MethodPost, "/persona/api/identity", strings.NewReader(`{"schemaVersion":1,"names":["김인턴"],"handle":"internkim"}`)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("admin identity update status = %d: %s", recorder.Code, recorder.Body.String())
	}
	if _, errorValue := os.Stat(service.Configuration.SoulDocumentPath); errorValue != nil {
		t.Fatalf("identity update removed soul: %v", errorValue)
	}
}

func TestARosterWriteGivesThePersonADocumentWithTheNameTheDirectoryKnows(t *testing.T) {
	var seededDocuments []string
	var seededPaths []string
	blueclaw := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/admin/api/policy":
			_, _ = responseWriter.Write([]byte(`{"people":[]}`))
		case request.Method == http.MethodPost && request.URL.Path == "/admin/api/persona/user":
			document, _ := io.ReadAll(request.Body)
			seededDocuments = append(seededDocuments, string(document))
			seededPaths = append(seededPaths, request.URL.RequestURI())
			_, _ = responseWriter.Write(document)
		default:
			_, _ = responseWriter.Write([]byte(`{}`))
		}
	}))
	defer blueclaw.Close()
	service := personaTestService(t)
	service.Configuration.BlueclawBaseURL = blueclaw.URL
	service.Configuration.BlueclawPolicyDeliveryPath = filepath.Join(t.TempDir(), "policy.json")

	if errorValue := service.inviteBlueclawPerson(context.Background(), "person-1", "sample@example.com", "샘플 이"); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(seededDocuments) != 1 {
		t.Fatalf("expected the invited person to start with the name the directory knows, got %v", seededDocuments)
	}
	var seededDocument userDocument
	if errorValue := json.Unmarshal([]byte(seededDocuments[0]), &seededDocument); errorValue != nil {
		t.Fatal(errorValue)
	}
	if seededDocument.CallMe != "샘플 님" {
		t.Fatalf("unexpected default form of address: %q", seededDocument.CallMe)
	}
	if len(seededPaths) != 1 || !strings.Contains(seededPaths[0], "personID=person-1") {
		t.Fatalf("expected the seed to name the person, got %v", seededPaths)
	}
}

func TestANameTheDocumentCannotHoldIsNotSeeded(t *testing.T) {
	var seeded []string
	blueclaw := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		seeded = append(seeded, request.URL.RequestURI())
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(`{"schemaVersion":1}`))
	}))
	defer blueclaw.Close()
	service := personaTestService(t)
	service.Configuration.BlueclawBaseURL = blueclaw.URL

	service.seedUserDocument(context.Background(), "person-1", strings.Repeat("샘", 65))
	service.seedUserDocument(context.Background(), "person-1", "   ")
	service.seedUserDocument(context.Background(), "", "이샘플")

	if len(seeded) != 0 {
		t.Fatalf("expected nothing to reach blueclaw, got %v", seeded)
	}
}

func TestDefaultDocumentsKeepEveryLineTheyName(t *testing.T) {
	identity := defaultIdentityDocument()
	normalizedIdentity := normalizeIdentityDocument(identity)
	if len(normalizedIdentity.Names) != len(identity.Names) {
		t.Fatalf("the default identity names %d names but keeps %v", len(identity.Names), normalizedIdentity.Names)
	}
	soul := defaultSoulDocument()
	normalizedSoul := normalizeSoulDocument(soul)
	for name, pair := range map[string][2]int{
		"values":       {len(soul.Values), len(normalizedSoul.Values)},
		"boundaries":   {len(soul.Boundaries), len(normalizedSoul.Boundaries)},
		"workingStyle": {len(soul.WorkingStyle), len(normalizedSoul.WorkingStyle)},
		"tone traits":  {len(soul.Tone.Traits), len(normalizedSoul.Tone.Traits)},
	} {
		if pair[0] != pair[1] {
			t.Fatalf("the default soul names %d %s but keeps %d", pair[0], name, pair[1])
		}
	}
	if _, errorValue := canonicalIdentityDocument(identity); errorValue != nil {
		t.Fatalf("the default identity does not pass its own rules: %v", errorValue)
	}
	if _, errorValue := canonicalSoulDocument(soul); errorValue != nil {
		t.Fatalf("the default soul does not pass its own rules: %v", errorValue)
	}
}
