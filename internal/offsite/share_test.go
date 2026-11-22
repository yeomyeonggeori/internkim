package offsite

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
)

var sampleLogFolders = []string{"base/", "base-chunks/", "wal/"}

func TestStandbysSplitTheLogEvenlyAndAJoiningStandbyTakesOnlyWhatItWins(t *testing.T) {
	paths := make([]string, 4000)
	for index := range paths {
		paths[index] = fmt.Sprintf("wal/7/%08d", index)
	}
	two := []string{"standby-b", "standby-c"}
	owned := map[string]int{}
	for _, path := range paths {
		owned[ownerOf(two, path)]++
	}
	for _, standby := range two {
		if share := float64(owned[standby]) / float64(len(paths)); share < 0.45 || share > 0.55 {
			t.Fatalf("%s owns %.3f of the log beside one other standby", standby, share)
		}
	}

	three := append(slices.Clone(two), "standby-d")
	moved := 0
	for _, path := range paths {
		before, after := ownerOf(two, path), ownerOf(three, path)
		if before != after && after != "standby-d" {
			t.Fatalf("%s moved from %s to %s when standby-d joined", path, before, after)
		}
		if before != after {
			moved++
		}
	}
	if share := float64(moved) / float64(len(paths)); share < 0.28 || share > 0.38 {
		t.Fatalf("a third standby took %.3f of the log", share)
	}
}

func TestAStandbyHoldsItsShareOfTheLogAndReportsIt(t *testing.T) {
	storage, destination := aStoreHoldingALog(t, 60)
	storage.objects[companyID+"/snapshots/internkim-snapshot-20261122T000000Z.json"] = storedBody{bytes: []byte("a snapshot"), createdAt: storage.now}
	share := Share{DirectoryPath: filepath.Join(t.TempDir(), "standby-chunks")}
	placement := box.SharePlacement{StandbysDigest: "b,c", Standbys: []box.ShareHolder{{PublicKey: "standby-b"}, {PublicKey: "standby-c"}}}

	report, errorValue := share.KeepUp(context.Background(), destination, sampleLogFolders, "standby-b", placement)

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	held := heldPathsOf(t, share)
	expected := []string{}
	for _, path := range logPathsOf(storage) {
		if ownerOf([]string{"standby-b", "standby-c"}, path) == "standby-b" {
			expected = append(expected, path)
		}
	}
	if !slices.Equal(held, expected) {
		t.Fatalf("the standby holds %v, not its share %v", held, expected)
	}
	if report.RankedAgainst != "b,c" || report.SharePieces != len(expected) || report.HeldPieces != len(expected) || report.LogPieces != 60 {
		t.Fatalf("reported %+v for a share of %d", report, len(expected))
	}
	if report.KeptThrough == nil || !report.KeptThrough.Equal(storage.now) {
		t.Fatalf("reported keeping the log through %v, and its newest piece came at %v", report.KeptThrough, storage.now)
	}
	for _, path := range held {
		kept, _ := os.ReadFile(share.pathOf(path))
		if !bytes.Equal(kept, storage.objects[companyID+"/"+path].bytes) {
			t.Fatalf("%s is not what the store holds", path)
		}
	}
}

func TestAStandbyKeepsWhatLeftItsShareUntilTheStandbyItWentToReportsHoldingIt(t *testing.T) {
	storage, destination := aStoreHoldingALog(t, 40)
	share := Share{DirectoryPath: t.TempDir()}
	alone := box.SharePlacement{StandbysDigest: "b", Standbys: []box.ShareHolder{{PublicKey: "standby-b"}}}
	if _, errorValue := share.KeepUp(context.Background(), destination, sampleLogFolders, "standby-b", alone); errorValue != nil {
		t.Fatal(errorValue)
	}
	if held := heldPathsOf(t, share); len(held) != 40 {
		t.Fatalf("the only standby holds %d of 40 pieces", len(held))
	}

	joined := box.SharePlacement{StandbysDigest: "b,c", Standbys: []box.ShareHolder{{PublicKey: "standby-b"}, {PublicKey: "standby-c"}}}
	beforeTheReport, errorValue := share.KeepUp(context.Background(), destination, sampleLogFolders, "standby-b", joined)
	if errorValue != nil || beforeTheReport.HeldPieces != 40 || beforeTheReport.SharePieces >= 40 {
		t.Fatalf("before standby-c reported, standby-b reported %+v, %v", beforeTheReport, errorValue)
	}

	earlier := storage.now.Add(-time.Minute)
	joined.Standbys[1].KeptThrough = &earlier
	if report, _ := share.KeepUp(context.Background(), destination, sampleLogFolders, "standby-b", joined); report.HeldPieces != 40 {
		t.Fatalf("standby-b dropped pieces standby-c reports holding only up to before they came: %+v", report)
	}

	joined.Standbys[1].KeptThrough = &storage.now
	report, errorValue := share.KeepUp(context.Background(), destination, sampleLogFolders, "standby-b", joined)
	if errorValue != nil || report.HeldPieces != report.SharePieces {
		t.Fatalf("once standby-c reports holding its share, standby-b reported %+v, %v", report, errorValue)
	}
}

func TestABoxThatIsNoStandbyHandsOnWhatItHeldAndADroppedPieceGoesEverywhere(t *testing.T) {
	storage, destination := aStoreHoldingALog(t, 20)
	share := Share{DirectoryPath: t.TempDir()}
	alone := box.SharePlacement{StandbysDigest: "b", Standbys: []box.ShareHolder{{PublicKey: "standby-b"}}}
	share.KeepUp(context.Background(), destination, sampleLogFolders, "standby-b", alone)
	pruned := logPathsOf(storage)[0]
	delete(storage.objects, companyID+"/"+pruned)

	servedFrom := box.SharePlacement{StandbysDigest: "c", Standbys: []box.ShareHolder{{PublicKey: "standby-c", KeptThrough: &storage.now}}}
	report, errorValue := share.KeepUp(context.Background(), destination, sampleLogFolders, "standby-b", servedFrom)

	if errorValue != nil || report.SharePieces != 0 || report.HeldPieces != 0 || report.LogPieces != 19 {
		t.Fatalf("a box that serves reported %+v, %v", report, errorValue)
	}
}

func TestAStoreReadsThePieceThisBoxHoldsBeforeAskingTheStore(t *testing.T) {
	storage, destination := aStoreHoldingALog(t, 1)
	path := logPathsOf(storage)[0]
	share := Share{DirectoryPath: t.TempDir()}
	os.MkdirAll(filepath.Dir(share.pathOf(path)), 0o700)
	os.WriteFile(share.pathOf(path), []byte("held on this box"), 0o600)
	storage.token = "a session the store refuses"
	destination.Store.Held = share

	held, errorValue := destination.Store.Get(context.Background(), companyID+"/"+path)

	if errorValue != nil || string(held) != "held on this box" {
		t.Fatalf("read %q, %v", held, errorValue)
	}
	if _, errorValue := destination.Store.Get(context.Background(), companyID+"/wal/7/not-held"); errorValue == nil {
		t.Fatal("a piece this box does not hold was read without asking the store")
	}
}

func TestAPassThatCannotFetchItsShareReportsNothing(t *testing.T) {
	storage, destination := aStoreHoldingALog(t, 10)
	share := Share{DirectoryPath: t.TempDir()}
	placement := box.SharePlacement{StandbysDigest: "b", Standbys: []box.ShareHolder{{PublicKey: "standby-b"}}}
	storage.refusesReads = true

	if _, errorValue := share.KeepUp(context.Background(), destination, sampleLogFolders, "standby-b", placement); errorValue == nil {
		t.Fatal("a pass that fetched nothing reported its share whole")
	}
}

func aStoreHoldingALog(t *testing.T, pieces int) (*fakeStorage, Destination) {
	t.Helper()
	storage := newFakeStorage()
	keyring := aKeyring(t)
	destination := destinationFor(t, storage, keyring)
	folders := []string{"base", "base-chunks", "wal/7", "wal/8"}
	started := storage.now
	for index := range pieces {
		path := fmt.Sprintf("%s/%s/piece-%03d", companyID, folders[index%len(folders)], index)
		storage.now = started.Add(time.Duration(index) * time.Second)
		storage.objects[path] = storedBody{bytes: []byte("sealed " + path), createdAt: storage.now}
	}
	return storage, destination
}

func logPathsOf(storage *fakeStorage) []string {
	paths := []string{}
	for path := range storage.objects {
		relative, isUnderTheCompany := strings.CutPrefix(path, companyID+"/")
		isInTheLog := slices.ContainsFunc(sampleLogFolders, func(folder string) bool { return strings.HasPrefix(relative, folder) })
		if isUnderTheCompany && isInTheLog {
			paths = append(paths, relative)
		}
	}
	slices.Sort(paths)
	return paths
}

func heldPathsOf(t *testing.T, share Share) []string {
	t.Helper()
	held, errorValue := share.heldPieces()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	paths := []string{}
	for path := range held {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths
}
