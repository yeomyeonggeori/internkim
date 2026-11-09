package walstream

import (
	"testing"

	"github.com/jackc/pglogrepl"
)

func TestPieceNameRoundTripsAndSortsByPosition(t *testing.T) {
	piece := Piece{Timeline: 1, Start: pglogrepl.LSN(0x1_0000_0000), Bytes: make([]byte, 300)}
	name, errorValue := ParsePieceName(piece.Name())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if name != (PieceName{Timeline: 1, Start: piece.Start, End: piece.End()}) {
		t.Fatalf("parsed %+v from %s", name, piece.Name())
	}
	later := Piece{Timeline: 1, Start: piece.End(), Bytes: []byte{1}}
	if later.Name() <= piece.Name() {
		t.Fatalf("%s sorts before %s, though it starts later", later.Name(), piece.Name())
	}
}

func TestPieceNameRefusesWhatIsNotOne(t *testing.T) {
	for _, text := range []string{
		"",
		"00000001-0000000000000010-0000000000000010",
		"00000001-0000000000000020-0000000000000010",
		"1-10-20",
		"00000001-0000000000000010-0000000000000020.partial",
	} {
		if _, errorValue := ParsePieceName(text); errorValue == nil {
			t.Errorf("%q was taken for a WAL piece name", text)
		}
	}
}
