package box

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.com/eastriver/internkim/internal/companyhost"
)

const sampleCompanyID = "00000000-0000-4000-8000-000000000001"

type fakePlane struct {
	isClaimed      bool
	accessToken    string
	announcements  int
	claimedWith    []string
	refreshToken   string
	projectURL     string
	sessionAsked   int
	refreshesAsked []string
	refreshHeaders http.Header
	refuseRefresh  bool

	projectSealedModelKey *SealedModelKey
	modelKeyReads         []string
}

func (plane *fakePlane) serveProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/rest/v1/rpc/box_sealed_model_key" {
			plane.modelKeyReads = append(plane.modelKeyReads, request.Header.Get("Authorization"))
			if request.Header.Get("apikey") != "publishable" {
				t.Errorf("the model key was asked for without the publishable key")
			}
			json.NewEncoder(writer).Encode(plane.projectSealedModelKey)
			return
		}
		if request.URL.Path != "/auth/v1/token" || request.URL.Query().Get("grant_type") != "refresh_token" {
			t.Errorf("unexpected request to the project at %s", request.URL)
			return
		}
		var body map[string]string
		json.NewDecoder(request.Body).Decode(&body)
		plane.refreshesAsked = append(plane.refreshesAsked, body["refresh_token"])
		plane.refreshHeaders = request.Header.Clone()
		if plane.refuseRefresh {
			http.Error(writer, `{"error_code":"refresh_token_not_found"}`, http.StatusBadRequest)
			return
		}
		json.NewEncoder(writer).Encode(map[string]any{
			"access_token":  fmt.Sprintf("refreshed.session.%d", len(plane.refreshesAsked)),
			"refresh_token": fmt.Sprintf("rotated-refresh-%d", len(plane.refreshesAsked)),
			"expires_at":    time.Now().Add(time.Hour).Unix(),
		})
	}))
	t.Cleanup(server.Close)
	plane.projectURL = server.URL
}

func (plane *fakePlane) serve(t *testing.T) *httptest.Server {
	plane.serveProject(t)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/box/announce":
			plane.announcements++
			writer.Write([]byte(`{"isClaimed":false}`))
		case "/api/box/claim":
			var body map[string]string
			json.NewDecoder(request.Body).Decode(&body)
			plane.claimedWith = append(plane.claimedWith, body["connectionKey"])
			plane.isClaimed = true
			writer.Write([]byte(`{"companyID":"` + sampleCompanyID + `"}`))
		case "/api/box/session":
			plane.sessionAsked++
			if !plane.isClaimed {
				http.Error(writer, "this box belongs to no company yet", http.StatusNotFound)
				return
			}
			json.NewEncoder(writer).Encode(Session{
				Configuration: Configuration{
					SchemaVersion: 1,
					AppURL:        "https://intern.example.test",
					Company:       companyhost.Company{ID: sampleCompanyID, Name: "샘플 회사", Slug: "sample"},
					CentralPlane:  companyhost.CentralPlane{ProjectURL: plane.projectURL, PublishableKey: "publishable"},
					GatewayURL:    "wss://gateway.example.test",
				},
				Session: HostSession{CompanyID: sampleCompanyID, AccessToken: plane.accessToken, RefreshToken: plane.refreshToken, ExpiresAt: time.Now().Add(time.Hour).Unix()},
			})
		default:
			t.Errorf("unexpected request to %s", request.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

type installs struct{ requests []companyhost.Request }

func (recorded *installs) install(request companyhost.Request) error {
	recorded.requests = append(recorded.requests, request)
	return nil
}

func daemonFor(t *testing.T, plane *fakePlane, recorded *installs) (Daemon, Places) {
	t.Helper()
	root := t.TempDir()
	places := Places{
		StateDirectoryPath:        filepath.Join(root, "box"),
		ConnectionFilePath:        filepath.Join(root, "current", "connection.json"),
		CredentialPaths:           []string{filepath.Join(root, "agent-key"), filepath.Join(root, "relay-agent-key")},
		ModelKeyPath:              filepath.Join(root, "openrouter-key"),
		CompanyStateDirectoryPath: func(companyID string) string { return filepath.Join(root, "companies", companyID) },
	}
	return Daemon{Client: Client{AppURL: plane.serve(t).URL}, Places: places, Install: recorded.install}, places
}

func runSteps(t *testing.T, daemon Daemon, steps int) {
	t.Helper()
	taken := 0
	daemon.Sleep = func(ctx context.Context, wait time.Duration) error {
		taken++
		if taken >= steps {
			return context.Canceled
		}
		return nil
	}
	if errorValue := daemon.Run(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func sealTo(t *testing.T, identity Identity, modelKey string) *SealedModelKey {
	t.Helper()
	ephemeral, errorValue := ecdh.X25519().GenerateKey(rand.Reader)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	boxKey := identity.encryptionKey.PublicKey()
	sharedSecret, errorValue := ephemeral.ECDH(boxKey)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	sealing, errorValue := modelKeySealing(sharedSecret, ephemeral.PublicKey().Bytes(), boxKey.Bytes())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	nonce := make([]byte, sealing.NonceSize())
	rand.Read(nonce)
	return &SealedModelKey{
		EphemeralPublicKey: encodedKey(ephemeral.PublicKey().Bytes()),
		Nonce:              base64.RawURLEncoding.EncodeToString(nonce),
		Ciphertext:         base64.RawURLEncoding.EncodeToString(sealing.Seal(nil, nonce, []byte(modelKey), nil)),
	}
}

func identityOf(t *testing.T, places Places) Identity {
	t.Helper()
	identity, errorValue := LoadOrCreateIdentity(filepath.Join(places.StateDirectoryPath, "identity.json"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return identity
}

func TestAnUnclaimedBoxKeepsAnnouncingAndInstallsNothing(t *testing.T) {
	plane := &fakePlane{}
	recorded := &installs{}
	daemon, _ := daemonFor(t, plane, recorded)

	runSteps(t, daemon, 3)

	if plane.announcements != 3 || len(recorded.requests) != 0 {
		t.Fatalf("announcements = %d, installs = %d", plane.announcements, len(recorded.requests))
	}
}

func TestAClaimedBoxWaitsForItsModelKeyBeforeInstalling(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "header.claims.signature"}
	recorded := &installs{}
	daemon, _ := daemonFor(t, plane, recorded)

	runSteps(t, daemon, 2)

	if plane.announcements != 0 || len(recorded.requests) != 0 {
		t.Fatalf("announcements = %d, installs = %d", plane.announcements, len(recorded.requests))
	}
}

func TestAClaimedBoxInstallsOnceThenKeepsItsSessionFresh(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "first.session.token"}
	recorded := &installs{}
	daemon, places := daemonFor(t, plane, recorded)
	plane.projectSealedModelKey = sealTo(t, identityOf(t, places), "sk-or-v1-sample")
	runSteps(t, daemon, 1)
	if len(recorded.requests) != 1 {
		t.Fatalf("installs = %d", len(recorded.requests))
	}
	installed := recorded.requests[0]
	if installed.Connection.AgentKey != "first.session.token" || installed.ModelKey != "sk-or-v1-sample" || installed.Connection.Company.ID != sampleCompanyID {
		t.Fatalf("installed %+v", installed)
	}
	if strings.Join(plane.modelKeyReads, ",") != "Bearer first.session.token" {
		t.Fatalf("the install took its model key from %v, not from the project with the new session", plane.modelKeyReads)
	}
	for _, path := range append([]string{places.ModelKeyPath}, places.CredentialPaths...) {
		if errorValue := os.WriteFile(path, []byte("installed\n"), 0o640); errorValue != nil {
			t.Fatal(errorValue)
		}
	}

	plane.accessToken = "second.session.token"
	runSteps(t, daemon, 1)

	if len(recorded.requests) != 1 {
		t.Fatalf("a renewal reinstalled the server: installs = %d", len(recorded.requests))
	}
	for _, path := range places.CredentialPaths {
		requireFile(t, path, "second.session.token\n", 0o640)
	}
	requireFile(t, places.ModelKeyPath, "sk-or-v1-sample\n", 0o640)
}

func requireFile(t *testing.T, path, contents string, mode os.FileMode) {
	t.Helper()
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	information, errorValue := os.Stat(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if string(document) != contents || information.Mode().Perm() != mode {
		t.Fatalf("%s = %q mode %o, want %q mode %o", path, document, information.Mode().Perm(), contents, mode)
	}
}

func TestAComputerConnectedWithAFileIsNotAnnouncedAsEmpty(t *testing.T) {
	plane := &fakePlane{}
	recorded := &installs{}
	daemon, places := daemonFor(t, plane, recorded)
	if errorValue := os.MkdirAll(filepath.Dir(places.ConnectionFilePath), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(places.ConnectionFilePath, []byte("{}"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	errorValue := daemon.Run(context.Background())

	if !errors.Is(errorValue, ErrConnectedByFile) || plane.announcements != 0 {
		t.Fatalf("error = %v, announcements = %d", errorValue, plane.announcements)
	}
}

func TestAConnectionFileClaimsThisComputerAndInstallsFromItsSession(t *testing.T) {
	plane := &fakePlane{accessToken: "file.session.token"}
	recorded := &installs{}
	daemon, _ := daemonFor(t, plane, recorded)

	if errorValue := daemon.InstallWithConnectionFile(context.Background(), "connection-file-key", "sk-or-v1-typed"); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(plane.claimedWith) != 1 || plane.claimedWith[0] != "connection-file-key" {
		t.Fatalf("claimed with %v", plane.claimedWith)
	}
	if len(recorded.requests) != 1 || recorded.requests[0].Connection.AgentKey != "file.session.token" || recorded.requests[0].ModelKey != "sk-or-v1-typed" {
		t.Fatalf("installed %+v", recorded.requests)
	}
}

func TestAComputerInstalledFromAFileKeepsRenewingWithNoSealedModelKey(t *testing.T) {
	plane := &fakePlane{accessToken: "file.session.token"}
	recorded := &installs{}
	daemon, places := daemonFor(t, plane, recorded)
	if errorValue := daemon.InstallWithConnectionFile(context.Background(), "connection-file-key", "sk-or-v1-typed"); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, path := range append([]string{places.ModelKeyPath}, places.CredentialPaths...) {
		if errorValue := os.WriteFile(path, []byte("installed\n"), 0o640); errorValue != nil {
			t.Fatal(errorValue)
		}
	}

	plane.accessToken = "renewed.session.token"
	runSteps(t, daemon, 1)

	for _, path := range places.CredentialPaths {
		requireFile(t, path, "renewed.session.token\n", 0o640)
	}
	requireFile(t, places.ModelKeyPath, "installed\n", 0o640)
}

func installingAndStoring(t *testing.T, plane *fakePlane) (Daemon, Places, *installs) {
	t.Helper()
	recorded := &installs{}
	daemon, places := daemonFor(t, plane, recorded)
	daemon.Install = func(request companyhost.Request) error {
		recorded.requests = append(recorded.requests, request)
		_, errorValue := companyhost.PrepareStateDirectory(request.StateDirectoryPath, *request.Connection)
		return errorValue
	}
	plane.projectSealedModelKey = sealTo(t, identityOf(t, places), "sk-or-v1-sample")
	runSteps(t, daemon, 1)
	if len(recorded.requests) != 1 {
		t.Fatalf("installs = %d", len(recorded.requests))
	}
	for _, path := range places.CredentialPaths {
		if errorValue := os.WriteFile(path, []byte("installed\n"), 0o640); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	if errorValue := os.WriteFile(places.ModelKeyPath, []byte("sk-or-v1-sample\n"), 0o640); errorValue != nil {
		t.Fatal(errorValue)
	}
	return daemon, places, recorded
}

func readStoredConnection(t *testing.T, places Places) []byte {
	t.Helper()
	document, errorValue := os.ReadFile(companyhost.StoredConnectionPath(places.CompanyStateDirectoryPath(sampleCompanyID)))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return document
}

func TestAnInstalledBoxRenewsWithItsRefreshTokenAndKeepsTheRotatedPair(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "first.session.token", refreshToken: "first-refresh"}
	daemon, places, recorded := installingAndStoring(t, plane)
	requireFile(t, filepath.Join(places.StateDirectoryPath, refreshTokenFileName), "first-refresh\n", 0o600)
	storedAtInstall := readStoredConnection(t, places)
	sessionsBeforeRenewal := plane.sessionAsked

	runSteps(t, daemon, 1)
	runSteps(t, daemon, 1)

	if plane.sessionAsked != sessionsBeforeRenewal {
		t.Fatalf("renewal asked the web for a session %d more times", plane.sessionAsked-sessionsBeforeRenewal)
	}
	if strings.Join(plane.refreshesAsked, ",") != "first-refresh,rotated-refresh-1" {
		t.Fatalf("refresh tokens presented: %v", plane.refreshesAsked)
	}
	if plane.refreshHeaders.Get("apikey") != "publishable" {
		t.Fatalf("refresh headers %v", plane.refreshHeaders)
	}
	if len(recorded.requests) != 1 {
		t.Fatalf("a renewal reinstalled the server: installs = %d", len(recorded.requests))
	}
	requireFile(t, filepath.Join(places.StateDirectoryPath, refreshTokenFileName), "rotated-refresh-2\n", 0o600)
	for _, path := range places.CredentialPaths {
		requireFile(t, path, "refreshed.session.2\n", 0o640)
	}
	requireFile(t, places.ModelKeyPath, "sk-or-v1-sample\n", 0o640)
	if string(readStoredConnection(t, places)) != string(storedAtInstall) {
		t.Fatalf("the stored connection changed: %s", readStoredConnection(t, places))
	}
}

func TestTheRefreshTokenStaysOutOfEveryFileTheAgentReads(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "first.session.token", refreshToken: "first-refresh"}
	daemon, places, _ := installingAndStoring(t, plane)
	runSteps(t, daemon, 1)

	keptAt := filepath.Join(places.StateDirectoryPath, refreshTokenFileName)
	holders := []string{}
	for _, root := range []string{filepath.Dir(places.ModelKeyPath), places.StateDirectoryPath} {
		fs.WalkDir(os.DirFS(root), ".", func(path string, entry fs.DirEntry, errorValue error) error {
			if errorValue != nil || entry.IsDir() {
				return errorValue
			}
			document, _ := os.ReadFile(filepath.Join(root, path))
			if strings.Contains(string(document), "rotated-refresh-1") && filepath.Join(root, path) != keptAt {
				holders = append(holders, filepath.Join(root, path))
			}
			return nil
		})
	}
	if len(holders) != 0 {
		t.Fatalf("the refresh token is also in %v", holders)
	}
}

func TestABoxWhoseRefreshTokenIsRefusedAsksTheCompanyForANewSession(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "first.session.token", refreshToken: "first-refresh"}
	daemon, places, recorded := installingAndStoring(t, plane)
	plane.refuseRefresh = true
	plane.accessToken = "bootstrapped.session.token"
	plane.refreshToken = "second-refresh"
	sessionsBeforeRenewal := plane.sessionAsked

	runSteps(t, daemon, 1)

	if plane.sessionAsked != sessionsBeforeRenewal+1 {
		t.Fatalf("sessions asked of the web: %d before, %d after", sessionsBeforeRenewal, plane.sessionAsked)
	}
	requireFile(t, filepath.Join(places.StateDirectoryPath, refreshTokenFileName), "second-refresh\n", 0o600)
	for _, path := range places.CredentialPaths {
		requireFile(t, path, "bootstrapped.session.token\n", 0o640)
	}
	if len(recorded.requests) != 1 {
		t.Fatalf("installs = %d", len(recorded.requests))
	}
}

func TestAChangedModelKeyReachesTheBoxOnItsNextRenewalAndAnUnchangedOneIsNotRewritten(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "first.session.token", refreshToken: "first-refresh"}
	daemon, places, _ := installingAndStoring(t, plane)
	before, errorValue := os.Stat(places.ModelKeyPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	runSteps(t, daemon, 1)

	after, errorValue := os.Stat(places.ModelKeyPath)
	if errorValue != nil || !os.SameFile(before, after) {
		t.Fatalf("an unchanged model key was rewritten: %v", errorValue)
	}
	if strings.Join(plane.modelKeyReads, ",") != "Bearer first.session.token,Bearer refreshed.session.1" {
		t.Fatalf("model key asked for with %v", plane.modelKeyReads)
	}

	plane.projectSealedModelKey = sealTo(t, identityOf(t, places), "sk-or-v1-changed")
	runSteps(t, daemon, 1)

	requireFile(t, places.ModelKeyPath, "sk-or-v1-changed\n", 0o640)
}
