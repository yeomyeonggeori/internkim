package admind

import "testing"

func TestBuzzProfileNameToWriteReadsARecordedNameTheWayTheCompanyReadsIt(t *testing.T) {
	if actual := buzzProfileNameToWrite(buzzProfileContent{}, "찬희 이", "ko"); actual != "이찬희" {
		t.Errorf("buzzProfileNameToWrite = %q, want 이찬희", actual)
	}
}

func TestBuzzProfileNameToWriteLeavesTheNameTheRelayAlreadyHolds(t *testing.T) {
	held := buzzProfileContent{Display: "찬희"}
	if actual := buzzProfileNameToWrite(held, "찬희 이", "ko"); actual != "" {
		t.Errorf("buzzProfileNameToWrite = %q, want the held name left alone", actual)
	}
}

func TestBuzzProfileNameToWriteWritesNothingForSomebodyTheDirectoryDoesNotName(t *testing.T) {
	if actual := buzzProfileNameToWrite(buzzProfileContent{}, "", "ko"); actual != "" {
		t.Errorf("buzzProfileNameToWrite = %q, want nothing", actual)
	}
}
