package offsite

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
)

const (
	shareDirectoryMode = 0o700
	sharePieceMode     = 0o600
	partialSuffix      = ".partial"
)

type Share struct {
	DirectoryPath string
}

type logPiece struct {
	path      string
	createdAt time.Time
}

func (share Share) KeepUp(ctx context.Context, destination Destination, logFolders []string, thisBox string, placement box.SharePlacement) (box.ShareReport, error) {
	pieces, errorValue := listTheLog(ctx, destination, logFolders)
	if errorValue != nil {
		return box.ShareReport{}, errorValue
	}
	held, errorValue := share.heldPieces()
	if errorValue != nil {
		return box.ShareReport{}, errorValue
	}
	standbys := standbyKeysOf(placement)
	mine := piecesOwnedBy(thisBox, standbys, pieces)
	if errorValue := share.fetch(ctx, destination, piecesNotHeld(mine, held)); errorValue != nil {
		return box.ShareReport{}, errorValue
	}
	if errorValue := share.handOn(held, pieces, thisBox, placement); errorValue != nil {
		return box.ShareReport{}, errorValue
	}
	stillHeld, errorValue := share.heldPieces()
	if errorValue != nil {
		return box.ShareReport{}, errorValue
	}
	return box.ShareReport{
		RankedAgainst: placement.StandbysDigest,
		KeptThrough:   newestOf(pieces),
		SharePieces:   len(mine),
		HeldPieces:    len(stillHeld),
		LogPieces:     len(pieces),
	}, nil
}

func listTheLog(ctx context.Context, destination Destination, logFolders []string) ([]logPiece, error) {
	pieces := []logPiece{}
	for _, folder := range logFolders {
		listed, errorValue := destination.Store.ListEvery(ctx, destination.CompanyID+"/"+folder)
		if errorValue != nil {
			return nil, errorValue
		}
		for _, object := range listed {
			pieces = append(pieces, logPiece{path: folder + object.Name, createdAt: object.CreatedAt})
		}
	}
	return pieces, nil
}

func ownerOf(standbys []string, path string) string {
	owner, ownerScore := "", uint64(0)
	for _, standby := range standbys {
		score := rendezvousScore(standby, path)
		if owner == "" || score > ownerScore || (score == ownerScore && standby < owner) {
			owner, ownerScore = standby, score
		}
	}
	return owner
}

func rendezvousScore(publicKey, path string) uint64 {
	hashed := sha256.Sum256([]byte(publicKey + "\n" + path))
	return binary.BigEndian.Uint64(hashed[:8])
}

func standbyKeysOf(placement box.SharePlacement) []string {
	keys := make([]string, 0, len(placement.Standbys))
	for _, standby := range placement.Standbys {
		keys = append(keys, standby.PublicKey)
	}
	return keys
}

func piecesOwnedBy(thisBox string, standbys []string, pieces []logPiece) []logPiece {
	owned := []logPiece{}
	for _, piece := range pieces {
		if ownerOf(standbys, piece.path) == thisBox {
			owned = append(owned, piece)
		}
	}
	return owned
}

func piecesNotHeld(pieces []logPiece, held map[string]bool) []logPiece {
	missing := []logPiece{}
	for _, piece := range pieces {
		if !held[piece.path] {
			missing = append(missing, piece)
		}
	}
	return missing
}

func newestOf(pieces []logPiece) *time.Time {
	var newest *time.Time
	for _, piece := range pieces {
		if newest == nil || piece.createdAt.After(*newest) {
			createdAt := piece.createdAt
			newest = &createdAt
		}
	}
	return newest
}

func (share Share) fetch(ctx context.Context, destination Destination, pieces []logPiece) error {
	keep := func(fetching context.Context, piece logPiece) (logPiece, error) {
		return piece, share.keep(fetching, destination, piece.path)
	}
	return FetchAhead(ctx, pieces, keep, func(logPiece) error { return nil })
}

func (share Share) keep(ctx context.Context, destination Destination, path string) error {
	sealed, errorValue := destination.Store.Get(ctx, destination.CompanyID+"/"+path)
	if errorValue != nil {
		return errorValue
	}
	heldPath := share.pathOf(path)
	if errorValue := os.MkdirAll(filepath.Dir(heldPath), shareDirectoryMode); errorValue != nil {
		return errorValue
	}
	if errorValue := os.WriteFile(heldPath+partialSuffix, sealed, sharePieceMode); errorValue != nil {
		return errorValue
	}
	return os.Rename(heldPath+partialSuffix, heldPath)
}

func (share Share) handOn(held map[string]bool, pieces []logPiece, thisBox string, placement box.SharePlacement) error {
	listed := map[string]logPiece{}
	for _, piece := range pieces {
		listed[piece.path] = piece
	}
	for path := range held {
		piece, isListed := listed[path]
		if isListed && !isHeldByItsOwner(piece, thisBox, placement) {
			continue
		}
		if errorValue := os.Remove(share.pathOf(path)); errorValue != nil && !errors.Is(errorValue, fs.ErrNotExist) {
			return errorValue
		}
	}
	return nil
}

func isHeldByItsOwner(piece logPiece, thisBox string, placement box.SharePlacement) bool {
	owner := ownerOf(standbyKeysOf(placement), piece.path)
	if owner == thisBox {
		return false
	}
	for _, standby := range placement.Standbys {
		if standby.PublicKey == owner && standby.KeptThrough != nil {
			return !piece.createdAt.After(*standby.KeptThrough)
		}
	}
	return false
}

func (share Share) heldPieces() (map[string]bool, error) {
	held := map[string]bool{}
	errorValue := filepath.WalkDir(share.DirectoryPath, func(path string, entry fs.DirEntry, walkError error) error {
		if errors.Is(walkError, fs.ErrNotExist) && path == share.DirectoryPath {
			return filepath.SkipDir
		}
		if walkError != nil || entry.IsDir() {
			return walkError
		}
		if strings.HasSuffix(path, partialSuffix) {
			return os.Remove(path)
		}
		relative, errorValue := filepath.Rel(share.DirectoryPath, path)
		if errorValue != nil {
			return errorValue
		}
		held[filepath.ToSlash(relative)] = true
		return nil
	})
	return held, errorValue
}

func (share Share) pathOf(path string) string {
	return filepath.Join(share.DirectoryPath, filepath.FromSlash(path))
}

func (share Share) read(storePath string) ([]byte, bool) {
	if share.DirectoryPath == "" {
		return nil, false
	}
	_, path, isUnderACompany := strings.Cut(storePath, "/")
	if !isUnderACompany {
		return nil, false
	}
	held, errorValue := os.ReadFile(share.pathOf(path))
	return held, errorValue == nil
}
