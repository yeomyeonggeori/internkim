package companyhost

import (
	"os"
	"path/filepath"
	"testing"
)

func requireSafetyStep(t *testing.T, earlier SafetyFinding, holdsACompany bool, want safetyStep) {
	t.Helper()
	step, errorValue := safetyStepFor(earlier, holdsACompany)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if step != want {
		t.Fatalf("with %+v found earlier and a company held %v, the step is %d, want %d", earlier, holdsACompany, step, want)
	}
}

func TestAFirstAttemptOnABoxHoldingACompanyTakesASafetyBackup(t *testing.T) {
	requireSafetyStep(t, SafetyFinding{}, true, takeASafetyBackup)
}

func TestAFirstAttemptOnABoxHoldingNoCompanyRecordsThatItHeldNone(t *testing.T) {
	requireSafetyStep(t, SafetyFinding{}, false, recordThatNoCompanyWasHeld)
}

func TestALaterAttemptAfterOneThatFoundNoCompanyKeepsNothing(t *testing.T) {
	requireSafetyStep(t, SafetyFinding{HeldNoCompany: true}, true, keepNothingAnEarlierAttemptFound)
}

func TestALaterAttemptReliesOnTheArchiveTheFirstAttemptKept(t *testing.T) {
	kept := filepath.Join(t.TempDir(), "internkim-backup-20261118T090000Z.tar")
	if errorValue := os.WriteFile(kept, []byte("archive"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	requireSafetyStep(t, SafetyFinding{Archive: kept}, true, relyOnTheEarlierArchive)
}

func TestALaterAttemptWhoseArchiveIsGoneTakesANewOne(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "internkim-backup-20261118T090000Z.tar")

	requireSafetyStep(t, SafetyFinding{Archive: gone}, true, takeASafetyBackup)
}
