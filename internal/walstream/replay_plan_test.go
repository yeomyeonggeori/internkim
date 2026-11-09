package walstream

import (
	"testing"

	"github.com/jackc/pglogrepl"
)

func TestPiecesCoveringAStretchMustRunWithoutAGap(t *testing.T) {
	names := []PieceName{
		{Timeline: 1, Start: 0x100, End: 0x200},
		{Timeline: 1, Start: 0x200, End: 0x300},
		{Timeline: 2, Start: 0x280, End: 0x400},
	}
	covered, errorValue := piecesCovering(names, timelineStretch{Timeline: 1, From: 0x180, To: 0x280})
	if errorValue != nil || len(covered) != 2 {
		t.Fatalf("covered %v: %v", covered, errorValue)
	}
	if _, errorValue := piecesCovering(names, timelineStretch{Timeline: 1, From: 0x50, To: endless}); errorValue == nil {
		t.Error("a stretch starting before the first kept piece was planned")
	}
	gapped := []PieceName{{Timeline: 1, Start: 0x100, End: 0x200}, {Timeline: 1, Start: 0x210, End: 0x300}}
	if _, errorValue := piecesCovering(gapped, timelineStretch{Timeline: 1, From: 0x100, To: endless}); errorValue == nil {
		t.Error("a gap in the kept WAL was planned over")
	}
}

func TestABaseBackupIsConsistentOnlyPastItsEnd(t *testing.T) {
	backupEnd := pglogrepl.LSN(0x500)
	if reach := consistentReach(timelineStretch{To: endless}, backupEnd); reach != backupEnd {
		t.Errorf("an open stretch must reach the backup's end, reaches %s", reach)
	}
	if reach := consistentReach(timelineStretch{To: 0x700}, backupEnd); reach != 0x700 {
		t.Errorf("a stretch that switches timelines must reach its switch point, reaches %s", reach)
	}
}

func TestATimelineHistoryNamesWhereItBranched(t *testing.T) {
	history := TimelineHistory{Timeline: 3, Content: []byte("1\t0/3004808\tno recovery target specified\n2\t0/30085A0\tno recovery target specified\n")}
	began, errorValue := history.Began()
	if errorValue != nil || began != 0x30085A0 {
		t.Fatalf("began at %s: %v", began, errorValue)
	}
	ancestors, _ := history.Ancestors()
	if len(ancestors) != 2 || ancestors[0].Timeline != 1 || ancestors[0].EndsAt != 0x3004808 {
		t.Errorf("ancestors %+v", ancestors)
	}
}
