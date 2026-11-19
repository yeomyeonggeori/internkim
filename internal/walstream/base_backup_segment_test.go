package walstream

import (
	"errors"
	"slices"
	"testing"
)

var piecesAroundAMidSegmentStart = []PieceName{
	{Timeline: 1, Start: 0x2000000, End: 0x27A0000},
	{Timeline: 1, Start: 0x27A0000, End: 0x2FA0000},
	{Timeline: 1, Start: 0x2FA0000, End: 0x30000D8},
	{Timeline: 1, Start: 0x30000D8, End: 0x3001000},
}

var aBaseBackupStartingMidSegment = BaseBackup{Name: "internkim-base-20261119T043535Z.json", Timeline: 1, Start: "0/30000D8"}

func TestTheWALABaseBackupNeedsStartsAtTheSegmentItsRedoPointLiesIn(t *testing.T) {
	first, errorValue := aBaseBackupStartingMidSegment.FirstWALNeeded()

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if first != 0x3000000 {
		t.Fatalf("a base backup whose redo point is 0/30000D8 needs WAL from %s, want 0/3000000", first)
	}
}

func TestAReplayIncludesThePieceThatEndsExactlyWhereTheBaseBackupStarts(t *testing.T) {
	first, errorValue := aBaseBackupStartingMidSegment.FirstWALNeeded()
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	covered, errorValue := piecesCovering(piecesAroundAMidSegmentStart, timelineStretch{Timeline: 1, From: first, To: endless})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !slices.Contains(covered, piecesAroundAMidSegmentStart[2]) || !slices.Contains(covered, piecesAroundAMidSegmentStart[3]) {
		t.Fatalf("the replay holds %v, without the piece that writes the start of the base backup's segment", covered)
	}
}

func TestPruningKeepsThePiecesTheOldestBaseBackupsSegmentNeeds(t *testing.T) {
	stale, errorValue := stalePieces(piecesAroundAMidSegmentStart, aBaseBackupStartingMidSegment)

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if slices.Contains(stale, piecesAroundAMidSegmentStart[2]) {
		t.Fatalf("pruning removes %v, which writes the start of the segment the base backup starts in", piecesAroundAMidSegmentStart[2])
	}
	if !slices.Contains(stale, piecesAroundAMidSegmentStart[0]) || !slices.Contains(stale, piecesAroundAMidSegmentStart[1]) {
		t.Fatalf("pruning keeps pieces wholly before the base backup's segment: %v", stale)
	}
}

func TestABaseBackupWhoseSegmentStartIsNoLongerKeptAsksForANewerOne(t *testing.T) {
	errorValue := requireTheWALFrom(piecesAroundAMidSegmentStart[3:], aBaseBackupStartingMidSegment)

	if !errors.Is(errorValue, errANewerBaseBackupIsNeeded) {
		t.Fatalf("a rebuild missing the start of the base backup's segment answered %v", errorValue)
	}
}
