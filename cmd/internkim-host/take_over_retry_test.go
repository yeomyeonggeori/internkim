package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/companyhost"
)

type retryClock struct{ now time.Time }

func (clock *retryClock) read() time.Time { return clock.now }

func aTakeOverRetry(t *testing.T) (takeOverRetry, *retryClock) {
	t.Helper()
	clock := &retryClock{now: time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)}
	retry := takeOverRetry{
		StatePath: filepath.Join(t.TempDir(), box.TakeOverRetryFileName),
		Now:       clock.read,
	}
	return retry, clock
}

func retriedOf(t *testing.T, retry takeOverRetry) int {
	t.Helper()
	attempts, errorValue := retry.read()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return attempts.Retried
}

func TestAFailedTakeOverIsTriedAgainOnceTheFirstWaitHasPassed(t *testing.T) {
	retry, clock := aTakeOverRetry(t)

	if errorValue := retry.allowARetryWhenItIsTime(); !errors.Is(errorValue, errTakeOverWaitsToRetry) {
		t.Fatalf("a take-over that has just failed was tried again at once: %v", errorValue)
	}
	clock.now = clock.now.Add(firstTakeOverRetryWait - time.Second)
	if errorValue := retry.allowARetryWhenItIsTime(); !errors.Is(errorValue, errTakeOverWaitsToRetry) {
		t.Fatalf("the failed take-over was tried again before its wait was over: %v", errorValue)
	}
	clock.now = clock.now.Add(time.Second)

	if errorValue := retry.allowARetryWhenItIsTime(); errorValue != nil {
		t.Fatal(errorValue)
	}
	if retried := retriedOf(t, retry); retried != 1 {
		t.Fatalf("the failed take-over was tried again %d times", retried)
	}
}

func TestEachFailureDoublesTheWaitUntilItReachesTheLongest(t *testing.T) {
	retry, clock := aTakeOverRetry(t)
	waits := []time.Duration{}
	for range 7 {
		retry.allowARetryWhenItIsTime()
		seenAt := clock.now
		for retry.allowARetryWhenItIsTime() != nil {
			clock.now = clock.now.Add(time.Minute)
		}
		waits = append(waits, clock.now.Sub(seenAt))
	}

	want := []time.Duration{time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute, 16 * time.Minute, longestTakeOverRetryWait, longestTakeOverRetryWait}
	for index := range want {
		if waits[index] != want[index] {
			t.Fatalf("waits were %v, want %v", waits, want)
		}
	}
	if retried := retriedOf(t, retry); retried != len(want) {
		t.Fatalf("the failed take-over was tried again %d times for %d failures", retried, len(want))
	}
}

func TestATakeOverThatEndedWithoutFailingStartsTheNextFailureFromTheFirstWait(t *testing.T) {
	retry, clock := aTakeOverRetry(t)
	for range 3 {
		retry.allowARetryWhenItIsTime()
		clock.now = clock.now.Add(longestTakeOverRetryWait)
		retry.allowARetryWhenItIsTime()
	}

	if errorValue := retry.forget(); errorValue != nil {
		t.Fatal(errorValue)
	}
	retry.allowARetryWhenItIsTime()
	clock.now = clock.now.Add(firstTakeOverRetryWait)

	if errorValue := retry.allowARetryWhenItIsTime(); errorValue != nil {
		t.Fatalf("a later failure waited longer than the first wait: %v", errorValue)
	}
}

func TestTheSafetyBackupATakeOverTookIsKeptThroughItsRetries(t *testing.T) {
	retry, clock := aTakeOverRetry(t)
	if errorValue := retry.keepTheFinding(companyhost.SafetyFinding{Archive: "/var/lib/internkim-backups/internkim-backup-20261118T090000Z.tar"}); errorValue != nil {
		t.Fatal(errorValue)
	}

	retry.allowARetryWhenItIsTime()
	clock.now = clock.now.Add(firstTakeOverRetryWait)
	if errorValue := retry.allowARetryWhenItIsTime(); errorValue != nil {
		t.Fatal(errorValue)
	}

	attempts, errorValue := retry.read()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if attempts.SafetyBackup != "/var/lib/internkim-backups/internkim-backup-20261118T090000Z.tar" || attempts.Retried != 1 {
		t.Fatalf("after a retry the take-over remembers %+v", attempts)
	}
}

func TestForgettingATakeOverLeavesItsSafetyBackupWhereItIs(t *testing.T) {
	retry, _ := aTakeOverRetry(t)
	archive := filepath.Join(t.TempDir(), "internkim-backup-20261118T090000Z.tar")
	if errorValue := os.WriteFile(archive, []byte("archive"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := retry.keepTheFinding(companyhost.SafetyFinding{Archive: archive}); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := retry.forget(); errorValue != nil {
		t.Fatal(errorValue)
	}

	attempts, errorValue := retry.read()
	if errorValue != nil || attempts.SafetyBackup != "" {
		t.Fatalf("a forgotten take-over still names %q: %v", attempts.SafetyBackup, errorValue)
	}
	if _, errorValue := os.Stat(archive); errorValue != nil {
		t.Fatalf("forgetting the take-over removed its safety backup: %v", errorValue)
	}
}

func TestATakeOverThatSucceededForgetsItsRecordAndLeavesItsSafetyBackup(t *testing.T) {
	retry, _ := aTakeOverRetry(t)
	archive := filepath.Join(t.TempDir(), "internkim-backup-20261118T090000Z.tar")
	if errorValue := os.WriteFile(archive, []byte("archive"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := retry.keepTheFinding(companyhost.SafetyFinding{Archive: archive}); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := retry.finishTheTakeOver(); errorValue != nil {
		t.Fatal(errorValue)
	}

	if _, errorValue := os.Stat(retry.StatePath); !os.IsNotExist(errorValue) {
		t.Fatalf("a take-over that succeeded left its record at %s", retry.StatePath)
	}
	if _, errorValue := os.Stat(archive); errorValue != nil {
		t.Fatalf("a take-over that succeeded removed its safety backup: %v", errorValue)
	}
}

func TestATakeOverRestoresTheSnapshotTheBoxNamedForIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), takeOverArchiveFileName)
	if errorValue := nameTheArchiveToTakeOverFrom(path, "/var/lib/internkim-backups/snapshot.tar"); errorValue != nil {
		t.Fatal(errorValue)
	}

	archive, errorValue := archiveToTakeOverFrom(path)
	if errorValue != nil || archive != "/var/lib/internkim-backups/snapshot.tar" {
		t.Fatalf("the take-over reads %q (%v) where the box named the snapshot", archive, errorValue)
	}
}

func TestATakeOverFromTheLogRestoresNoArchiveAnEarlierOneNamed(t *testing.T) {
	path := filepath.Join(t.TempDir(), takeOverArchiveFileName)
	if errorValue := nameTheArchiveToTakeOverFrom(path, "/var/lib/internkim-backups/snapshot.tar"); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := nameTheArchiveToTakeOverFrom(path, ""); errorValue != nil {
		t.Fatal(errorValue)
	}

	archive, errorValue := archiveToTakeOverFrom(path)
	if errorValue != nil || archive != "" {
		t.Fatalf("a take-over from the company's log restores %q (%v)", archive, errorValue)
	}
	if errorValue := nameTheArchiveToTakeOverFrom(path, ""); errorValue != nil {
		t.Fatalf("a second take-over from the log failed where nothing was named: %v", errorValue)
	}
}

func TestAFirstAttemptThatFoundNoCompanyIsRememberedThroughItsRetries(t *testing.T) {
	retry, clock := aTakeOverRetry(t)
	if errorValue := retry.keepTheFinding(companyhost.SafetyFinding{HeldNoCompany: true}); errorValue != nil {
		t.Fatal(errorValue)
	}

	retry.allowARetryWhenItIsTime()
	clock.now = clock.now.Add(firstTakeOverRetryWait)
	if errorValue := retry.allowARetryWhenItIsTime(); errorValue != nil {
		t.Fatal(errorValue)
	}

	attempts, errorValue := retry.read()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !attempts.finding().HeldNoCompany {
		t.Fatalf("after a retry the take-over forgot that its first attempt found no company: %+v", attempts)
	}
}
