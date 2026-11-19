package walstream

import (
	"slices"
	"testing"
	"time"
)

var cadenceNow = time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC)

func TestABaseBackupIsDueWhenTheNewestIsADayOld(t *testing.T) {
	facts := baseBackupFacts{hasNewest: true, newestTakenAt: cadenceNow.Add(-baseBackupEvery), piecesSince: 10}

	if !baseBackupIsDue(facts, cadenceNow) {
		t.Fatal("a base backup a day old was kept as the newest")
	}
}

func TestABaseBackupIsDueWhenATakeOverWouldFetchTooManyPieces(t *testing.T) {
	facts := baseBackupFacts{hasNewest: true, newestTakenAt: cadenceNow.Add(-time.Hour), piecesSince: piecesATakeOverFetchesAtMost}

	if !baseBackupIsDue(facts, cadenceNow) {
		t.Fatalf("a take-over would fetch %d pieces and no base backup is due", piecesATakeOverFetchesAtMost)
	}
}

func TestNoBaseBackupIsDueWhileTheNewestIsRecentAndFewPiecesFollowIt(t *testing.T) {
	facts := baseBackupFacts{hasNewest: true, newestTakenAt: cadenceNow.Add(-time.Hour), piecesSince: piecesATakeOverFetchesAtMost - 1}

	if baseBackupIsDue(facts, cadenceNow) {
		t.Fatal("a base backup is due while the newest is an hour old and few pieces follow it")
	}
}

func TestABaseBackupIsDueWhenItIsBothOldAndFollowedByManyPieces(t *testing.T) {
	facts := baseBackupFacts{hasNewest: true, newestTakenAt: cadenceNow.Add(-2 * baseBackupEvery), piecesSince: 2 * piecesATakeOverFetchesAtMost}

	if !baseBackupIsDue(facts, cadenceNow) {
		t.Fatal("an old base backup followed by many pieces was kept as the newest")
	}
}

func TestAFailedAttemptIsNotTriedAgainUntilTheNextHourlyCheck(t *testing.T) {
	failedAt := cadenceNow.Add(-time.Minute)

	if !aFailedAttemptIsRecent(failedAt, cadenceNow) {
		t.Fatal("a base backup that failed a minute ago would be tried again at once")
	}
	if aFailedAttemptIsRecent(failedAt, failedAt.Add(upkeepCheckEvery)) {
		t.Fatal("a base backup that failed an hour ago is never tried again")
	}
	if aFailedAttemptIsRecent(time.Time{}, cadenceNow) {
		t.Fatal("an upkeep that never failed waits as if it had")
	}
}

func TestTheCounterSignalsOnceTheBoundIsReached(t *testing.T) {
	pieces := NewPiecesSinceBaseBackup()
	pieces.start(piecesATakeOverFetchesAtMost - 2)

	pieces.add()
	select {
	case <-pieces.reached:
		t.Fatal("the counter signalled before the bound")
	default:
	}
	pieces.add()

	select {
	case <-pieces.reached:
	default:
		t.Fatalf("the counter reached %d pieces without signalling", piecesATakeOverFetchesAtMost)
	}
}

func TestOnlyPiecesFromTheBaseBackupsStartAreCounted(t *testing.T) {
	names := []PieceName{{Timeline: 1, Start: 0x100, End: 0x200}, {Timeline: 1, Start: 0x200, End: 0x300}, {Timeline: 1, Start: 0x300, End: 0x400}}

	if count := piecesFrom(names, 0x200); count != 2 {
		t.Fatalf("counted %d pieces from 0x200, want 2", count)
	}
}

func baseBackupTakenAgo(name string, ago time.Duration) BaseBackup {
	return BaseBackup{Name: name, CreatedAt: cadenceNow.Add(-ago)}
}

func TestPruningKeepsTheNewestTwoAndAnyATakeOverMayStillBeUsing(t *testing.T) {
	backups := []BaseBackup{
		baseBackupTakenAgo("oldest", 5*time.Hour),
		baseBackupTakenAgo("an-hour-and-more", 2*time.Hour),
		baseBackupTakenAgo("recent", 50*time.Minute),
		baseBackupTakenAgo("newer", 20*time.Minute),
		baseBackupTakenAgo("newest", 5*time.Minute),
	}

	kept, stale := baseBackupsToKeep(backups, cadenceNow)

	keptNames, staleNames := []string{}, []string{}
	for _, backup := range kept {
		keptNames = append(keptNames, backup.Name)
	}
	for _, backup := range stale {
		staleNames = append(staleNames, backup.Name)
	}
	if !slices.Equal(keptNames, []string{"recent", "newer", "newest"}) || !slices.Equal(staleNames, []string{"oldest", "an-hour-and-more"}) {
		t.Fatalf("kept %v and pruned %v", keptNames, staleNames)
	}
}

func TestPruningKeepsTheNewestTwoEvenWhenTheyAreOld(t *testing.T) {
	backups := []BaseBackup{
		baseBackupTakenAgo("three-days", 72*time.Hour),
		baseBackupTakenAgo("two-days", 48*time.Hour),
		baseBackupTakenAgo("one-day", 24*time.Hour),
	}

	kept, stale := baseBackupsToKeep(backups, cadenceNow)

	if len(kept) != 2 || kept[0].Name != "two-days" || len(stale) != 1 || stale[0].Name != "three-days" {
		t.Fatalf("kept %v and pruned %v", kept, stale)
	}
}
