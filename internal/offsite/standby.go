package offsite

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/yeomyeonggeori/internkim/internal/box"
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
	partialPath := cache.pathOf(chunkID) + ".partial"
	if errorValue := os.WriteFile(partialPath, sealed, 0o600); errorValue != nil {
		return errorValue
	}
	return os.Rename(partialPath, cache.pathOf(chunkID))
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
	if !cache.holds(chunkID) {
		if errorValue := os.MkdirAll(cache.DirectoryPath, 0o700); errorValue != nil {
			return errorValue
		}
		if errorValue := cache.fetch(ctx, destination, chunkID); errorValue != nil {
			return errorValue
		}
	}
	sealed, errorValue := os.ReadFile(cache.pathOf(chunkID))
	if errorValue != nil {
		return errorValue
	}
	opened, errorValue := keyring.OpenStream(bytes.NewReader(sealed), box.BackupChunkPurpose(destination.CompanyID, chunkID))
	if errorValue != nil {
		return errorValue
	}
	_, errorValue = io.Copy(output, opened)
	return errorValue
}
