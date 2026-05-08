package fleet

import "errors"

const (
	LedgerEntryEventReceived      = "event.received"
	LedgerEntryJobReserved        = "job.reserved"
	LedgerEntryOutboxReserved     = "outbox.reserved"
	LedgerEntryOutboxSent         = "outbox.sent"
	LedgerEntryWorkspacePublished = "workspace.published"
)

type LedgerEntry struct {
	Revision      uint64
	Kind          string
	Key           string
	OwnerNodeID   string
	WorkspaceHead string
}

type Ledger struct {
	State   State
	Entries []LedgerEntry
}

func NewLedger(state State) Ledger {
	return Ledger{State: state}
}

func (ledger Ledger) Commit(entry LedgerEntry, availableNodeIDs []string) (Ledger, error) {
	if !ledger.State.CanCommit(availableNodeIDs) {
		return Ledger{}, errors.New("fleet quorum is required to commit ledger entries")
	}
	nextLedger := ledger.copy()
	entry.Revision = uint64(len(nextLedger.Entries) + 1)
	nextLedger.Entries = append(nextLedger.Entries, entry)
	return nextLedger, nil
}

func (ledger Ledger) ReserveJob(eventID string, availableNodeIDs []string) (Ledger, LedgerEntry, error) {
	owner, ok := ledger.State.SelectOwner(eventID)
	if !ok {
		return Ledger{}, LedgerEntry{}, errors.New("no active fleet node can own the job")
	}
	entry := LedgerEntry{
		Kind:        LedgerEntryJobReserved,
		Key:         eventID,
		OwnerNodeID: owner.NodeID,
	}
	nextLedger, errorValue := ledger.Commit(entry, availableNodeIDs)
	if errorValue != nil {
		return Ledger{}, LedgerEntry{}, errorValue
	}
	return nextLedger, nextLedger.Entries[len(nextLedger.Entries)-1], nil
}

func (ledger Ledger) copy() Ledger {
	entries := append([]LedgerEntry(nil), ledger.Entries...)
	return Ledger{State: ledger.State, Entries: entries}
}
