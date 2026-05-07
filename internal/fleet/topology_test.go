package fleet

import "testing"

func TestAddBoardKeepsSecondBoardPendingAndPromotesThird(t *testing.T) {
	state, errorValue := NewState("fleet-1", "board-a")
	if errorValue != nil {
		t.Fatalf("expected state: %v", errorValue)
	}

	secondJoin, errorValue := state.AddBoard("board-b")
	if errorValue != nil {
		t.Fatalf("expected second join: %v", errorValue)
	}
	if len(secondJoin.State.ActiveMembers()) != 1 || len(secondJoin.State.PendingMembers()) != 1 {
		t.Fatalf("expected one active and one pending, got %+v", secondJoin.State.Members)
	}
	if secondJoin.Member.Status != MemberStatusPending {
		t.Fatalf("expected second board pending, got %+v", secondJoin.Member)
	}

	thirdJoin, errorValue := secondJoin.State.AddBoard("board-c")
	if errorValue != nil {
		t.Fatalf("expected third join: %v", errorValue)
	}
	if len(thirdJoin.State.ActiveMembers()) != 3 || len(thirdJoin.State.PendingMembers()) != 0 {
		t.Fatalf("expected three active boards, got %+v", thirdJoin.State.Members)
	}
	if thirdJoin.State.QuorumSize() != 2 {
		t.Fatalf("expected quorum 2, got %d", thirdJoin.State.QuorumSize())
	}
}

func TestAddBoardKeepsFourthPendingAndPromotesFifth(t *testing.T) {
	state, _ := NewState("fleet-1", "board-a")
	secondJoin, _ := state.AddBoard("board-b")
	thirdJoin, _ := secondJoin.State.AddBoard("board-c")
	fourthJoin, _ := thirdJoin.State.AddBoard("board-d")

	if len(fourthJoin.State.ActiveMembers()) != 3 || len(fourthJoin.State.PendingMembers()) != 1 {
		t.Fatalf("expected fourth board pending, got %+v", fourthJoin.State.Members)
	}

	fifthJoin, errorValue := fourthJoin.State.AddBoard("board-e")
	if errorValue != nil {
		t.Fatalf("expected fifth join: %v", errorValue)
	}
	if len(fifthJoin.State.ActiveMembers()) != 5 || len(fifthJoin.State.PendingMembers()) != 0 {
		t.Fatalf("expected five active boards, got %+v", fifthJoin.State.Members)
	}
	if fifthJoin.State.QuorumSize() != 3 {
		t.Fatalf("expected quorum 3, got %d", fifthJoin.State.QuorumSize())
	}
}

func TestCanCommitRequiresMajorityOfActiveBoards(t *testing.T) {
	state, _ := NewState("fleet-1", "board-a")
	secondJoin, _ := state.AddBoard("board-b")
	thirdJoin, _ := secondJoin.State.AddBoard("board-c")

	if !thirdJoin.State.CanCommit([]string{"board-a", "board-c"}) {
		t.Fatal("expected two active boards to commit")
	}
	if thirdJoin.State.CanCommit([]string{"board-a"}) {
		t.Fatal("expected one active board not to commit in a three-board fleet")
	}
	if thirdJoin.State.CanCommit([]string{"board-b"}) {
		t.Fatal("expected pending board history not to matter after promotion state check")
	}
}

func TestSelectOwnerIgnoresPendingBoards(t *testing.T) {
	state, _ := NewState("fleet-1", "board-a")
	secondJoin, _ := state.AddBoard("board-b")

	owner, ok := secondJoin.State.SelectOwner("event-1")
	if !ok {
		t.Fatal("expected owner")
	}
	if owner.BoardID != "board-a" {
		t.Fatalf("expected active board to own event, got %+v", owner)
	}
}

func TestRemoveActiveBoardRequiresStandaloneReset(t *testing.T) {
	state, _ := NewState("fleet-1", "board-a")
	secondJoin, _ := state.AddBoard("board-b")
	thirdJoin, _ := secondJoin.State.AddBoard("board-c")

	_, errorValue := thirdJoin.State.RemoveBoard("board-c")
	if errorValue == nil {
		t.Fatal("expected active board removal to be rejected")
	}

	standaloneState, errorValue := thirdJoin.State.ResetStandalone("board-a")
	if errorValue != nil {
		t.Fatalf("expected standalone reset: %v", errorValue)
	}
	if len(standaloneState.ActiveMembers()) != 1 || standaloneState.QuorumSize() != 1 {
		t.Fatalf("expected standalone state, got %+v", standaloneState)
	}
}

func TestLedgerCommitRequiresFleetQuorum(t *testing.T) {
	state, _ := NewState("fleet-1", "board-a")
	secondJoin, _ := state.AddBoard("board-b")
	thirdJoin, _ := secondJoin.State.AddBoard("board-c")
	ledger := NewLedger(thirdJoin.State)

	_, _, errorValue := ledger.ReserveJob("event-1", []string{"board-a"})
	if errorValue == nil {
		t.Fatal("expected minority reservation to fail")
	}

	nextLedger, entry, errorValue := ledger.ReserveJob("event-1", []string{"board-a", "board-c"})
	if errorValue != nil {
		t.Fatalf("expected majority reservation: %v", errorValue)
	}
	if entry.Revision != 1 || entry.Kind != LedgerEntryJobReserved || len(nextLedger.Entries) != 1 {
		t.Fatalf("unexpected ledger entry: %+v ledger=%+v", entry, nextLedger)
	}
}
