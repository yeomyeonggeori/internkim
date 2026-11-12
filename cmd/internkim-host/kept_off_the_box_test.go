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
	socketPath := theFileJournalServedFor(t, place, filepath.Join(place.BoxStatePath, "several-boxes"))
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

func TestOnlyABoxHoldingTheCompanyKeyAmongSeveralBoxesJournals(t *testing.T) {
	for _, scenario := range []struct {
		name               string
		holdsTheCompanyKey bool
		boxCount           string
		wantAnswer         string
	}{
		{"no company key, several boxes", false, "several", "unjournaled"},
		{"the company key, one box", true, "one", "unjournaled"},
		{"the company key, a count nobody wrote", true, "", "unjournaled"},
	} {
		place := boxWithAConnectionAndNoCompany(t)
		identity := identityOfTheBoxAt(t, place)
		if errorValue := box.MarkInstalledCompany(place.BoxStatePath, exampleCompanyID); errorValue != nil {
			t.Fatal(errorValue)
		}
		if scenario.holdsTheCompanyKey {
			holdTheCompanyKey(t, place, identity)
		}
		severalBoxesPath := filepath.Join(place.BoxStatePath, "several-boxes")
		if scenario.boxCount != "" {
			if errorValue := box.NoteSeveralBoxes(severalBoxesPath, scenario.boxCount == "several"); errorValue != nil {
				t.Fatal(errorValue)
			}
		}

		answer := askedOverTheSocket(t, theFileJournalServedFor(t, place, severalBoxesPath))

		if answer != scenario.wantAnswer {
			t.Errorf("%s: the file journal answered %q, want %q", scenario.name, answer, scenario.wantAnswer)
		}
	}
}

func TestABoxHoldingTheCompanyKeyAmongSeveralBoxesJournals(t *testing.T) {
	place := boxWithAConnectionAndNoCompany(t)
	identity := identityOfTheBoxAt(t, place)
	if errorValue := box.MarkInstalledCompany(place.BoxStatePath, exampleCompanyID); errorValue != nil {
		t.Fatal(errorValue)
	}
	holdTheCompanyKey(t, place, identity)
	severalBoxesPath := filepath.Join(place.BoxStatePath, "several-boxes")
	if errorValue := box.NoteSeveralBoxes(severalBoxesPath, true); errorValue != nil {
		t.Fatal(errorValue)
	}

	kept, errorValue := whatIsKeptOffTheBox(place, severalBoxesPath)

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !kept.isJournaled || kept.destination.CompanyID != exampleCompanyID || kept.destination.SealingKey != identity.EncryptionPublicKey() {
		t.Fatalf("the box journals %v for %s under %s", kept.isJournaled, kept.destination.CompanyID, kept.destination.SealingKey)
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

func theFileJournalServedFor(t *testing.T, place offsite.ThisBox, severalBoxesPath string) string {
	t.Helper()
	kept, errorValue := whatIsKeptOffTheBox(place, severalBoxesPath)
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
