package admind

import "testing"

func TestBuzzProfileNameToWriteReadsARecordedNameTheWayTheCompanyReadsIt(t *testing.T) {
	if actual := buzzProfileNameToWrite(buzzProfileContent{}, "모형 이", "ko"); actual != "이모형" {
		t.Errorf("buzzProfileNameToWrite = %q, want 이모형", actual)
	}
}

func TestBuzzProfileNameToWriteLeavesTheNameTheRelayAlreadyHolds(t *testing.T) {
	held := buzzProfileContent{Display: "모형"}
	if actual := buzzProfileNameToWrite(held, "모형 이", "ko"); actual != "" {
		t.Errorf("buzzProfileNameToWrite = %q, want the held name left alone", actual)
	}
}

func TestBuzzProfileNameToWriteWritesNothingForSomebodyTheDirectoryDoesNotName(t *testing.T) {
	if actual := buzzProfileNameToWrite(buzzProfileContent{}, "", "ko"); actual != "" {
		t.Errorf("buzzProfileNameToWrite = %q, want nothing", actual)
	}
}
