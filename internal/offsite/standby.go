package offsite

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/durablefile"
	"github.com/yeomyeonggeori/internkim/internal/hostbackup"
)

type ChunkCache struct {
	DirectoryPath string
}

func (cache ChunkCache) pathOf(chunkID string) string {
	return filepath.Join(cache.DirectoryPath, chunkID)
}

func (cache ChunkCache) holds(chunkID string) bool {
	_, errorValue := os.Stat(cache.pathOf(chunkID))
	return errorValue == nil
}

func (cache ChunkCache) fetch(ctx context.Context, destination Destination, chunkID string) error {
	sealed, errorValue := destination.Store.Get(ctx, destination.chunkPath(chunkID))
	if errorValue != nil {
		return errorValue
	}
	return durablefile.Write(cache.pathOf(chunkID), sealed, 0o600)
}

func (cache ChunkCache) FetchMissing(ctx context.Context, destination Destination, chunkIDs []string) error {
	if errorValue := os.MkdirAll(cache.DirectoryPath, 0o700); errorValue != nil {
		return errorValue
	}
	missing := []string{}
	isListed := map[string]bool{}
	for _, chunkID := range chunkIDs {
		if isListed[chunkID] || cache.holds(chunkID) {
			continue
		}
		isListed[chunkID] = true
		missing = append(missing, chunkID)
	}
	fetch := func(fetching context.Context, chunkID string) (string, error) {
		return chunkID, cache.fetch(fetching, destination, chunkID)
	}
	return FetchAhead(ctx, missing, fetch, func(string) error { return nil })
}

func (cache ChunkCache) Materialize(ctx context.Context, destination Destination, manifest Manifest, keyring box.Keyring, archivePath string) error {
	partialPath := archivePath + ".partial"
	os.Remove(partialPath)
	writer, errorValue := hostbackup.Create(partialPath)
	if errorValue != nil {
		return errorValue
	}
	for _, member := range manifest.Archive.Members {
		if errorValue := writer.AddMember(member.Name, func(output io.Writer) error {
			return cache.WriteMember(ctx, destination, manifest, member, keyring, output)
		}); errorValue != nil {
			writer.Abandon()
			return errorValue
		}
	}
	archive := manifest.Archive
	archive.Members = nil
	if _, errorValue := writer.Finish(archive); errorValue != nil {
		writer.Abandon()
		return errorValue
	}
	return os.Rename(partialPath, archivePath)
}

func (cache ChunkCache) WriteMember(ctx context.Context, destination Destination, manifest Manifest, member hostbackup.Member, keyring box.Keyring, output io.Writer) error {
	hasher := sha256.New()
	counter := &byteCounter{}
	checked := io.MultiWriter(output, hasher, counter)
	for _, chunkID := range manifest.Chunks[member.Name] {
		if errorValue := cache.writeChunk(ctx, destination, chunkID, keyring, checked); errorValue != nil {
			return fmt.Errorf("%s of the snapshot %s: %w", member.Name, manifest.Name, errorValue)
		}
	}
	if counter.size != member.Size || hex.EncodeToString(hasher.Sum(nil)) != member.SHA256 {
		return fmt.Errorf("%s of the snapshot %s is not what the service box kept", member.Name, manifest.Name)
	}
	return nil
}

func (cache ChunkCache) writeChunk(ctx context.Context, destination Destination, chunkID string, keyring box.Keyring, output io.Writer) error {
	opened, errorValue := cache.openedChunk(ctx, destination, chunkID, keyring)
	if errorValue != nil {
		return errorValue
	}
	_, errorValue = output.Write(opened)
	return errorValue
}

func (cache ChunkCache) openedChunk(ctx context.Context, destination Destination, chunkID string, keyring box.Keyring) ([]byte, error) {
	if cache.holds(chunkID) {
		opened, errorValue := cache.open(destination, chunkID, keyring)
		if errorValue == nil || errors.Is(errorValue, box.ErrCompanyKeyNotHeld) {
			return opened, errorValue
		}
		if errorValue := os.Remove(cache.pathOf(chunkID)); errorValue != nil && !errors.Is(errorValue, fs.ErrNotExist) {
			return nil, fmt.Errorf("removing the cached chunk %s, which does not open, to fetch it again: %w", chunkID, errorValue)
		}
	}
	if errorValue := os.MkdirAll(cache.DirectoryPath, 0o700); errorValue != nil {
		return nil, errorValue
	}
	if errorValue := cache.fetch(ctx, destination, chunkID); errorValue != nil {
		return nil, fmt.Errorf("fetching the chunk %s: %w", chunkID, errorValue)
	}
	opened, errorValue := cache.open(destination, chunkID, keyring)
	if errors.Is(errorValue, box.ErrCompanyKeyNotHeld) {
		return nil, errorValue
	}
	if errorValue != nil {
		return nil, fmt.Errorf("the chunk %s does not open as the plane holds it, so the plane's copy is bad: %w", chunkID, errorValue)
	}
	return opened, nil
}

func (cache ChunkCache) open(destination Destination, chunkID string, keyring box.Keyring) ([]byte, error) {
	sealed, errorValue := os.ReadFile(cache.pathOf(chunkID))
	if errorValue != nil {
		return nil, errorValue
	}
	opened, errorValue := keyring.OpenStream(bytes.NewReader(sealed), box.BackupChunkPurpose(destination.CompanyID, chunkID))
	if errorValue != nil {
		return nil, errorValue
	}
	return io.ReadAll(opened)
}
