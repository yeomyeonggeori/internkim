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

func (cache ChunkCache) KeepUp(ctx context.Context, destination Destination) (Manifest, int, error) {
	manifest, errorValue := Newest(ctx, destination)
	if errorValue != nil {
		return Manifest{}, 0, errorValue
	}
	if errorValue := os.MkdirAll(cache.DirectoryPath, 0o700); errorValue != nil {
		return Manifest{}, 0, errorValue
	}
	fetched := 0
	for _, chunkID := range manifest.chunkIDs() {
		if cache.holds(chunkID) {
			continue
		}
		if errorValue := cache.fetch(ctx, destination, chunkID); errorValue != nil {
			return Manifest{}, fetched, errorValue
		}
		fetched++
	}
	return manifest, fetched, cache.forgetAllBut(manifest)
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

func (cache ChunkCache) forgetAllBut(manifest Manifest) error {
	wanted := map[string]bool{}
	for _, chunkID := range manifest.chunkIDs() {
		wanted[chunkID] = true
	}
	entries, errorValue := os.ReadDir(cache.DirectoryPath)
	if errorValue != nil {
		return errorValue
	}
	for _, entry := range entries {
		if wanted[entry.Name()] {
			continue
		}
		if errorValue := os.Remove(filepath.Join(cache.DirectoryPath, entry.Name())); errorValue != nil && !errors.Is(errorValue, fs.ErrNotExist) {
			return errorValue
		}
	}
	return nil
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
			return cache.writeMember(ctx, destination, manifest, member, keyring, output)
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

func (cache ChunkCache) writeMember(ctx context.Context, destination Destination, manifest Manifest, member hostbackup.Member, keyring box.Keyring, output io.Writer) error {
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
