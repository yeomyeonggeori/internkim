package walstream

import (
	"slices"
	"testing"
	"time"
)

var cadenceNow = time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC)

func TestABaseBackupIsDueWhenTheNewestIsADayOld(t *testing.T) {
	facts := baseBackupFacts{hasNewest: true, namesItsMajor: true, newestTakenAt: cadenceNow.Add(-baseBackupEvery), piecesSince: 10}

	if !baseBackupIsDue(facts, cadenceNow) {
		t.Fatal("a base backup a day old was kept as the newest")
	}
}

func TestABaseBackupIsDueWhenATakeOverWouldFetchTooManyPieces(t *testing.T) {
	facts := baseBackupFacts{hasNewest: true, namesItsMajor: true, newestTakenAt: cadenceNow.Add(-time.Hour), piecesSince: piecesATakeOverFetchesAtMost}

	if !baseBackupIsDue(facts, cadenceNow) {
		t.Fatalf("a take-over would fetch %d pieces and no base backup is due", piecesATakeOverFetchesAtMost)
	}
}

func TestNoBaseBackupIsDueWhileTheNewestIsRecentAndFewPiecesFollowIt(t *testing.T) {
	facts := baseBackupFacts{hasNewest: true, namesItsMajor: true, newestTakenAt: cadenceNow.Add(-time.Hour), piecesSince: piecesATakeOverFetchesAtMost - 1}

	if baseBackupIsDue(facts, cadenceNow) {
		t.Fatal("a base backup is due while the newest is an hour old and few pieces follow it")
	}
}

func TestABaseBackupIsDueWhenTheNewestDoesNotNameThePostgreSQLMajorThatWroteIt(t *testing.T) {
	facts := baseBackupFacts{hasNewest: true, newestTakenAt: cadenceNow.Add(-time.Hour), piecesSince: 1}

	if !baseBackupIsDue(facts, cadenceNow) {
		t.Fatal("the newest base backup names no PostgreSQL major, so no take-over can start from it, and no newer one is due")
	}
}

func TestABaseBackupIsDueWhenItIsBothOldAndFollowedByManyPieces(t *testing.T) {
	facts := baseBackupFacts{hasNewest: true, namesItsMajor: true, newestTakenAt: cadenceNow.Add(-2 * baseBackupEvery), piecesSince: 2 * piecesATakeOverFetchesAtMost}

	if !baseBackupIsDue(facts, cadenceNow) {
		t.Fatal("an old base backup followed by many pieces was kept as the newest")
	}
}

func TestTheFirstFailedBaseBackupIsTriedAgainWithinSeconds(t *testing.T) {
	attempts := failedAttempts{}.afterAnotherFailure(cadenceNow)

	if !attempts.isWaiting(cadenceNow.Add(time.Second)) {
		t.Fatal("a base backup that failed a second ago is tried again at once")
	}
	if attempts.isWaiting(cadenceNow.Add(firstRetryAfterAFailedBaseBackup)) {
		t.Fatalf("a base backup that failed once waits longer than %s, and every standby waits with it to be ready", firstRetryAfterAFailedBaseBackup)
	}
	if wait := attempts.nextCheckIn(cadenceNow.Add(4 * time.Second)); wait != firstRetryAfterAFailedBaseBackup-4*time.Second {
		t.Fatalf("the next check after a failure comes in %s", wait)
	}
}

func TestFailedBaseBackupsWaitLongerEachTimeUpToTheHourlyCheck(t *testing.T) {
	attempts := failedAttempts{}
	waits := []time.Duration{}
	for range 12 {
		attempts = attempts.afterAnotherFailure(cadenceNow)
		waits = append(waits, attempts.wait)
	}

	if waits[1] != 2*waits[0] {
		t.Fatalf("the second failure waits %s after a first of %s", waits[1], waits[0])
	}
	if waits[len(waits)-1] != upkeepCheckEvery {
		t.Fatalf("failures keep a base backup from being tried for %s, past the hourly check", waits[len(waits)-1])
	}
}

func TestAnUpkeepThatNeverFailedChecksHourly(t *testing.T) {
	attempts := failedAttempts{}

	if attempts.isWaiting(cadenceNow) {
		t.Fatal("an upkeep that never failed waits as if it had")
	}
	if wait := attempts.nextCheckIn(cadenceNow); wait != upkeepCheckEvery {
		t.Fatalf("an upkeep that never failed checks again in %s", wait)
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
