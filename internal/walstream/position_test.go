package walstream

import "testing"

func TestAMoveReplaysTheKeptWALExactlyToWhereTheOldBoxStopped(t *testing.T) {
	pieces := []PieceName{
		{Timeline: 1, Start: 0x100, End: 0x200},
		{Timeline: 2, Start: 0x200, End: 0x300},
		{Timeline: 2, Start: 0x300, End: 0x380},
	}
	ending, errorValue := endingAt(pieces, Position{Timeline: 2, LSN: 0x300})
	if errorValue != nil || len(ending) != 2 || ending[1].End != 0x300 {
		t.Fatalf("ending at 0x300 on timeline 2 replays %v: %v", ending, errorValue)
	}
	if _, errorValue := endingAt(pieces, Position{Timeline: 2, LSN: 0x400}); errorValue == nil {
		t.Error("a replay was planned to a position the store does not yet reach")
	}
	if _, errorValue := endingAt(pieces, Position{Timeline: 2, LSN: 0x340}); errorValue == nil {
		t.Error("a replay was planned to a position inside a piece")
	}
	if _, errorValue := endingAt(pieces, Position{Timeline: 1, LSN: 0x200}); errorValue == nil {
		t.Error("a replay was planned to stop on a timeline the kept WAL goes on from")
	}
}

func TestTheKeptWALEndsWhereItsNewestTimelineEnds(t *testing.T) {
	end := endOf([]PieceName{
		{Timeline: 1, Start: 0x100, End: 0x900},
		{Timeline: 2, Start: 0x200, End: 0x300},
		{Timeline: 2, Start: 0x300, End: 0x380},
	})
	if end != (Position{Timeline: 2, LSN: 0x380}) {
		t.Fatalf("the kept WAL ends at %s", end)
	}
}
