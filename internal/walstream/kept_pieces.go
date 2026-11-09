package walstream

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/jackc/pglogrepl"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/offsite"
)

const (
	pieceFolder      = "wal"
	keepAttempts     = 3
	firstKeepBackoff = 200 * time.Millisecond
)

type KeptPieces struct {
	Destination offsite.Destination
	SystemID    string
}

func (kept KeptPieces) folder() string {
	return kept.Destination.CompanyID + "/" + pieceFolder + "/" + kept.SystemID + "/"
}

func (kept KeptPieces) pathOf(name string) string {
	return kept.folder() + name
}

func (kept KeptPieces) Keep(ctx context.Context, piece Piece) error {
	name := piece.Name()
	var sealed bytes.Buffer
	sealing, errorValue := box.SealStreamTo(&sealed, kept.Destination.SealingKey, box.WALPiecePurpose(kept.Destination.CompanyID, kept.SystemID+"/"+name))
	if errorValue != nil {
		return errorValue
	}
	if _, errorValue := sealing.Write(piece.Bytes); errorValue != nil {
		return errorValue
	}
	if errorValue := sealing.Close(); errorValue != nil {
		return errorValue
	}
	backoff := firstKeepBackoff
	for attempt := 1; ; attempt++ {
		errorValue = kept.Destination.Store.Put(ctx, kept.pathOf(name), sealed.Bytes())
		if errorValue == nil || attempt == keepAttempts || ctx.Err() != nil {
			break
		}
		select {
		case <-ctx.Done():
		case <-time.After(backoff):
		}
		backoff *= 2
	}
	if errorValue != nil {
		return fmt.Errorf("keeping the WAL piece %s (%d bytes) after %d attempts: %w", name, len(piece.Bytes), keepAttempts, errorValue)
	}
	return nil
}

func (kept KeptPieces) Names(ctx context.Context) ([]PieceName, error) {
	listed, errorValue := kept.Destination.Store.List(ctx, kept.folder())
	if errorValue != nil {
		return nil, errorValue
	}
	names := make([]PieceName, 0, len(listed))
	for _, object := range listed {
		name, errorValue := ParsePieceName(object.Name)
		if errorValue != nil {
			return nil, fmt.Errorf("the company's WAL folder holds %q: %w", object.Name, errorValue)
		}
		names = append(names, name)
	}
	slices.SortFunc(names, func(left, right PieceName) int {
		if left.Start != right.Start {
			return compareLSN(left.Start, right.Start)
		}
		return int(left.Timeline - right.Timeline)
	})
	return names, nil
}

func (kept KeptPieces) Read(ctx context.Context, name PieceName, keyring box.Keyring) (Piece, error) {
	sealed, errorValue := kept.Destination.Store.Get(ctx, kept.pathOf(name.String()))
	if errorValue != nil {
		return Piece{}, errorValue
	}
	opened, errorValue := keyring.OpenStream(bytes.NewReader(sealed), box.WALPiecePurpose(kept.Destination.CompanyID, kept.SystemID+"/"+name.String()))
	if errorValue != nil {
		return Piece{}, fmt.Errorf("opening the WAL piece %s: %w", name, errorValue)
	}
	contents, errorValue := io.ReadAll(opened)
	if errorValue != nil {
		return Piece{}, fmt.Errorf("opening the WAL piece %s: %w", name, errorValue)
	}
	piece := Piece{Timeline: name.Timeline, Start: name.Start, Bytes: contents}
	if piece.End() != name.End {
		return Piece{}, fmt.Errorf("the WAL piece %s holds %d bytes, which its name does not account for", name, len(contents))
	}
	return piece, nil
}

func (kept KeptPieces) NewestEnd(ctx context.Context, timeline int32) (pglogrepl.LSN, bool, error) {
	names, errorValue := kept.Names(ctx)
	if errorValue != nil {
		return 0, false, errorValue
	}
	var newest pglogrepl.LSN
	isFound := false
	for _, name := range names {
		if name.Timeline == timeline && name.End > newest {
			newest = name.End
			isFound = true
		}
	}
	return newest, isFound, nil
}

func compareLSN(left, right pglogrepl.LSN) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	}
	return 0
}
