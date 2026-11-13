package filestream

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/hostbackup"
	"github.com/yeomyeonggeori/internkim/internal/offsite"
)

const companyID = "00000000-0000-4000-8000-0000000000c1"

type keptObjects struct {
	mutex       sync.Mutex
	objects     map[string][]byte
	entriesRead int
}

func (kept *keptObjects) serve(t *testing.T) offsite.Store {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		kept.mutex.Lock()
		defer kept.mutex.Unlock()
		path := strings.TrimPrefix(request.URL.EscapedPath(), "/storage/v1")
		switch {
		case request.Method == http.MethodPost && strings.HasPrefix(path, "/object/list/"+offsite.Bucket):
			kept.list(writer, request)
		case request.Method == http.MethodPost && strings.HasPrefix(path, "/object/"+offsite.Bucket+"/"):
			body, _ := io.ReadAll(request.Body)
			kept.objects[strings.TrimPrefix(path, "/object/"+offsite.Bucket+"/")] = body
			writer.Write([]byte(`{}`))
		case request.Method == http.MethodGet && strings.HasPrefix(path, "/object/authenticated/"+offsite.Bucket+"/"):
			name := strings.TrimPrefix(path, "/object/authenticated/"+offsite.Bucket+"/")
			if strings.Contains(name, "/"+journalFolder+"/") {
				kept.entriesRead++
			}
			body, isKept := kept.objects[name]
			if !isKept {
				http.Error(writer, "not found", http.StatusBadRequest)
				return
			}
			writer.Write(body)
		case request.Method == http.MethodDelete && path == "/object/"+offsite.Bucket:
			var body struct{ Prefixes []string }
			json.NewDecoder(request.Body).Decode(&body)
			for _, removed := range body.Prefixes {
				delete(kept.objects, removed)
			}
			writer.Write([]byte(`[]`))
		default:
			t.Errorf("unexpected %s %s", request.Method, path)
		}
	}))
	t.Cleanup(server.Close)
	return offsite.Store{ProjectURL: server.URL, PublishableKey: "publishable", AccessToken: func() (string, error) { return "host.session.token", nil }}
}

func (kept *keptObjects) list(writer http.ResponseWriter, request *http.Request) {
	var asked struct {
		Prefix string `json:"prefix"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}
	json.NewDecoder(request.Body).Decode(&asked)
	names := []string{}
	for path := range kept.objects {
		if rest, isUnder := strings.CutPrefix(path, asked.Prefix); isUnder && !strings.Contains(rest, "/") {
			names = append(names, rest)
		}
	}
	slices.Sort(names)
	page := []offsite.StoredObject{}
	for index := asked.Offset; index < len(names) && index < asked.Offset+asked.Limit; index++ {
		page = append(page, offsite.StoredObject{Name: names[index]})
	}
	json.NewEncoder(writer).Encode(page)
}

func (kept *keptObjects) journalEntries() int {
	kept.mutex.Lock()
	defer kept.mutex.Unlock()
	count := 0
	for path := range kept.objects {
		if strings.Contains(path, "/"+journalFolder+"/") {
			count++
		}
	}
	return count
}

func aFollower(t *testing.T, kept *keptObjects) Follower {
	t.Helper()
	directory := t.TempDir()
	identity, errorValue := box.LoadOrCreateIdentity(filepath.Join(directory, "identity.json"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	keyring, errorValue := box.KeyringFor(identity, directory, companyID, identity.EncryptionPublicKey(), nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	sealingKey, _ := keyring.SealingKey()
	rootPath := filepath.Join(directory, "state")
	if errorValue := os.MkdirAll(rootPath, 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	return Follower{
		Roots:                []hostbackup.FileRoot{{Role: "state", Path: rootPath}},
		Journal:              Journal{Destination: offsite.Destination{Store: kept.serve(t), CompanyID: companyID, SealingKey: sealingKey}},
		Keyring:              keyring,
		ScratchDirectoryPath: directory,
	}
}

func keepOneChange(t *testing.T, state *following, number int) {
	t.Helper()
	root := state.Roots[0]
	if errorValue := os.WriteFile(filepath.Join(root.Path, "note"), []byte(fmt.Sprintf("change %d", number)), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	gathered := newGathering()
	change := Change{Root: root, Relative: "note"}
	gathered.changes[change.archivedName()] = change
	if errorValue := state.keepWhatChanged(context.Background(), gathered); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func keepTheNoteAsSeen(t *testing.T, state *following) {
	t.Helper()
	gathered := newGathering()
	change := Change{Root: state.Roots[0], Relative: "note"}
	gathered.changes[change.archivedName()] = change
	if errorValue := state.keepWhatChanged(context.Background(), gathered); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestAFileSeenAgainUnchangedIsNotKeptAgain(t *testing.T) {
	kept := &keptObjects{objects: map[string][]byte{}}
	state, errorValue := aFollower(t, kept).start(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	notePath := filepath.Join(state.Roots[0].Path, "note")
	if errorValue := os.WriteFile(notePath, []byte("as written"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	time.Sleep(settledAfter + 100*time.Millisecond)
	keepTheNoteAsSeen(t, state)
	afterTheFirst := kept.journalEntries()

	keepTheNoteAsSeen(t, state)
	if entries := kept.journalEntries(); entries != afterTheFirst {
		t.Fatalf("the journal went from %d to %d entries for a file nobody changed", afterTheFirst, entries)
	}

	if errorValue := os.WriteFile(notePath, []byte("written again"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	keepTheNoteAsSeen(t, state)
	if entries := kept.journalEntries(); entries != afterTheFirst+1 {
		t.Fatalf("the journal holds %d entries after the file changed, want %d", entries, afterTheFirst+1)
	}
}

func TestTheJournalIsCutBackOnceItHoldsAsManyEntriesAsAStartMayRead(t *testing.T) {
	kept := &keptObjects{objects: map[string][]byte{}}
	state, errorValue := aFollower(t, kept).start(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for number := range fullEntryAfter {
		keepOneChange(t, state, number)
	}
	if entries := kept.journalEntries(); entries != fullEntryAfter+1 {
		t.Fatalf("the journal holds %d entries after %d changes, want the full entry and every change", entries, fullEntryAfter)
	}
	keepOneChange(t, state, fullEntryAfter)
	if entries := kept.journalEntries(); entries != 1 {
		t.Fatalf("the journal holds %d entries after one change past its limit, want only the new full entry", entries)
	}
	newest, errorValue := state.Journal.Read(context.Background(), state.next-1, state.Keyring)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !newest.IsFull {
		t.Fatal("the entry kept past the limit is not a full one, so nothing behind it could be removed")
	}
}

func TestAStartOnAJournalPastItsLimitKeepsAFullEntryWithoutReadingTheOldOnes(t *testing.T) {
	kept := &keptObjects{objects: map[string][]byte{}}
	follower := aFollower(t, kept)
	for sequence := int64(1); sequence <= fullEntryAfter+2; sequence++ {
		if errorValue := follower.Journal.Keep(context.Background(), Entry{Version: entryVersion, Sequence: sequence, IsFull: sequence == 1}); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	state, errorValue := follower.start(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if kept.entriesRead != 0 {
		t.Fatalf("the start read %d journal entries, want none: a journal past its limit is replaced, not walked", kept.entriesRead)
	}
	if entries := kept.journalEntries(); entries != 1 {
		t.Fatalf("the journal holds %d entries after the start, want only the new full entry", entries)
	}
	if state.next != fullEntryAfter+4 {
		t.Fatalf("the next entry is numbered %d, want %d: numbers never go back", state.next, fullEntryAfter+4)
	}
}
