package admind

import (
	"bytes"
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
	return NewService(Configuration{
		IdentityDocumentPath:  filepath.Join(configurationDirectory, "identity.json"),
		SoulDocumentPath:      filepath.Join(configurationDirectory, "soul.json"),
		BlueclawWorkspacePath: t.TempDir(),
		AdminEmailPath:        writeTestFile(t, "admin@example.com"),
	})
}

func TestPersonaSeedsBothDocumentsAndWritesTheWorkspace(t *testing.T) {
	service := personaTestService(t)

	identity, soul, errorValue := service.loadOrSeedPersona()

	if errorValue != nil {
		t.Fatalf("expected the persona to seed: %v", errorValue)
	}
	if identity.Name != "김인턴" || identity.Handle != "internkim" || identity.EnglishName != "Intern Kim" {
		t.Fatalf("expected the default identity, got %+v", identity)
	}
	if soul.Language == nil || soul.Language.Default != "ko" || !soul.Language.MatchRequester {
		t.Fatalf("expected the default soul language, got %+v", soul)
	}
	for _, fileName := range []string{"identity.json", "soul.json"} {
		document, errorValue := os.ReadFile(filepath.Join(service.Configuration.BlueclawWorkspacePath, fileName))
		if errorValue != nil || !strings.Contains(string(document), `"schemaVersion": 1`) {
			t.Fatalf("expected the workspace %s in canonical form: %v %s", fileName, errorValue, document)
		}
	}
}

func TestPersonaMigratesTheLegacyBotProfileButKeepsTheName(t *testing.T) {
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
	writeFile(t, filepath.Join(service.Configuration.BlueclawWorkspacePath, "BOT_PROFILE.yaml"), "displayName: 김비서\n")

	identity, _, errorValue := service.loadOrSeedPersona()

	if errorValue != nil {
		t.Fatalf("expected the migration to run: %v", errorValue)
	}
	if identity.Name != "김인턴" {
		t.Fatalf("the name is fixed by rule, got %q", identity.Name)
	}
	if identity.EnglishName != "Kim Secretary" || len(identity.Aliases) != 1 || identity.Aliases[0] != "비서" || identity.Introduction != "회사 일을 돕는 인턴입니다." {
		t.Fatalf("expected the legacy fields to carry over, got %+v", identity)
	}
	if _, errorValue := os.Stat(filepath.Join(service.Configuration.BlueclawWorkspacePath, "BOT_PROFILE.yaml")); !os.IsNotExist(errorValue) {
		t.Fatal("expected the retired workspace profile to be removed")
	}
}

func TestIdentityUpdateRefusesARenameAndAnUnknownField(t *testing.T) {
	service := personaTestService(t)
	for name, body := range map[string]string{
		"a rename":         `{"schemaVersion": 1, "name": "김비서", "handle": "internkim"}`,
		"another handle":   `{"schemaVersion": 1, "name": "김인턴", "handle": "kimbot"}`,
		"an unknown field": `{"schemaVersion": 1, "name": "김인턴", "handle": "internkim", "nickname": "kim"}`,
		"a secret":         `{"schemaVersion": 1, "name": "김인턴", "handle": "internkim", "introduction": "my key is sk-abc"}`,
	} {
		request := httptest.NewRequest(http.MethodPut, "/identity", strings.NewReader(body))
		recorder := httptest.NewRecorder()
		service.updateIdentity(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected %s to be refused, got %d %s", name, recorder.Code, recorder.Body.String())
		}
	}
	request := httptest.NewRequest(http.MethodPut, "/identity", strings.NewReader(`{"schemaVersion": 1, "name": "김인턴", "handle": "internkim", "emoji": "🐱", "aliases": [" 인턴킴 "]}`))
	recorder := httptest.NewRecorder()
	service.updateIdentity(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"emoji":"🐱"`) {
		t.Fatalf("expected the identity to save, got %d %s", recorder.Code, recorder.Body.String())
	}
	saved, errorValue := os.ReadFile(service.Configuration.IdentityDocumentPath)
	if errorValue != nil || !strings.Contains(string(saved), `"인턴킴"`) || strings.Contains(string(saved), `" 인턴킴 "`) {
		t.Fatalf("expected a normalized canonical identity on disk, got %s (%v)", saved, errorValue)
	}
}

func TestSoulUpdateRefusesWhatTheSchemaDoesNotName(t *testing.T) {
	service := personaTestService(t)
	for name, body := range map[string]string{
		"an unknown register": `{"schemaVersion": 1, "tone": {"register": "shouty"}}`,
		"an unknown field":    `{"schemaVersion": 1, "mood": "happy"}`,
		"a repeated value":    `{"schemaVersion": 1, "values": ["a", "a"]}`,
	} {
		request := httptest.NewRequest(http.MethodPut, "/soul", strings.NewReader(body))
		recorder := httptest.NewRecorder()
		service.updateSoul(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected %s to be refused, got %d %s", name, recorder.Code, recorder.Body.String())
		}
	}
	request := httptest.NewRequest(http.MethodPut, "/soul", strings.NewReader(`{"schemaVersion": 1, "values": ["Lead with the result."], "tone": {"register": "polite"}, "language": {"default": "ko", "matchRequester": true}}`))
	recorder := httptest.NewRecorder()
	service.updateSoul(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected the soul to save, got %d %s", recorder.Code, recorder.Body.String())
	}
	workspaceSoul, errorValue := os.ReadFile(filepath.Join(service.Configuration.BlueclawWorkspacePath, "soul.json"))
	if errorValue != nil || !strings.Contains(string(workspaceSoul), "Lead with the result.") {
		t.Fatalf("expected the workspace soul to follow the save, got %s (%v)", workspaceSoul, errorValue)
	}
}
