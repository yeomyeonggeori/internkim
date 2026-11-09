package walstream

import (
	"fmt"

	"github.com/jackc/pglogrepl"
)

const pieceNameLayout = "%08X-%016X-%016X"

type Piece struct {
	Timeline int32
	Start    pglogrepl.LSN
	Bytes    []byte
}

func (piece Piece) End() pglogrepl.LSN {
	return piece.Start + pglogrepl.LSN(len(piece.Bytes))
}

func (piece Piece) Name() string {
	return PieceName{Timeline: piece.Timeline, Start: piece.Start, End: piece.End()}.String()
}

type PieceName struct {
	Timeline int32
	Start    pglogrepl.LSN
	End      pglogrepl.LSN
}

func (name PieceName) String() string {
	return fmt.Sprintf(pieceNameLayout, uint32(name.Timeline), uint64(name.Start), uint64(name.End))
}

func ParsePieceName(text string) (PieceName, error) {
	var timeline uint32
	var start, end uint64
	parsed, errorValue := fmt.Sscanf(text, pieceNameLayout, &timeline, &start, &end)
	if errorValue != nil || parsed != 3 {
		return PieceName{}, fmt.Errorf("%q is not a WAL piece name (timeline-start-end in hexadecimal)", text)
	}
	if end <= start {
		return PieceName{}, fmt.Errorf("the WAL piece %q ends before it starts", text)
	}
	name := PieceName{Timeline: int32(timeline), Start: pglogrepl.LSN(start), End: pglogrepl.LSN(end)}
	if name.String() != text {
		return PieceName{}, fmt.Errorf("%q is not a WAL piece name in its canonical spelling %q", text, name.String())
	}
	return name, nil
}
