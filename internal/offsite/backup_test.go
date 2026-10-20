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
)

const companyID = "00000000-0000-4000-8000-0000000000c1"

type fakeStorage struct {
	mutex   sync.Mutex
	objects map[string][]byte
	token   string
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
			storage.objects[strings.TrimPrefix(path, "/object/"+Bucket+"/")] = body
			writer.Write([]byte(`{}`))
		case request.Method == http.MethodGet && strings.HasPrefix(path, "/object/authenticated/"+Bucket+"/"):
			body, isKept := storage.objects[strings.TrimPrefix(path, "/object/authenticated/"+Bucket+"/")]
			if !isKept {
				http.Error(writer, "not found", http.StatusBadRequest)
				return
			}
			writer.Write(body)
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
	for path := range storage.objects {
		if rest, isUnder := strings.CutPrefix(path, asked.Prefix); isUnder {
			name, _, _ := strings.Cut(rest, "/")
			if !slices.Contains(names, name) {
				names = append(names, name)
			}
		}
	}
	slices.Sort(names)
	page := []StoredObject{}
	for index := asked.Offset; index < len(names) && index < asked.Offset+asked.Limit; index++ {
		page = append(page, StoredObject{Name: names[index]})
	}
	json.NewEncoder(writer).Encode(page)
}

func anIdentity(t *testing.T) box.Identity {
	t.Helper()
	identity, errorValue := box.LoadOrCreateIdentity(filepath.Join(t.TempDir(), "identity.json"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return identity
}

func anArchive(t *testing.T, name string, length int) (string, []byte) {
	t.Helper()
	contents := make([]byte, length)
	if _, errorValue := rand.Read(contents); errorValue != nil {
		t.Fatal(errorValue)
	}
	path := filepath.Join(t.TempDir(), name)
	if errorValue := os.WriteFile(path, contents, 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return path, contents
}

func TestABackupShippedByTheServiceBoxIsRestoredByAnotherBoxOfTheCompany(t *testing.T) {
	storage := &fakeStorage{objects: map[string][]byte{}, token: "host.session.token"}
	serving := anIdentity(t)
	destination := Destination{Store: storage.serve(t), CompanyID: companyID, SealingKey: serving.EncryptionPublicKey()}
	archivePath, contents := anArchive(t, "internkim-backup-20261019T031500Z.tar", partBytes+4096)

	shipped, errorValue := Ship(context.Background(), archivePath, destination, time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(shipped.Parts) != 2 {
		t.Fatalf("a backup larger than one part was kept in %d parts", len(shipped.Parts))
	}
	for path, kept := range storage.objects {
		if bytes.Contains(kept, contents[:64]) {
			t.Fatalf("%s holds the archive readable", path)
		}
	}

	newest, errorValue := Newest(context.Background(), destination)
	if errorValue != nil || newest.ArchiveName != shipped.ArchiveName {
		t.Fatalf("newest %+v, %v", newest, errorValue)
	}
	restoredPath := filepath.Join(t.TempDir(), newest.ArchiveName)
	if errorValue := Fetch(context.Background(), destination, newest, box.Keyring{Identity: serving}, restoredPath); errorValue != nil {
		t.Fatal(errorValue)
	}
	restored, errorValue := os.ReadFile(restoredPath)
	if errorValue != nil || !bytes.Equal(restored, contents) {
		t.Fatalf("the restored archive differs: %v", errorValue)
	}
}

func TestABoxWithoutTheCompanyKeyCannotOpenTheBackupAndLeavesNoArchive(t *testing.T) {
	storage := &fakeStorage{objects: map[string][]byte{}, token: "host.session.token"}
	serving := anIdentity(t)
	destination := Destination{Store: storage.serve(t), CompanyID: companyID, SealingKey: serving.EncryptionPublicKey()}
	archivePath, _ := anArchive(t, "internkim-backup-20261019T031500Z.tar", 1024)
	shipped, errorValue := Ship(context.Background(), archivePath, destination, time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	restoredPath := filepath.Join(t.TempDir(), shipped.ArchiveName)
	errorValue = Fetch(context.Background(), destination, shipped, box.Keyring{Identity: anIdentity(t)}, restoredPath)

	if !errors.Is(errorValue, box.ErrCompanyKeyNotHeld) {
		t.Fatalf("a box without the key fetched it: %v", errorValue)
	}
	if _, statError := os.Stat(restoredPath); !os.IsNotExist(statError) {
		t.Fatal("a refused backup left an archive behind")
	}
}

func TestAPartAlteredInTheStoreIsRefused(t *testing.T) {
	storage := &fakeStorage{objects: map[string][]byte{}, token: "host.session.token"}
	serving := anIdentity(t)
	destination := Destination{Store: storage.serve(t), CompanyID: companyID, SealingKey: serving.EncryptionPublicKey()}
	archivePath, _ := anArchive(t, "internkim-backup-20261019T031500Z.tar", 4096)
	shipped, errorValue := Ship(context.Background(), archivePath, destination, time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	partPath := companyID + "/" + shipped.ArchiveName + "/" + shipped.Parts[0].Name
	storage.objects[partPath][100] ^= 1

	if errorValue := Fetch(context.Background(), destination, shipped, box.Keyring{Identity: serving}, filepath.Join(t.TempDir(), "restored.tar")); errorValue == nil {
		t.Fatal("an altered part was restored")
	}
}

func TestOnlyACompleteBackupSealedToTheCurrentKeyIsTheNewest(t *testing.T) {
	storage := &fakeStorage{objects: map[string][]byte{}, token: "host.session.token"}
	serving := anIdentity(t)
	store := storage.serve(t)
	current := Destination{Store: store, CompanyID: companyID, SealingKey: serving.EncryptionPublicKey()}
	retired := Destination{Store: store, CompanyID: companyID, SealingKey: anIdentity(t).EncryptionPublicKey()}
	olderPath, _ := anArchive(t, "internkim-backup-20261017T031500Z.tar", 512)
	if _, errorValue := Ship(context.Background(), olderPath, current, time.Now()); errorValue != nil {
		t.Fatal(errorValue)
	}
	retiredPath, _ := anArchive(t, "internkim-backup-20261018T031500Z.tar", 512)
	if _, errorValue := Ship(context.Background(), retiredPath, retired, time.Now()); errorValue != nil {
		t.Fatal(errorValue)
	}
	storage.objects[companyID+"/internkim-backup-20261019T031500Z.tar/part-000001"] = []byte("an upload that never finished")

	newest, errorValue := Newest(context.Background(), current)

	if errorValue != nil || newest.ArchiveName != "internkim-backup-20261017T031500Z.tar" {
		t.Fatalf("newest %+v, %v", newest, errorValue)
	}
	if _, errorValue := Newest(context.Background(), Destination{Store: store, CompanyID: "00000000-0000-4000-8000-0000000000c2", SealingKey: serving.EncryptionPublicKey()}); !errors.Is(errorValue, ErrNoBackup) {
		t.Fatalf("a company with no backups answered %v", errorValue)
	}
}

func TestPruningKeepsTheNewestBackupsAndRemovesEveryPartOfTheRest(t *testing.T) {
	storage := &fakeStorage{objects: map[string][]byte{}, token: "host.session.token"}
	serving := anIdentity(t)
	destination := Destination{Store: storage.serve(t), CompanyID: companyID, SealingKey: serving.EncryptionPublicKey()}
	for _, day := range []string{"15", "16", "17", "18"} {
		archivePath, _ := anArchive(t, "internkim-backup-202610"+day+"T031500Z.tar", 256)
		if _, errorValue := Ship(context.Background(), archivePath, destination, time.Now()); errorValue != nil {
			t.Fatal(errorValue)
		}
	}

	pruned, errorValue := Prune(context.Background(), destination, 2)

	if errorValue != nil || !slices.Equal(pruned, []string{"internkim-backup-20261015T031500Z.tar", "internkim-backup-20261016T031500Z.tar"}) {
		t.Fatalf("pruned %v, %v", pruned, errorValue)
	}
	for path := range storage.objects {
		if strings.Contains(path, "20261015") || strings.Contains(path, "20261016") {
			t.Fatalf("%s outlived pruning", path)
		}
	}
	if kept, _ := archiveNamesOf(context.Background(), destination.Store, companyID); len(kept) != 2 {
		t.Fatalf("kept %v", kept)
	}
}
