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
	heldChunk   *heldChunk
}

type heldChunk struct {
	isHeld   chan struct{}
	released chan struct{}
}

func (kept *keptObjects) holdTheNextChunk(t *testing.T) *heldChunk {
	t.Helper()
	held := &heldChunk{isHeld: make(chan struct{}), released: make(chan struct{})}
	kept.mutex.Lock()
	kept.heldChunk = held
	kept.mutex.Unlock()
	t.Cleanup(held.release)
	return held
}

func (held *heldChunk) release() {
	select {
	case <-held.released:
	default:
		close(held.released)
	}
}

func (kept *keptObjects) waitIfThisChunkIsHeld(request *http.Request) {
	if request.Method != http.MethodPost || !strings.Contains(request.URL.EscapedPath(), "/"+fileChunkFolder+"/") {
		return
	}
	kept.mutex.Lock()
	held := kept.heldChunk
	kept.heldChunk = nil
	kept.mutex.Unlock()
	if held == nil {
		return
	}
	close(held.isHeld)
	<-held.released
}

func (kept *keptObjects) serve(t *testing.T) offsite.Store {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		kept.waitIfThisChunkIsHeld(request)
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
	state := startedWithItsFullEntryKept(t, aFollower(t, kept))
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
	state := startedWithItsFullEntryKept(t, aFollower(t, kept))
	for number := range fullEntryAfter {
		keepOneChange(t, state, number)
	}
	if entries := kept.journalEntries(); entries != fullEntryAfter+1 {
		t.Fatalf("the journal holds %d entries after %d changes, want the full entry and every change", entries, fullEntryAfter)
	}
	keepOneChange(t, state, fullEntryAfter)
	landTheFullEntry(t, state)
	if entries := kept.journalEntries(); entries != 2 {
		t.Fatalf("the journal holds %d entries after one change past its limit, want the new full entry and the change kept beside it", entries)
	}
	full, errorValue := state.Journal.Read(context.Background(), state.next-2, state.Keyring)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !full.IsFull {
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
	state := startedWithItsFullEntryKept(t, follower)
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

const promptly = 5 * time.Second

func landTheFullEntry(t *testing.T, state *following) {
	t.Helper()
	select {
	case kept := <-state.fullEntryKept():
		if errorValue := state.landFullEntry(context.Background(), kept); errorValue != nil {
			t.Fatal(errorValue)
		}
	case <-time.After(promptly):
		t.Fatal("no full entry was kept")
	}
}

func startedWithItsFullEntryKept(t *testing.T, follower Follower) *following {
	t.Helper()
	state, errorValue := follower.start(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	landTheFullEntry(t, state)
	return state
}

func theMarkerAWriterWaitsOn(t *testing.T, barrier *Barrier) string {
	t.Helper()
	deadline := time.Now().Add(promptly)
	for time.Now().Before(deadline) {
		markers, errorValue := os.ReadDir(barrier.directoryPath())
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if len(markers) == 1 {
			return markers[0].Name()
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the writer left no marker to wait on")
	return ""
}

func TestAStartDoesNotWaitForTheWholeTreeToBeKept(t *testing.T) {
	kept := &keptObjects{objects: map[string][]byte{}}
	follower := aFollower(t, kept)
	if errorValue := os.WriteFile(filepath.Join(follower.Roots[0].Path, "kept before the journal began"), []byte("what the company already held"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	held := kept.holdTheNextChunk(t)
	started := make(chan error, 1)
	go func() {
		_, errorValue := follower.start(context.Background())
		started <- errorValue
	}()
	select {
	case <-held.isHeld:
	case <-time.After(promptly):
		t.Fatal("the start kept nothing of the tree the company already held")
	}
	select {
	case errorValue := <-started:
		if errorValue != nil {
			t.Fatal(errorValue)
		}
	case <-time.After(promptly):
		t.Fatal("the start is still waiting for the whole tree to be kept, so no writer is answered until it is")
	}
}

func TestAWriterIsAnsweredWhileTheWholeTreeIsStillBeingKept(t *testing.T) {
	kept := &keptObjects{objects: map[string][]byte{}}
	follower := aFollower(t, kept)
	root := follower.Roots[0]
	follower.Barrier = NewBarrier(root.Path, root.Role)
	if errorValue := follower.Barrier.Prepare(); errorValue != nil {
		t.Fatal(errorValue)
	}
	held := kept.holdTheNextChunk(t)
	state, errorValue := follower.start(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	<-held.isHeld
	if errorValue := os.WriteFile(filepath.Join(root.Path, "note"), []byte("written while the whole tree was being kept"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	answered := make(chan error, 1)
	go func() {
		waiting, cancel := context.WithTimeout(context.Background(), promptly)
		defer cancel()
		answered <- follower.Barrier.Wait(waiting)
	}()
	gathered := newGathering()
	change := Change{Root: root, Relative: "note"}
	gathered.changes[change.archivedName()] = change
	gathered.markers = []string{theMarkerAWriterWaitsOn(t, follower.Barrier)}

	if errorValue := state.keepWhatChanged(context.Background(), gathered); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := <-answered; errorValue != nil {
		t.Fatalf("the writer was not answered while the whole tree was still being kept: %v", errorValue)
	}
	if entries := kept.journalEntries(); entries != 1 {
		t.Fatalf("the journal holds %d entries while the whole tree is still being kept, want the one change", entries)
	}
	held.release()
	landTheFullEntry(t, state)
}

func TestAWriterAskingAfterChangesWereDroppedIsAnsweredOnceTheWholeTreeIsKept(t *testing.T) {
	kept := &keptObjects{objects: map[string][]byte{}}
	follower := aFollower(t, kept)
	root := follower.Roots[0]
	follower.Barrier = NewBarrier(root.Path, root.Role)
	if errorValue := follower.Barrier.Prepare(); errorValue != nil {
		t.Fatal(errorValue)
	}
	state := startedWithItsFullEntryKept(t, follower)
	if errorValue := os.WriteFile(filepath.Join(root.Path, "note"), []byte("written in a change the kernel dropped"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	held := kept.holdTheNextChunk(t)
	dropped := newGathering()
	dropped.isOverflowed = true
	if errorValue := state.keepWhatChanged(context.Background(), dropped); errorValue != nil {
		t.Fatal(errorValue)
	}
	<-held.isHeld
	answered := make(chan error, 1)
	go func() {
		waiting, cancel := context.WithTimeout(context.Background(), promptly)
		defer cancel()
		answered <- follower.Barrier.Wait(waiting)
	}()
	asked := newGathering()
	asked.markers = []string{theMarkerAWriterWaitsOn(t, follower.Barrier)}

	if errorValue := state.keepWhatChanged(context.Background(), asked); errorValue != nil {
		t.Fatal(errorValue)
	}

	select {
	case <-answered:
		t.Fatal("the writer was answered while the dropped change was still on the box alone")
	case <-time.After(200 * time.Millisecond):
	}
	held.release()
	landTheFullEntry(t, state)
	if errorValue := <-answered; errorValue != nil {
		t.Fatalf("the writer was not answered once the whole tree was kept: %v", errorValue)
	}
}

func TestAChangeKeptWhileTheWholeTreeWasBeingKeptIsReplayedAfterIt(t *testing.T) {
	kept := &keptObjects{objects: map[string][]byte{}}
	follower := aFollower(t, kept)
	state := startedWithItsFullEntryKept(t, follower)
	keepOneChange(t, state, 1)
	held := kept.holdTheNextChunk(t)
	if errorValue := state.startFullEntry(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}
	<-held.isHeld
	keepOneChange(t, state, 2)

	whileItWasKept := replayedEntries(t, follower)
	held.release()
	landTheFullEntry(t, state)
	onceItWasKept := replayedEntries(t, follower)

	if got := areFull(whileItWasKept); !slices.Equal(got, []bool{true, false, false}) {
		t.Fatalf("a replay while the whole tree was being kept applied full entries as %v, want the earlier full entry and both changes", got)
	}
	if got := areFull(onceItWasKept); !slices.Equal(got, []bool{true, false}) {
		t.Fatalf("a replay once the whole tree was kept applied full entries as %v, want the new full entry and the change kept beside it", got)
	}
	for _, replayed := range [][]replayedEntry{whileItWasKept, onceItWasKept} {
		if last := replayed[len(replayed)-1]; !strings.Contains(last.contents, "change 2") {
			t.Fatalf("the last entry replayed is %d, which does not hold the newest change", last.sequence)
		}
	}
	if entries := kept.journalEntries(); entries != 2 {
		t.Fatalf("the journal holds %d entries once the whole tree was kept, want the full entry and the change kept beside it", entries)
	}
}

type replayedEntry struct {
	sequence int64
	isFull   bool
	contents string
}

func replayedEntries(t *testing.T, follower Follower) []replayedEntry {
	t.Helper()
	cache := offsite.ChunkCache{DirectoryPath: filepath.Join(t.TempDir(), "file-chunks")}
	replayed := []replayedEntry{}
	_, errorValue := follower.Journal.Replay(context.Background(), follower.Keyring, cache, func(entry Entry, contents io.Reader) error {
		read, errorValue := io.ReadAll(contents)
		replayed = append(replayed, replayedEntry{sequence: entry.Sequence, isFull: entry.IsFull, contents: string(read)})
		return errorValue
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return replayed
}

func areFull(replayed []replayedEntry) []bool {
	full := []bool{}
	for _, entry := range replayed {
		full = append(full, entry.isFull)
	}
	return full
}
