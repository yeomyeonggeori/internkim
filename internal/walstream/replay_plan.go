package walstream

import (
	"context"
	"fmt"
	"math"

	"github.com/jackc/pglogrepl"

	"github.com/yeomyeonggeori/internkim/internal/box"
)

type replayPlan struct {
	Pieces    []PieceName
	Histories []TimelineHistory
}

func (plan replayPlan) replaysTo() pglogrepl.LSN {
	return plan.Pieces[len(plan.Pieces)-1].End
}

const endless = pglogrepl.LSN(math.MaxUint64)

func consistentReach(stretch timelineStretch, backupEnd pglogrepl.LSN) pglogrepl.LSN {
	if stretch.To == endless {
		return backupEnd
	}
	return max(backupEnd, stretch.To)
}

type timelineStretch struct {
	Timeline int32
	From     pglogrepl.LSN
	To       pglogrepl.LSN
}

func planTheReplay(ctx context.Context, kept KeptPieces, backup BaseBackup, keyring box.Keyring) (replayPlan, error) {
	start, errorValue := backup.StartLSN()
	if errorValue != nil {
		return replayPlan{}, fmt.Errorf("the base backup %s names no start: %w", backup.Name, errorValue)
	}
	end, errorValue := backup.EndLSN()
	if errorValue != nil {
		return replayPlan{}, fmt.Errorf("the base backup %s names no end: %w", backup.Name, errorValue)
	}
	names, errorValue := kept.Names(ctx)
	if errorValue != nil {
		return replayPlan{}, fmt.Errorf("listing the company's kept WAL: %w", errorValue)
	}
	newest := backup.Timeline
	for _, name := range names {
		newest = max(newest, name.Timeline)
	}
	stretches, histories, errorValue := stretchesFrom(ctx, kept, keyring, backup, start, newest)
	if errorValue != nil {
		return replayPlan{}, errorValue
	}
	plan := replayPlan{Histories: histories}
	for index, stretch := range stretches {
		covered, errorValue := piecesCovering(names, stretch)
		if errorValue != nil {
			return replayPlan{}, errorValue
		}
		reach := stretch.To
		if index == 0 {
			reach = consistentReach(stretch, end)
		}
		if reach != endless && covered[len(covered)-1].End < reach {
			return replayPlan{}, fmt.Errorf("the company's kept WAL on timeline %d stops at %s, before %s; the database cannot be rebuilt past it", stretch.Timeline, covered[len(covered)-1].End, reach)
		}
		plan.Pieces = append(plan.Pieces, covered...)
	}
	return plan, nil
}

func stretchesFrom(ctx context.Context, kept KeptPieces, keyring box.Keyring, backup BaseBackup, start pglogrepl.LSN, newest int32) ([]timelineStretch, []TimelineHistory, error) {
	if newest == backup.Timeline {
		return []timelineStretch{{Timeline: newest, From: start, To: endless}}, nil, nil
	}
	newestHistory, errorValue := kept.ReadHistory(ctx, newest, keyring)
	if errorValue != nil {
		return nil, nil, errorValue
	}
	ancestors, errorValue := newestHistory.Ancestors()
	if errorValue != nil {
		return nil, nil, errorValue
	}
	stretches := []timelineStretch{}
	histories := []TimelineHistory{newestHistory}
	from := start
	for _, ancestor := range ancestors {
		if ancestor.Timeline < backup.Timeline {
			continue
		}
		stretches = append(stretches, timelineStretch{Timeline: ancestor.Timeline, From: from, To: ancestor.EndsAt})
		from = ancestor.EndsAt
		if ancestor.Timeline > 1 {
			history, errorValue := kept.ReadHistory(ctx, ancestor.Timeline, keyring)
			if errorValue != nil {
				return nil, nil, errorValue
			}
			histories = append(histories, history)
		}
	}
	if len(stretches) == 0 || stretches[0].Timeline != backup.Timeline {
		return nil, nil, fmt.Errorf("timeline %d, the newest the company keeps WAL on, did not branch from timeline %d of the base backup %s", newest, backup.Timeline, backup.Name)
	}
	return append(stretches, timelineStretch{Timeline: newest, From: from, To: endless}), histories, nil
}

func piecesCovering(names []PieceName, stretch timelineStretch) ([]PieceName, error) {
	covered := []PieceName{}
	for _, name := range names {
		if name.Timeline == stretch.Timeline && name.End > stretch.From && name.Start < stretch.To {
			covered = append(covered, name)
		}
	}
	if len(covered) == 0 || covered[0].Start > stretch.From {
		return nil, fmt.Errorf("the company keeps no WAL on timeline %d from %s, so the database cannot be rebuilt", stretch.Timeline, stretch.From)
	}
	for index := 1; index < len(covered); index++ {
		if covered[index].Start != covered[index-1].End {
			return nil, fmt.Errorf("the company's kept WAL on timeline %d has a gap between %s and %s; the database cannot be rebuilt past it", stretch.Timeline, covered[index-1].End, covered[index].Start)
		}
	}
	return covered, nil
}
