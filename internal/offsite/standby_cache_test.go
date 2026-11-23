package offsite

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/hostbackup"
)

type keptUpCache struct {
	storage     *fakeStorage
	keyring     box.Keyring
	destination Destination
	cache       ChunkCache
	shipped     Manifest
	members     hostMembers
}

func aCacheKeptUpWithASnapshot(t *testing.T) keptUpCache {
	t.Helper()
	storage := newFakeStorage()
	keyring := aKeyring(t)
	destination := destinationFor(t, storage, keyring)
	members := hostMembers{
		hostbackup.DatabaseMemberName("blueclaw"): randomBytes(t, 3*typicalChunkBytes),
		hostbackup.RosterMemberName:               []byte(`{"people":[]}`),
	}
	shipped, _ := shipSnapshot(t, destination, keyring, members, time.Now())
	cache := ChunkCache{DirectoryPath: t.TempDir()}
	if _, _, errorValue := cache.KeepUp(context.Background(), destination); errorValue != nil {
		t.Fatal(errorValue)
	}
	return keptUpCache{storage: storage, keyring: keyring, destination: destination, cache: cache, shipped: shipped, members: members}
}

func (kept keptUpCache) databaseChunk() string {
	return kept.shipped.Chunks[hostbackup.DatabaseMemberName("blueclaw")][0]
}

func (kept keptUpCache) materializes(t *testing.T) {
	t.Helper()
	archivePath := filepath.Join(t.TempDir(), "taken-over.tar")
	if errorValue := kept.cache.Materialize(context.Background(), kept.destination, kept.shipped, kept.keyring, archivePath); errorValue != nil {
		t.Fatal(errorValue)
	}
	rebuilt := membersOfArchive(t, archivePath)
	for name, contents := range kept.members {
		if !bytes.Equal(rebuilt[name], contents) {
			t.Fatalf("%s differs after the take-over", name)
		}
	}
}

func replaceCachedChunk(t *testing.T, kept keptUpCache, chunkID string, contents []byte) {
	t.Helper()
	if errorValue := os.WriteFile(kept.cache.pathOf(chunkID), contents, 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestAZeroByteCachedChunkIsFetchedAgainOnce(t *testing.T) {
	kept := aCacheKeptUpWithASnapshot(t)
	chunkID := kept.databaseChunk()
	replaceCachedChunk(t, kept, chunkID, nil)

	kept.materializes(t)

	if fetched := kept.storage.gets[kept.destination.chunkPath(chunkID)]; fetched != 2 {
		t.Fatalf("the chunk was fetched %d times; once to keep up and once more after it read empty", fetched)
	}
}

func TestACachedChunkCutBeforeItsHeaderEndsIsFetchedAgain(t *testing.T) {
	kept := aCacheKeptUpWithASnapshot(t)
	chunkID := kept.databaseChunk()
	sealed, _ := os.ReadFile(kept.cache.pathOf(chunkID))
	replaceCachedChunk(t, kept, chunkID, sealed[:bytes.IndexByte(sealed, '\n')])

	kept.materializes(t)

	if fetched := kept.storage.gets[kept.destination.chunkPath(chunkID)]; fetched != 2 {
		t.Fatalf("the chunk was fetched %d times", fetched)
	}
}

func TestACachedChunkHoldingOtherContentIsFetchedAgain(t *testing.T) {
	kept := aCacheKeptUpWithASnapshot(t)
	chunkID := kept.databaseChunk()
	other, _ := os.ReadFile(kept.cache.pathOf(kept.shipped.Chunks[hostbackup.DatabaseMemberName("blueclaw")][1]))
	replaceCachedChunk(t, kept, chunkID, other)

	kept.materializes(t)

	if fetched := kept.storage.gets[kept.destination.chunkPath(chunkID)]; fetched != 2 {
		t.Fatalf("the chunk was fetched %d times", fetched)
	}
}

func TestAChunkBadOnThePlaneTooFailsNamingItAfterOneFetch(t *testing.T) {
	storage := newFakeStorage()
	keyring := aKeyring(t)
	destination := destinationFor(t, storage, keyring)
	shipped, _ := shipSnapshot(t, destination, keyring, hostMembers{hostbackup.RosterMemberName: []byte(`{"people":["sample"]}`)}, time.Now())
	chunkID := shipped.chunkIDs()[0]
	altered := storage.objects[destination.chunkPath(chunkID)]
	altered.bytes[len(altered.bytes)-1] ^= 1

	errorValue := ChunkCache{DirectoryPath: t.TempDir()}.Materialize(context.Background(), destination, shipped, keyring, filepath.Join(t.TempDir(), "taken-over.tar"))

	if errorValue == nil || !strings.Contains(errorValue.Error(), chunkID) || !strings.Contains(errorValue.Error(), "the plane's copy is bad") {
		t.Fatalf("an altered chunk on the plane answered %v", errorValue)
	}
	if fetched := storage.gets[destination.chunkPath(chunkID)]; fetched != 1 {
		t.Fatalf("the chunk was fetched %d times; a bad copy on the plane is fetched once", fetched)
	}
}

func TestAFetchedChunkIsWholeAndLeavesNothingBeside(t *testing.T) {
	kept := aCacheKeptUpWithASnapshot(t)

	entries, errorValue := os.ReadDir(kept.cache.DirectoryPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(entries) != len(kept.shipped.chunkIDs()) {
		t.Fatalf("the cache holds %d entries for %d chunks", len(entries), len(kept.shipped.chunkIDs()))
	}
	for _, chunkID := range kept.shipped.chunkIDs() {
		cached, errorValue := os.ReadFile(kept.cache.pathOf(chunkID))
		if errorValue != nil || !bytes.Equal(cached, kept.storage.objects[kept.destination.chunkPath(chunkID)].bytes) {
			t.Fatalf("the cached chunk %s is not the plane's copy: %v", chunkID, errorValue)
		}
	}
}
