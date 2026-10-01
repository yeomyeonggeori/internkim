package admind

import "testing"

func TestBuzzProfileNameToWriteReadsARecordedNameTheWayTheCompanyReadsIt(t *testing.T) {
	if actual := buzzProfileNameToWrite(buzzProfileContent{}, "샘플 이", "ko"); actual != "이샘플" {
		t.Errorf("buzzProfileNameToWrite = %q, want 이샘플", actual)
	}
}

func TestBuzzProfileNameToWriteLeavesTheNameTheRelayAlreadyHolds(t *testing.T) {
	held := buzzProfileContent{Display: "샘플"}
	if actual := buzzProfileNameToWrite(held, "샘플 이", "ko"); actual != "" {
		t.Errorf("buzzProfileNameToWrite = %q, want the held name left alone", actual)
	}
}

func TestBuzzProfileNameToWriteWritesNothingForSomebodyTheDirectoryDoesNotName(t *testing.T) {
	if actual := buzzProfileNameToWrite(buzzProfileContent{}, "", "ko"); actual != "" {
		t.Errorf("buzzProfileNameToWrite = %q, want nothing", actual)
	}
}
