package fleet

import "testing"

func TestAddNodeKeepsSecondNodePendingAndPromotesThird(t *testing.T) {
	state, errorValue := NewState("fleet-1", "node-a")
	if errorValue != nil {
		t.Fatalf("expected state: %v", errorValue)
	}

	secondJoin, errorValue := state.AddNode("node-b")
	if errorValue != nil {
		t.Fatalf("expected second join: %v", errorValue)
	}
	if len(secondJoin.State.ActiveMembers()) != 1 || len(secondJoin.State.PendingMembers()) != 1 {
		t.Fatalf("expected one active and one pending, got %+v", secondJoin.State.Members)
	}
	if secondJoin.Member.Status != MemberStatusPending {
		t.Fatalf("expected second node pending, got %+v", secondJoin.Member)
	}

	thirdJoin, errorValue := secondJoin.State.AddNode("node-c")
	if errorValue != nil {
		t.Fatalf("expected third join: %v", errorValue)
	}
	if len(thirdJoin.State.ActiveMembers()) != 3 || len(thirdJoin.State.PendingMembers()) != 0 {
		t.Fatalf("expected three active nodes, got %+v", thirdJoin.State.Members)
	}
	if thirdJoin.State.QuorumSize() != 2 {
		t.Fatalf("expected quorum 2, got %d", thirdJoin.State.QuorumSize())
	}
}

func TestAddNodeKeepsFourthPendingAndPromotesFifth(t *testing.T) {
	state, _ := NewState("fleet-1", "node-a")
	secondJoin, _ := state.AddNode("node-b")
	thirdJoin, _ := secondJoin.State.AddNode("node-c")
	fourthJoin, _ := thirdJoin.State.AddNode("node-d")

	if len(fourthJoin.State.ActiveMembers()) != 3 || len(fourthJoin.State.PendingMembers()) != 1 {
		t.Fatalf("expected fourth node pending, got %+v", fourthJoin.State.Members)
	}

	fifthJoin, errorValue := fourthJoin.State.AddNode("node-e")
	if errorValue != nil {
		t.Fatalf("expected fifth join: %v", errorValue)
	}
	if len(fifthJoin.State.ActiveMembers()) != 5 || len(fifthJoin.State.PendingMembers()) != 0 {
		t.Fatalf("expected five active nodes, got %+v", fifthJoin.State.Members)
	}
	if fifthJoin.State.QuorumSize() != 3 {
		t.Fatalf("expected quorum 3, got %d", fifthJoin.State.QuorumSize())
	}
}

func TestCanCommitRequiresMajorityOfActiveNodes(t *testing.T) {
	state, _ := NewState("fleet-1", "node-a")
	secondJoin, _ := state.AddNode("node-b")
	thirdJoin, _ := secondJoin.State.AddNode("node-c")

	if !thirdJoin.State.CanCommit([]string{"node-a", "node-c"}) {
		t.Fatal("expected two active nodes to commit")
	}
	if thirdJoin.State.CanCommit([]string{"node-a"}) {
		t.Fatal("expected one active node not to commit in a three-node fleet")
	}
	if thirdJoin.State.CanCommit([]string{"node-b"}) {
		t.Fatal("expected pending node history not to matter after promotion state check")
	}
}

func TestSelectOwnerIgnoresPendingNodes(t *testing.T) {
	state, _ := NewState("fleet-1", "node-a")
	secondJoin, _ := state.AddNode("node-b")

	owner, ok := secondJoin.State.SelectOwner("event-1")
	if !ok {
		t.Fatal("expected owner")
	}
	if owner.NodeID != "node-a" {
		t.Fatalf("expected active node to own event, got %+v", owner)
	}
}

func TestRemoveActiveNodeRequiresStandaloneReset(t *testing.T) {
	state, _ := NewState("fleet-1", "node-a")
	secondJoin, _ := state.AddNode("node-b")
	thirdJoin, _ := secondJoin.State.AddNode("node-c")

	_, errorValue := thirdJoin.State.RemoveNode("node-c")
	if errorValue == nil {
		t.Fatal("expected active node removal to be rejected")
	}

	standaloneState, errorValue := thirdJoin.State.ResetStandalone("node-a")
	if errorValue != nil {
		t.Fatalf("expected standalone reset: %v", errorValue)
	}
	if len(standaloneState.ActiveMembers()) != 1 || standaloneState.QuorumSize() != 1 {
		t.Fatalf("expected standalone state, got %+v", standaloneState)
	}
}

func TestLedgerCommitRequiresFleetQuorum(t *testing.T) {
	state, _ := NewState("fleet-1", "node-a")
	secondJoin, _ := state.AddNode("node-b")
	thirdJoin, _ := secondJoin.State.AddNode("node-c")
	ledger := NewLedger(thirdJoin.State)

	_, _, errorValue := ledger.ReserveJob("event-1", []string{"node-a"})
	if errorValue == nil {
		t.Fatal("expected minority reservation to fail")
	}

	nextLedger, entry, errorValue := ledger.ReserveJob("event-1", []string{"node-a", "node-c"})
	if errorValue != nil {
		t.Fatalf("expected majority reservation: %v", errorValue)
	}
	if entry.Revision != 1 || entry.Kind != LedgerEntryJobReserved || len(nextLedger.Entries) != 1 {
		t.Fatalf("unexpected ledger entry: %+v ledger=%+v", entry, nextLedger)
	}
}
