package box

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitlab.com/eastriver/internkim/internal/companyhost"
)

const sampleCompanyID = "00000000-0000-4000-8000-000000000001"

type fakePlane struct {
	isClaimed      bool
	accessToken    string
	sealedModelKey *SealedModelKey
	announcements  int
	claimedWith    []string
}

func (plane *fakePlane) serve(t *testing.T) *httptest.Server {
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
			if !plane.isClaimed {
				http.Error(writer, "this box belongs to no company yet", http.StatusNotFound)
				return
			}
			json.NewEncoder(writer).Encode(Session{
				Configuration: Configuration{
					SchemaVersion: 1,
					AppURL:        "https://intern.example.test",
					Company:       companyhost.Company{ID: sampleCompanyID, Name: "샘플 회사", Slug: "sample"},
					CentralPlane:  companyhost.CentralPlane{ProjectURL: "https://project.example.test", PublishableKey: "publishable"},
					GatewayURL:    "wss://gateway.example.test",
				},
				Session:        HostSession{CompanyID: sampleCompanyID, AccessToken: plane.accessToken, ExpiresAt: time.Now().Add(time.Hour).Unix()},
				SealedModelKey: plane.sealedModelKey,
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
	plane.sealedModelKey = sealTo(t, identityOf(t, places), "sk-or-v1-sample")
	runSteps(t, daemon, 1)
	if len(recorded.requests) != 1 {
		t.Fatalf("installs = %d", len(recorded.requests))
	}
	installed := recorded.requests[0]
	if installed.Connection.AgentKey != "first.session.token" || installed.ModelKey != "sk-or-v1-sample" || installed.Connection.Company.ID != sampleCompanyID {
		t.Fatalf("installed %+v", installed)
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
