package walstream

import (
	"context"
	"fmt"

	"github.com/jackc/pglogrepl"

	"github.com/yeomyeonggeori/internkim/internal/offsite"
)

type Position struct {
	Timeline int32
	LSN      pglogrepl.LSN
}

func (position Position) IsZero() bool {
	return position.Timeline == 0
}

func (position Position) String() string {
	return fmt.Sprintf("%s on timeline %d", position.LSN, position.Timeline)
}

func KeptEnd(ctx context.Context, destination offsite.Destination) (Position, error) {
	backup, errorValue := NewestBaseBackup(ctx, destination)
	if errorValue != nil {
		return Position{}, errorValue
	}
	names, errorValue := KeptPieces{Destination: destination, SystemID: backup.SystemID}.Names(ctx)
	if errorValue != nil {
		return Position{}, fmt.Errorf("listing the company's kept WAL: %w", errorValue)
	}
	end := endOf(names)
	if end.IsZero() {
		return Position{}, fmt.Errorf("the company keeps no WAL for the database its base backup %s is of", backup.Name)
	}
	return end, nil
}

func endOf(names []PieceName) Position {
	var end Position
	for _, name := range names {
		if name.Timeline > end.Timeline || (name.Timeline == end.Timeline && name.End > end.LSN) {
			end = Position{Timeline: name.Timeline, LSN: name.End}
		}
	}
	return end
}

func endingAt(pieces []PieceName, through Position) ([]PieceName, error) {
	for index, piece := range pieces {
		if piece.Timeline != through.Timeline || piece.End != through.LSN {
			continue
		}
		for _, later := range pieces[index+1:] {
			if later.Timeline != through.Timeline {
				return nil, fmt.Errorf("the company's kept WAL goes on to timeline %d after %s, where it was to end", later.Timeline, through)
			}
		}
		return pieces[:index+1], nil
	}
	return nil, fmt.Errorf("no piece of the company's kept WAL ends at %s; it reaches %s", through, endOf(pieces))
}
