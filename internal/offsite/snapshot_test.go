package offsite

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
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
)

const companyID = "00000000-0000-4000-8000-0000000000c1"

type storedBody struct {
	bytes     []byte
	createdAt time.Time
}

type fakeStorage struct {
	mutex   sync.Mutex
	objects map[string]storedBody
	token   string
	now     time.Time
	puts    int
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{objects: map[string]storedBody{}, token: "host.session.token", now: time.Now()}
}

func (storage *fakeStorage) serve(t *testing.T) Store {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		storage.mutex.Lock()
		defer storage.mutex.Unlock()
		if request.Header.Get("Authorization") != "Bearer "+storage.token {
			http.Error(writer, "not this company's computer", http.StatusForbidden)
			return
		}
		path := strings.TrimPrefix(request.URL.EscapedPath(), "/storage/v1")
		switch {
		case request.Method == http.MethodPost && strings.HasPrefix(path, "/object/list/"+Bucket):
			storage.list(writer, request)
		case request.Method == http.MethodPost && strings.HasPrefix(path, "/object/"+Bucket+"/"):
			body, _ := io.ReadAll(request.Body)
			storage.objects[strings.TrimPrefix(path, "/object/"+Bucket+"/")] = storedBody{bytes: body, createdAt: storage.now}
			storage.puts++
			writer.Write([]byte(`{}`))
		case request.Method == http.MethodGet && strings.HasPrefix(path, "/object/authenticated/"+Bucket+"/"):
			body, isKept := storage.objects[strings.TrimPrefix(path, "/object/authenticated/"+Bucket+"/")]
			if !isKept {
				http.Error(writer, "not found", http.StatusBadRequest)
				return
			}
			writer.Write(body.bytes)
		case request.Method == http.MethodDelete && path == "/object/"+Bucket:
			var body struct{ Prefixes []string }
			json.NewDecoder(request.Body).Decode(&body)
			for _, removed := range body.Prefixes {
				delete(storage.objects, removed)
			}
			writer.Write([]byte(`[]`))
		default:
			t.Errorf("unexpected %s %s", request.Method, path)
		}
	}))
	t.Cleanup(server.Close)
	return Store{ProjectURL: server.URL, PublishableKey: "publishable", AccessToken: func() (string, error) { return storage.token, nil }}
}

func (storage *fakeStorage) list(writer http.ResponseWriter, request *http.Request) {
	var asked struct {
		Prefix string `json:"prefix"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}
	json.NewDecoder(request.Body).Decode(&asked)
	names := []string{}
	created := map[string]time.Time{}
	for path, body := range storage.objects {
		if rest, isUnder := strings.CutPrefix(path, asked.Prefix); isUnder && !strings.Contains(rest, "/") {
			names = append(names, rest)
			created[rest] = body.createdAt
		}
	}
	slices.Sort(names)
	page := []StoredObject{}
	for index := asked.Offset; index < len(names) && index < asked.Offset+asked.Limit; index++ {
		page = append(page, StoredObject{Name: names[index], CreatedAt: created[names[index]]})
	}
	json.NewEncoder(writer).Encode(page)
}

func (storage *fakeStorage) chunkCount() int {
	count := 0
	for path := range storage.objects {
		if strings.Contains(path, "/"+chunkFolder+"/") {
			count++
		}
	}
	return count
}

func aKeyring(t *testing.T) box.Keyring {
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
	return keyring
}

func destinationFor(t *testing.T, storage *fakeStorage, keyring box.Keyring) Destination {
	t.Helper()
	sealingKey, _ := keyring.SealingKey()
	return Destination{Store: storage.serve(t), CompanyID: companyID, SealingKey: sealingKey}
}

func randomBytes(t *testing.T, length int) []byte {
	t.Helper()
	contents := make([]byte, length)
	if _, errorValue := rand.Read(contents); errorValue != nil {
		t.Fatal(errorValue)
	}
	return contents
}

type hostMembers map[string][]byte

func shipSnapshot(t *testing.T, destination Destination, keyring box.Keyring, members hostMembers, now time.Time) (Manifest, int) {
	t.Helper()
	snapshot, errorValue := StartSnapshot(context.Background(), destination, keyring)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	names := make([]string, 0, len(members))
	for name := range members {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if errorValue := snapshot.AddMember(name, func(output io.Writer) error {
			_, errorValue := output.Write(members[name])
			return errorValue
		}); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	manifest, errorValue := snapshot.Finish(hostbackup.Manifest{CreatedAt: now, CompanyID: companyID, PackageVersion: "1:0.0.1"}, now)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return manifest, snapshot.UploadedChunks()
}

func membersOfArchive(t *testing.T, archivePath string) hostMembers {
	t.Helper()
	archive, errorValue := hostbackup.Open(archivePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := archive.Verify(); errorValue != nil {
		t.Fatal(errorValue)
	}
	read := hostMembers{}
	for _, member := range archive.Manifest.Members {
		if errorValue := archive.ReadMember(member.Name, func(reader io.Reader) error {
			contents, errorValue := io.ReadAll(reader)
			read[member.Name] = contents
			return errorValue
		}); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	return read
}

func TestAStandbyBoxRebuildsTheArchiveTheServiceBoxSnapshotted(t *testing.T) {
	storage := newFakeStorage()
	keyring := aKeyring(t)
	destination := destinationFor(t, storage, keyring)
	members := hostMembers{
		hostbackup.DatabaseMemberName("blueclaw"): randomBytes(t, 5*typicalChunkBytes),
		hostbackup.RosterMemberName:               []byte(`{"people":[]}`),
	}
	shipped, _ := shipSnapshot(t, destination, keyring, members, time.Now())
	for path, kept := range storage.objects {
		if bytes.Contains(kept.bytes, members[hostbackup.DatabaseMemberName("blueclaw")][:64]) {
			t.Fatalf("%s holds the company's data readable", path)
		}
	}

	cache := ChunkCache{DirectoryPath: t.TempDir()}
	newest, fetched, errorValue := cache.KeepUp(context.Background(), destination)
	if errorValue != nil || newest.Name != shipped.Name || fetched != len(shipped.chunkIDs()) {
		t.Fatalf("kept up with %s fetching %d, %v", newest.Name, fetched, errorValue)
	}
	archivePath := filepath.Join(t.TempDir(), "taken-over.tar")
	if errorValue := cache.Materialize(context.Background(), destination, newest, keyring, archivePath); errorValue != nil {
		t.Fatal(errorValue)
	}
	rebuilt := membersOfArchive(t, archivePath)
	for name, contents := range members {
		if !bytes.Equal(rebuilt[name], contents) {
			t.Fatalf("%s differs after the take-over", name)
		}
	}
}

func TestASnapshotUploadsOnlyWhatChangedSinceTheLastOne(t *testing.T) {
	storage := newFakeStorage()
	keyring := aKeyring(t)
	destination := destinationFor(t, storage, keyring)
	database := randomBytes(t, 8*typicalChunkBytes)
	_, first := shipSnapshot(t, destination, keyring, hostMembers{hostbackup.DatabaseMemberName("buzz"): database}, time.Now())

	edited := append(append([]byte{}, database[:3*typicalChunkBytes]...), append([]byte("a new message"), database[3*typicalChunkBytes:]...)...)
	_, second := shipSnapshot(t, destination, keyring, hostMembers{hostbackup.DatabaseMemberName("buzz"): edited}, time.Now().Add(time.Minute))

	if first < 4 || second == 0 || second > 2 {
		t.Fatalf("the first snapshot uploaded %d chunks and a 13-byte insertion uploaded %d", first, second)
	}
}

func TestABoxWithoutTheCompanyKeyCannotRebuildASnapshotAndLeavesNoArchive(t *testing.T) {
	storage := newFakeStorage()
	keyring := aKeyring(t)
	destination := destinationFor(t, storage, keyring)
	shipped, _ := shipSnapshot(t, destination, keyring, hostMembers{hostbackup.RosterMemberName: []byte("{}")}, time.Now())
	stranger := aKeyring(t)

	archivePath := filepath.Join(t.TempDir(), "taken-over.tar")
	errorValue := ChunkCache{DirectoryPath: t.TempDir()}.Materialize(context.Background(), destination, shipped, stranger, archivePath)

	if !errors.Is(errorValue, box.ErrCompanyKeyNotHeld) {
		t.Fatalf("a box without the key rebuilt it: %v", errorValue)
	}
	if _, statError := os.Stat(archivePath); !os.IsNotExist(statError) {
		t.Fatal("a refused snapshot left an archive behind")
	}
	if _, errorValue := StartSnapshot(context.Background(), destination, box.Keyring{Identity: stranger.Identity}); !errors.Is(errorValue, box.ErrCompanyKeyNotHeld) {
		t.Fatalf("a box without the key started a snapshot: %v", errorValue)
	}
}

func TestAChunkAlteredInTheStoreIsRefused(t *testing.T) {
	storage := newFakeStorage()
	keyring := aKeyring(t)
	destination := destinationFor(t, storage, keyring)
	shipped, _ := shipSnapshot(t, destination, keyring, hostMembers{hostbackup.RosterMemberName: []byte(`{"people":["sample"]}`)}, time.Now())
	chunkPath := destination.chunkPath(shipped.chunkIDs()[0])
	altered := storage.objects[chunkPath]
	altered.bytes[len(altered.bytes)-1] ^= 1

	errorValue := ChunkCache{DirectoryPath: t.TempDir()}.Materialize(context.Background(), destination, shipped, keyring, filepath.Join(t.TempDir(), "taken-over.tar"))

	if errorValue == nil {
		t.Fatal("an altered chunk was rebuilt into the archive")
	}
}

func TestOnlyASnapshotSealedToTheCurrentKeyIsTheNewest(t *testing.T) {
	storage := newFakeStorage()
	current := aKeyring(t)
	retired := aKeyring(t)
	destination := destinationFor(t, storage, current)
	retiredDestination := destinationFor(t, storage, retired)
	older, _ := shipSnapshot(t, destination, current, hostMembers{hostbackup.RosterMemberName: []byte("{}")}, time.Now())
	shipSnapshot(t, retiredDestination, retired, hostMembers{hostbackup.RosterMemberName: []byte("{}")}, time.Now().Add(time.Minute))
	storage.objects[destination.snapshotPath(snapshotNameAt(time.Now().Add(2*time.Minute)))] = storedBody{bytes: []byte("not a manifest")}

	newest, errorValue := Newest(context.Background(), destination)

	if errorValue != nil || newest.Name != older.Name {
		t.Fatalf("newest %s, %v", newest.Name, errorValue)
	}
	other := Destination{Store: destination.Store, CompanyID: "00000000-0000-4000-8000-0000000000c2", SealingKey: destination.SealingKey}
	if _, errorValue := Newest(context.Background(), other); !errors.Is(errorValue, ErrNoBackup) {
		t.Fatalf("a company with no snapshots answered %v", errorValue)
	}
}

func TestPruningKeepsTheNewestSnapshotsAndSweepsOnlyChunksNothingKeptNeedsAndNothingNewIsWriting(t *testing.T) {
	storage := newFakeStorage()
	keyring := aKeyring(t)
	destination := destinationFor(t, storage, keyring)
	start := time.Now()
	storage.now = start.Add(-3 * time.Hour)
	shipSnapshot(t, destination, keyring, hostMembers{hostbackup.RosterMemberName: []byte("first")}, start.Add(-3*time.Hour))
	shipSnapshot(t, destination, keyring, hostMembers{hostbackup.RosterMemberName: []byte("second")}, start.Add(-2*time.Hour))
	storage.now = start
	kept, _ := shipSnapshot(t, destination, keyring, hostMembers{hostbackup.RosterMemberName: []byte("third")}, start)
	storage.objects[destination.chunkPath("being-written")] = storedBody{bytes: []byte("sealed"), createdAt: start}

	pruned, errorValue := Prune(context.Background(), destination, 1, start)

	if errorValue != nil || len(pruned) != 2 {
		t.Fatalf("pruned %v, %v", pruned, errorValue)
	}
	if storage.chunkCount() != len(kept.chunkIDs())+1 {
		t.Fatalf("%d chunks are left; the kept snapshot needs %d and one is still being written", storage.chunkCount(), len(kept.chunkIDs()))
	}
	newest, errorValue := Newest(context.Background(), destination)
	if errorValue != nil || newest.Name != kept.Name {
		t.Fatalf("newest after pruning %s, %v", newest.Name, errorValue)
	}
}

func TestAStandbyCacheForgetsWhatTheNewestSnapshotNoLongerNeeds(t *testing.T) {
	storage := newFakeStorage()
	keyring := aKeyring(t)
	destination := destinationFor(t, storage, keyring)
	cache := ChunkCache{DirectoryPath: t.TempDir()}
	shipSnapshot(t, destination, keyring, hostMembers{hostbackup.RosterMemberName: []byte("before")}, time.Now())
	if _, _, errorValue := cache.KeepUp(context.Background(), destination); errorValue != nil {
		t.Fatal(errorValue)
	}
	newest, _ := shipSnapshot(t, destination, keyring, hostMembers{hostbackup.RosterMemberName: []byte("after")}, time.Now().Add(time.Minute))

	if _, fetched, errorValue := cache.KeepUp(context.Background(), destination); errorValue != nil || fetched != 1 {
		t.Fatalf("fetched %d, %v", fetched, errorValue)
	}
	entries, _ := os.ReadDir(cache.DirectoryPath)
	if len(entries) != len(newest.chunkIDs()) {
		t.Fatalf("the cache holds %d chunks for a snapshot of %d", len(entries), len(newest.chunkIDs()))
	}
}
