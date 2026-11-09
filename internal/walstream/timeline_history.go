package walstream

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/jackc/pglogrepl"

	"github.com/yeomyeonggeori/internkim/internal/box"
)

const historyFolder = "wal-history"

type TimelineSwitch struct {
	Timeline int32
	EndsAt   pglogrepl.LSN
}

type TimelineHistory struct {
	Timeline int32
	Content  []byte
}

func (history TimelineHistory) FileName() string {
	return fmt.Sprintf("%08X.history", uint32(history.Timeline))
}

func (history TimelineHistory) Ancestors() ([]TimelineSwitch, error) {
	ancestors := []TimelineSwitch{}
	scanner := bufio.NewScanner(bytes.NewReader(history.Content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return nil, fmt.Errorf("the history of timeline %d holds the line %q, which names no switch point", history.Timeline, line)
		}
		timeline, errorValue := strconv.ParseInt(fields[0], 10, 32)
		if errorValue != nil {
			return nil, fmt.Errorf("the history of timeline %d holds the line %q: %w", history.Timeline, line, errorValue)
		}
		endsAt, errorValue := pglogrepl.ParseLSN(fields[1])
		if errorValue != nil {
			return nil, fmt.Errorf("the history of timeline %d holds the line %q: %w", history.Timeline, line, errorValue)
		}
		ancestors = append(ancestors, TimelineSwitch{Timeline: int32(timeline), EndsAt: endsAt})
	}
	return ancestors, scanner.Err()
}

func (history TimelineHistory) Began() (pglogrepl.LSN, error) {
	ancestors, errorValue := history.Ancestors()
	if errorValue != nil {
		return 0, errorValue
	}
	if len(ancestors) == 0 {
		return 0, fmt.Errorf("the history of timeline %d names no timeline it branched from", history.Timeline)
	}
	return ancestors[len(ancestors)-1].EndsAt, nil
}

func (kept KeptPieces) historyPath(timeline int32) string {
	return kept.Destination.CompanyID + "/" + historyFolder + "/" + kept.SystemID + "/" + TimelineHistory{Timeline: timeline}.FileName()
}

func (kept KeptPieces) KeepHistory(ctx context.Context, history TimelineHistory) error {
	var sealed bytes.Buffer
	sealing, errorValue := box.SealStreamTo(&sealed, kept.Destination.SealingKey, box.WALPiecePurpose(kept.Destination.CompanyID, kept.SystemID+"/"+history.FileName()))
	if errorValue != nil {
		return errorValue
	}
	if _, errorValue := sealing.Write(history.Content); errorValue != nil {
		return errorValue
	}
	if errorValue := sealing.Close(); errorValue != nil {
		return errorValue
	}
	if errorValue := kept.Destination.Store.Put(ctx, kept.historyPath(history.Timeline), sealed.Bytes()); errorValue != nil {
		return fmt.Errorf("keeping the history of timeline %d: %w", history.Timeline, errorValue)
	}
	return nil
}

func (kept KeptPieces) ReadHistory(ctx context.Context, timeline int32, keyring box.Keyring) (TimelineHistory, error) {
	history := TimelineHistory{Timeline: timeline}
	sealed, errorValue := kept.Destination.Store.Get(ctx, kept.historyPath(timeline))
	if errorValue != nil {
		return TimelineHistory{}, fmt.Errorf("reading the history of timeline %d: %w", timeline, errorValue)
	}
	opened, errorValue := keyring.OpenStream(bytes.NewReader(sealed), box.WALPiecePurpose(kept.Destination.CompanyID, kept.SystemID+"/"+history.FileName()))
	if errorValue != nil {
		return TimelineHistory{}, fmt.Errorf("opening the history of timeline %d: %w", timeline, errorValue)
	}
	history.Content, errorValue = io.ReadAll(opened)
	if errorValue != nil {
		return TimelineHistory{}, fmt.Errorf("opening the history of timeline %d: %w", timeline, errorValue)
	}
	return history, nil
}
