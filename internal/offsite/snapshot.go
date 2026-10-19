package offsite

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/hostbackup"
)

const (
	manifestVersion     = 2
	snapshotFolder      = "snapshots"
	chunkFolder         = "chunks"
	snapshotNamePrefix  = "internkim-snapshot-"
	snapshotNameSuffix  = ".json"
	snapshotTimeLayout  = "20060102T150405Z"
	unreferencedGrace   = time.Hour
	defaultSnapshotKept = 8
)

var ErrNoBackup = errors.New("the company keeps no snapshot this box can open")

type Destination struct {
	Store      Store
	CompanyID  string
	SealingKey string
}

type Manifest struct {
	Version    int                 `json:"version"`
	Name       string              `json:"name"`
	SealingKey string              `json:"sealingKey"`
	CreatedAt  time.Time           `json:"createdAt"`
	Archive    hostbackup.Manifest `json:"archive"`
	Chunks     map[string][]string `json:"chunks"`
}

func (manifest Manifest) chunkIDs() []string {
	identifiers := []string{}
	for _, member := range manifest.Archive.Members {
		identifiers = append(identifiers, manifest.Chunks[member.Name]...)
	}
	return identifiers
}

func (destination Destination) snapshotPath(name string) string {
	return destination.CompanyID + "/" + snapshotFolder + "/" + name
}

func (destination Destination) chunkPath(chunkID string) string {
	return destination.CompanyID + "/" + chunkFolder + "/" + chunkID
}

func snapshotNameAt(now time.Time) string {
	return snapshotNamePrefix + now.UTC().Format(snapshotTimeLayout) + snapshotNameSuffix
}

type Snapshot struct {
	ctx         context.Context
	destination Destination
	identityKey []byte
	kept        map[string]bool
	members     []hostbackup.Member
	chunks      map[string][]string
	uploaded    int
}

func StartSnapshot(ctx context.Context, destination Destination, keyring box.Keyring) (*Snapshot, error) {
	identityKey, errorValue := keyring.ChunkIdentityKey()
	if errorValue != nil {
		return nil, errorValue
	}
	listed, errorValue := destination.Store.List(ctx, destination.CompanyID+"/"+chunkFolder+"/")
	if errorValue != nil {
		return nil, errorValue
	}
	kept := map[string]bool{}
	for _, object := range listed {
		kept[object.Name] = true
	}
	return &Snapshot{ctx: ctx, destination: destination, identityKey: identityKey, kept: kept, chunks: map[string][]string{}}, nil
}

func (snapshot *Snapshot) AddMember(name string, produce func(io.Writer) error) error {
	hasher := sha256.New()
	counter := &byteCounter{}
	identifiers := []string{}
	splitter := newChunker(func(chunk []byte) error {
		identifier, errorValue := snapshot.keepChunk(chunk)
		identifiers = append(identifiers, identifier)
		return errorValue
	})
	if errorValue := produce(io.MultiWriter(splitter, hasher, counter)); errorValue != nil {
		return fmt.Errorf("snapshotting %s: %w", name, errorValue)
	}
	if errorValue := splitter.Close(); errorValue != nil {
		return fmt.Errorf("snapshotting %s: %w", name, errorValue)
	}
	snapshot.members = append(snapshot.members, hostbackup.Member{Name: name, Size: counter.size, SHA256: hex.EncodeToString(hasher.Sum(nil))})
	snapshot.chunks[name] = identifiers
	return nil
}

func (snapshot *Snapshot) keepChunk(chunk []byte) (string, error) {
	mac := hmac.New(sha256.New, snapshot.identityKey)
	mac.Write(chunk)
	identifier := hex.EncodeToString(mac.Sum(nil))
	if snapshot.kept[identifier] {
		return identifier, nil
	}
	var sealed bytes.Buffer
	sealing, errorValue := box.SealStreamTo(&sealed, snapshot.destination.SealingKey, box.BackupChunkPurpose(snapshot.destination.CompanyID, identifier))
	if errorValue != nil {
		return "", errorValue
	}
	if _, errorValue := sealing.Write(chunk); errorValue != nil {
		return "", errorValue
	}
	if errorValue := sealing.Close(); errorValue != nil {
		return "", errorValue
	}
	if errorValue := snapshot.destination.Store.Put(snapshot.ctx, snapshot.destination.chunkPath(identifier), sealed.Bytes()); errorValue != nil {
		return "", errorValue
	}
	snapshot.kept[identifier] = true
	snapshot.uploaded++
	return identifier, nil
}

func (snapshot *Snapshot) Finish(archive hostbackup.Manifest, now time.Time) (Manifest, error) {
	archive.Format = hostbackup.FormatName
	archive.FormatVersion = hostbackup.FormatVersion
	archive.Members = snapshot.members
	manifest := Manifest{
		Version:    manifestVersion,
		Name:       snapshotNameAt(now),
		SealingKey: snapshot.destination.SealingKey,
		CreatedAt:  now.UTC(),
		Archive:    archive,
		Chunks:     snapshot.chunks,
	}
	document, errorValue := json.Marshal(manifest)
	if errorValue != nil {
		return Manifest{}, errorValue
	}
	return manifest, snapshot.destination.Store.Put(snapshot.ctx, snapshot.destination.snapshotPath(manifest.Name), document)
}

func (snapshot *Snapshot) UploadedChunks() int {
	return snapshot.uploaded
}

type byteCounter struct{ size int64 }

func (counter *byteCounter) Write(bytes []byte) (int, error) {
	counter.size += int64(len(bytes))
	return len(bytes), nil
}

func snapshotNamesOf(ctx context.Context, destination Destination) ([]string, error) {
	listed, errorValue := destination.Store.List(ctx, destination.CompanyID+"/"+snapshotFolder+"/")
	if errorValue != nil {
		return nil, errorValue
	}
	names := []string{}
	for _, object := range listed {
		if strings.HasPrefix(object.Name, snapshotNamePrefix) && strings.HasSuffix(object.Name, snapshotNameSuffix) {
			names = append(names, object.Name)
		}
	}
	slices.Sort(names)
	return names, nil
}

func Newest(ctx context.Context, destination Destination) (Manifest, error) {
	names, errorValue := snapshotNamesOf(ctx, destination)
	if errorValue != nil {
		return Manifest{}, errorValue
	}
	for index := len(names) - 1; index >= 0; index-- {
		manifest, isReadable := readManifest(ctx, destination, names[index])
		if isReadable && manifest.SealingKey == destination.SealingKey {
			return manifest, nil
		}
	}
	return Manifest{}, ErrNoBackup
}

func readManifest(ctx context.Context, destination Destination, name string) (Manifest, bool) {
	document, errorValue := destination.Store.Get(ctx, destination.snapshotPath(name))
	if errorValue != nil {
		return Manifest{}, false
	}
	var manifest Manifest
	if json.Unmarshal(document, &manifest) != nil || manifest.Version != manifestVersion || manifest.Name != name {
		return Manifest{}, false
	}
	return manifest, true
}

func Prune(ctx context.Context, destination Destination, keep int, now time.Time) ([]string, error) {
	if keep <= 0 {
		keep = defaultSnapshotKept
	}
	names, errorValue := snapshotNamesOf(ctx, destination)
	if errorValue != nil {
		return nil, errorValue
	}
	pruned := []string{}
	if len(names) > keep {
		pruned = names[:len(names)-keep]
		paths := make([]string, 0, len(pruned))
		for _, name := range pruned {
			paths = append(paths, destination.snapshotPath(name))
		}
		if errorValue := destination.Store.Remove(ctx, paths); errorValue != nil {
			return nil, errorValue
		}
	}
	return pruned, sweepUnreferencedChunks(ctx, destination, names[len(pruned):], now)
}

func sweepUnreferencedChunks(ctx context.Context, destination Destination, keptNames []string, now time.Time) error {
	referenced := map[string]bool{}
	for _, name := range keptNames {
		manifest, isReadable := readManifest(ctx, destination, name)
		if !isReadable {
			return fmt.Errorf("the kept snapshot %s cannot be read, so no chunk is swept until it is", name)
		}
		for _, identifier := range manifest.chunkIDs() {
			referenced[identifier] = true
		}
	}
	listed, errorValue := destination.Store.List(ctx, destination.CompanyID+"/"+chunkFolder+"/")
	if errorValue != nil {
		return errorValue
	}
	unreferenced := []string{}
	for _, object := range listed {
		if !referenced[object.Name] && object.CreatedAt.Before(now.Add(-unreferencedGrace)) {
			unreferenced = append(unreferenced, destination.chunkPath(object.Name))
		}
	}
	for start := 0; start < len(unreferenced); start += listPageSize {
		if errorValue := destination.Store.Remove(ctx, unreferenced[start:min(start+listPageSize, len(unreferenced))]); errorValue != nil {
			return errorValue
		}
	}
	return nil
}
