package walstream

import (
	"sync"
	"time"

	"github.com/jackc/pglogrepl"
)

const piecesATakeOverFetchesAtMost = 750

type PiecesSinceBaseBackup struct {
	mutex         sync.Mutex
	count         int
	isInitialized bool
	reached       chan struct{}
}

func NewPiecesSinceBaseBackup() *PiecesSinceBaseBackup {
	return &PiecesSinceBaseBackup{reached: make(chan struct{}, 1)}
}

func (pieces *PiecesSinceBaseBackup) add() {
	pieces.mutex.Lock()
	defer pieces.mutex.Unlock()
	pieces.count++
	if pieces.count < piecesATakeOverFetchesAtMost {
		return
	}
	select {
	case pieces.reached <- struct{}{}:
	default:
	}
}

func (pieces *PiecesSinceBaseBackup) start(count int) {
	pieces.mutex.Lock()
	defer pieces.mutex.Unlock()
	pieces.count = count
	pieces.isInitialized = true
}

func (pieces *PiecesSinceBaseBackup) counted() (int, bool) {
	pieces.mutex.Lock()
	defer pieces.mutex.Unlock()
	return pieces.count, pieces.isInitialized
}

type baseBackupFacts struct {
	hasNewest       bool
	newestTakenAt   time.Time
	piecesSince     int
	isOtherDatabase bool
	namesItsMajor   bool
}

func baseBackupIsDue(facts baseBackupFacts, now time.Time) bool {
	if !facts.hasNewest || facts.isOtherDatabase || !facts.namesItsMajor {
		return true
	}
	if now.Sub(facts.newestTakenAt) >= baseBackupEvery {
		return true
	}
	return facts.piecesSince >= piecesATakeOverFetchesAtMost
}

func aFailedAttemptIsRecent(lastFailedAt time.Time, now time.Time) bool {
	return !lastFailedAt.IsZero() && now.Sub(lastFailedAt) < upkeepCheckEvery
}

func piecesFrom(names []PieceName, start pglogrepl.LSN) int {
	count := 0
	for _, name := range names {
		if name.Start >= start {
			count++
		}
	}
	return count
}
