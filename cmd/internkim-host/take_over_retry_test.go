package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/companyhost"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

type retryClock struct{ now time.Time }

func (clock *retryClock) read() time.Time { return clock.now }

type unitsCleared struct{ cleared int }

func (units *unitsCleared) resetFailed() error {
	units.cleared++
	return nil
}

func aTakeOverRetry(t *testing.T) (takeOverRetry, *retryClock, *unitsCleared) {
	t.Helper()
	clock := &retryClock{now: time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)}
	units := &unitsCleared{}
	retry := takeOverRetry{
		StatePath:   filepath.Join(t.TempDir(), box.TakeOverRetryFileName),
		Now:         clock.read,
		ResetFailed: units.resetFailed,
	}
	return retry, clock, units
}

func TestAFailedTakeOverIsClearedForAnotherTryOnceTheFirstWaitHasPassed(t *testing.T) {
	retry, clock, units := aTakeOverRetry(t)

	if errorValue := retry.clearTheFailedUnitWhenItIsTime(); !errors.Is(errorValue, errTakeOverWaitsToRetry) {
		t.Fatalf("a take-over that has just failed was tried again at once: %v", errorValue)
	}
	clock.now = clock.now.Add(firstTakeOverRetryWait - time.Second)
	if errorValue := retry.clearTheFailedUnitWhenItIsTime(); !errors.Is(errorValue, errTakeOverWaitsToRetry) {
		t.Fatalf("the failed take-over was cleared before its wait was over: %v", errorValue)
	}
	clock.now = clock.now.Add(time.Second)

	if errorValue := retry.clearTheFailedUnitWhenItIsTime(); errorValue != nil {
		t.Fatal(errorValue)
	}
	if units.cleared != 1 {
		t.Fatalf("the failed unit was cleared %d times", units.cleared)
	}
}

func TestEachFailureDoublesTheWaitUntilItReachesTheLongest(t *testing.T) {
	retry, clock, units := aTakeOverRetry(t)
	waits := []time.Duration{}
	for range 7 {
		retry.clearTheFailedUnitWhenItIsTime()
		seenAt := clock.now
		for retry.clearTheFailedUnitWhenItIsTime() != nil {
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
	if units.cleared != len(want) {
		t.Fatalf("the failed unit was cleared %d times for %d failures", units.cleared, len(want))
	}
}

func TestATakeOverThatEndedWithoutFailingStartsTheNextFailureFromTheFirstWait(t *testing.T) {
	retry, clock, _ := aTakeOverRetry(t)
	for range 3 {
		retry.clearTheFailedUnitWhenItIsTime()
		clock.now = clock.now.Add(longestTakeOverRetryWait)
		retry.clearTheFailedUnitWhenItIsTime()
	}

	if errorValue := retry.forget(); errorValue != nil {
		t.Fatal(errorValue)
	}
	retry.clearTheFailedUnitWhenItIsTime()
	clock.now = clock.now.Add(firstTakeOverRetryWait)

	if errorValue := retry.clearTheFailedUnitWhenItIsTime(); errorValue != nil {
		t.Fatalf("a later failure waited longer than the first wait: %v", errorValue)
	}
}

func TestTheSafetyBackupATakeOverTookIsKeptThroughItsRetries(t *testing.T) {
	retry, clock, _ := aTakeOverRetry(t)
	if errorValue := retry.keepTheFinding(companyhost.SafetyFinding{Archive: "/var/lib/internkim-backups/internkim-backup-20261118T090000Z.tar"}); errorValue != nil {
		t.Fatal(errorValue)
	}

	retry.clearTheFailedUnitWhenItIsTime()
	clock.now = clock.now.Add(firstTakeOverRetryWait)
	if errorValue := retry.clearTheFailedUnitWhenItIsTime(); errorValue != nil {
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
	retry, _, _ := aTakeOverRetry(t)
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
	retry, _, _ := aTakeOverRetry(t)
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

func TestATakeOverThatFailsBringsTheBoxBackToTryItAgain(t *testing.T) {
	arguments := takeOverUnitArguments("/usr/bin/internkim", "/var/lib/internkim-backups/snapshot.tar", time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC))

	if !slices.Contains(arguments, "--property=OnFailure="+blueclaw.BoxServiceName+".service") {
		t.Fatalf("the take-over unit is started without bringing the box back when it fails: %v", arguments)
	}
	if !slices.Contains(arguments, "--unit="+takeOverUnitName) {
		t.Fatalf("the take-over unit is not started under its name: %v", arguments)
	}
}

func TestAFirstAttemptThatFoundNoCompanyIsRememberedThroughItsRetries(t *testing.T) {
	retry, clock, _ := aTakeOverRetry(t)
	if errorValue := retry.keepTheFinding(companyhost.SafetyFinding{HeldNoCompany: true}); errorValue != nil {
		t.Fatal(errorValue)
	}

	retry.clearTheFailedUnitWhenItIsTime()
	clock.now = clock.now.Add(firstTakeOverRetryWait)
	if errorValue := retry.clearTheFailedUnitWhenItIsTime(); errorValue != nil {
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
