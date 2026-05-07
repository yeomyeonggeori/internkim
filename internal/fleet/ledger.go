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
	OwnerBoardID  string
	WorkspaceHead string
}

type Ledger struct {
	State   State
	Entries []LedgerEntry
}

func NewLedger(state State) Ledger {
	return Ledger{State: state}
}

func (ledger Ledger) Commit(entry LedgerEntry, availableBoardIDs []string) (Ledger, error) {
	if !ledger.State.CanCommit(availableBoardIDs) {
		return Ledger{}, errors.New("fleet quorum is required to commit ledger entries")
	}
	nextLedger := ledger.copy()
	entry.Revision = uint64(len(nextLedger.Entries) + 1)
	nextLedger.Entries = append(nextLedger.Entries, entry)
	return nextLedger, nil
}

func (ledger Ledger) ReserveJob(eventID string, availableBoardIDs []string) (Ledger, LedgerEntry, error) {
	owner, ok := ledger.State.SelectOwner(eventID)
	if !ok {
		return Ledger{}, LedgerEntry{}, errors.New("no active fleet board can own the job")
	}
	entry := LedgerEntry{
		Kind:         LedgerEntryJobReserved,
		Key:          eventID,
		OwnerBoardID: owner.BoardID,
	}
	nextLedger, errorValue := ledger.Commit(entry, availableBoardIDs)
	if errorValue != nil {
		return Ledger{}, LedgerEntry{}, errorValue
	}
	return nextLedger, nextLedger.Entries[len(nextLedger.Entries)-1], nil
}

func (ledger Ledger) copy() Ledger {
	entries := append([]LedgerEntry(nil), ledger.Entries...)
	return Ledger{State: ledger.State, Entries: entries}
}
