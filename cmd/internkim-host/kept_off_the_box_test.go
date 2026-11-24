package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/offsite"
)

const exampleCompanyID = "00000000-0000-4000-8000-000000000001"

func TestAnAttachmentIsStoredOnABoxThatHasItsOwnKeyAndNoCompanyYet(t *testing.T) {
	place := boxWithAConnectionAndNoCompany(t)
	identityOfTheBoxAt(t, place)
	socketPath := theFileJournalServedFor(t, place)
	store := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	defer store.Close()
	upstream, _ := url.Parse(store.URL)
	gate := httptest.NewServer(mediaGate(upstream, socketPath).Handler())
	defer gate.Close()

	request, _ := http.NewRequest(http.MethodPut, gate.URL+"/media/object", strings.NewReader("picture"))
	response, errorValue := http.DefaultClient.Do(request)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if response.StatusCode != http.StatusOK {
		t.Fatalf("the gate answered %d to an attachment stored on a box that journals nothing", response.StatusCode)
	}
}

func TestABoxOfACompanyWithoutTheCompanyKeyJournalsNothing(t *testing.T) {
	place := boxWithAConnectionAndNoCompany(t)
	identityOfTheBoxAt(t, place)
	if errorValue := box.MarkInstalledCompany(place.BoxStatePath, exampleCompanyID); errorValue != nil {
		t.Fatal(errorValue)
	}

	answer := askedOverTheSocket(t, theFileJournalServedFor(t, place))

	if answer != "unjournaled" {
		t.Fatalf("a box that cannot seal to the company key answered %q, want \"unjournaled\"", answer)
	}
}

func TestTheFirstBoxOfACompanyJournalsWithNoOtherBoxToTakeOver(t *testing.T) {
	place := boxWithAConnectionAndNoCompany(t)
	identity := identityOfTheBoxAt(t, place)
	if errorValue := box.MarkInstalledCompany(place.BoxStatePath, exampleCompanyID); errorValue != nil {
		t.Fatal(errorValue)
	}
	holdTheCompanyKey(t, place, identity)

	kept, errorValue := whatIsKeptOffTheBox(place)

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !kept.holdsTheCompanyKey || kept.destination.CompanyID != exampleCompanyID || kept.destination.SealingKey != identity.EncryptionPublicKey() {
		t.Fatalf("the company's only box journals %v for %s under %s", kept.holdsTheCompanyKey, kept.destination.CompanyID, kept.destination.SealingKey)
	}
}

func TestACompanyKeyThatCannotBeReadIsAnErrorAndNotABoxStillWaiting(t *testing.T) {
	place := boxWithAConnectionAndNoCompany(t)
	identity := identityOfTheBoxAt(t, place)
	if errorValue := box.MarkInstalledCompany(place.BoxStatePath, exampleCompanyID); errorValue != nil {
		t.Fatal(errorValue)
	}
	before := filesIn(t, place.BoxStatePath)
	holdTheCompanyKey(t, place, identity)
	for name := range filesIn(t, place.BoxStatePath) {
		if !before[name] {
			if errorValue := os.WriteFile(filepath.Join(place.BoxStatePath, name), []byte("torn"), 0o600); errorValue != nil {
				t.Fatal(errorValue)
			}
		}
	}

	_, _, isConnected, errorValue := place.Destination()

	if errorValue == nil {
		t.Fatalf("a box whose company key cannot be read answered connected=%v and no error", isConnected)
	}
}

func claimOf(identity box.Identity) box.Session {
	claim := box.Session{SealingKey: identity.EncryptionPublicKey()}
	claim.Configuration.Company.ID = exampleCompanyID
	claim.Configuration.Company.Name = "Example Co"
	claim.Configuration.CentralPlane.ProjectURL = "https://project.supabase.co"
	claim.Configuration.CentralPlane.PublishableKey = "publishable"
	return claim
}

func TestABoxTakingOverReachesItsCompanysBackupsWithNoCompanyInstalledAndNoConnectionFile(t *testing.T) {
	boxStatePath := filepath.Join(t.TempDir(), "box")
	place := offsite.ThisBox{BoxStatePath: boxStatePath}
	identity := identityOfTheBoxAt(t, place)
	holdTheCompanyKey(t, place, identity)

	kept, errorValue := keptForTheTakeOver(claimOf(identity), boxStatePath, filepath.Join(t.TempDir(), takeOverSessionFileName))

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if kept.destination.CompanyID != exampleCompanyID || kept.destination.SealingKey != identity.EncryptionPublicKey() || kept.destination.Store.ProjectURL != "https://project.supabase.co" {
		t.Fatalf("a standby holding the company key reaches %+v for the take-over", kept.destination)
	}
}

func TestABoxTakingOverSignsInAsItselfAndNotWithTheSessionTheBackupCarries(t *testing.T) {
	boxStatePath := filepath.Join(t.TempDir(), "box")
	place := offsite.ThisBox{BoxStatePath: boxStatePath}
	identity := identityOfTheBoxAt(t, place)
	holdTheCompanyKey(t, place, identity)
	sessionPath := filepath.Join(t.TempDir(), "backups", takeOverSessionFileName)
	if errorValue := keepTheTakeOverSession(sessionPath, "this-box-own-session"); errorValue != nil {
		t.Fatal(errorValue)
	}

	kept, errorValue := keptForTheTakeOver(claimOf(identity), boxStatePath, sessionPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	token, errorValue := kept.destination.Store.AccessToken()

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if token != "this-box-own-session" {
		t.Fatalf("the take-over signs in with %q, which stops working an hour after the backup was made", token)
	}
	if information, errorValue := os.Stat(sessionPath); errorValue != nil || information.Mode().Perm() != 0o600 {
		t.Fatalf("the session kept for the take-over is not a file only its owner reads: %v", errorValue)
	}
}

func TestABoxWithoutTheCompanyKeyDoesNotTakeTheCompanyOver(t *testing.T) {
	boxStatePath := filepath.Join(t.TempDir(), "box")
	identity := identityOfTheBoxAt(t, offsite.ThisBox{BoxStatePath: boxStatePath})
	claim := claimOf(identity)
	claim.SealingKey = "a-key-another-box-made"

	_, errorValue := keptForTheTakeOver(claim, boxStatePath, filepath.Join(t.TempDir(), takeOverSessionFileName))

	if errorValue == nil {
		t.Fatal("a box that holds no company key was given the company's backups to take over from")
	}
}

func identityOfTheBoxAt(t *testing.T, place offsite.ThisBox) box.Identity {
	t.Helper()
	identity, errorValue := box.LoadOrCreateIdentity(filepath.Join(place.BoxStatePath, "identity.json"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return identity
}

func holdTheCompanyKey(t *testing.T, place offsite.ThisBox, identity box.Identity) {
	t.Helper()
	if _, errorValue := box.KeyringFor(identity, place.BoxStatePath, exampleCompanyID, identity.EncryptionPublicKey(), nil); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func filesIn(t *testing.T, directoryPath string) map[string]bool {
	t.Helper()
	entries, errorValue := os.ReadDir(directoryPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	names := map[string]bool{}
	for _, entry := range entries {
		names[entry.Name()] = true
	}
	return names
}

func theFileJournalServedFor(t *testing.T, place offsite.ThisBox) string {
	t.Helper()
	kept, errorValue := whatIsKeptOffTheBox(place)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	socketDirectory, errorValue := os.MkdirTemp("", "journal")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	socketPath := filepath.Join(socketDirectory, "barrier.sock")
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan error, 1)
	go func() { stopped <- serveTheFileJournal(ctx, kept, socketPath) }()
	t.Cleanup(func() {
		cancel()
		<-stopped
		os.RemoveAll(socketDirectory)
	})
	for range 200 {
		if _, errorValue := os.Stat(socketPath); errorValue == nil {
			return socketPath
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("the file journal never opened its socket at %s", socketPath)
	return ""
}

func askedOverTheSocket(t *testing.T, socketPath string) string {
	t.Helper()
	connection, errorValue := net.Dial("unix", socketPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer connection.Close()
	fmt.Fprintln(connection, "wait")
	answer, errorValue := bufio.NewReader(connection).ReadString('\n')
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return strings.TrimSpace(answer)
}

func boxWithAConnectionAndNoCompany(t *testing.T) offsite.ThisBox {
	t.Helper()
	directory := t.TempDir()
	connectionPath := filepath.Join(directory, "connection.json")
	document := `{"schemaVersion":1,"appURL":"https://company.example.com",` +
		`"company":{"id":"` + exampleCompanyID + `","name":"Example Co","slug":"example"},` +
		`"centralPlane":{"projectURL":"https://project.supabase.co","publishableKey":"publishable"},` +
		`"gatewayURL":"wss://gateway.example.com","agentKey":"` + strings.Repeat("a", 64) + `"}`
	if errorValue := os.WriteFile(connectionPath, []byte(document), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return offsite.ThisBox{BoxStatePath: filepath.Join(directory, "box"), ConnectionPath: connectionPath}
}
