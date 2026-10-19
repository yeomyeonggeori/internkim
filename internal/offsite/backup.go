package offsite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
)

const (
	manifestVersion  = 1
	manifestFileName = "manifest.json"
	partBytes        = 32 * 1024 * 1024
)

var ErrNoBackup = errors.New("the company keeps no backup this box can open")

type Destination struct {
	Store      Store
	CompanyID  string
	SealingKey string
}

type Manifest struct {
	Version     int       `json:"version"`
	ArchiveName string    `json:"archiveName"`
	SealingKey  string    `json:"sealingKey"`
	CreatedAt   time.Time `json:"createdAt"`
	Parts       []Part    `json:"parts"`
}

type Part struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

func (destination Destination) folderOf(archiveName string) string {
	return destination.CompanyID + "/" + archiveName
}

func Ship(ctx context.Context, archivePath string, destination Destination, now time.Time) (Manifest, error) {
	archive, errorValue := os.Open(archivePath)
	if errorValue != nil {
		return Manifest{}, errorValue
	}
	defer archive.Close()
	archiveName := filepath.Base(archivePath)
	uploader := &partUploader{ctx: ctx, destination: destination, folder: destination.folderOf(archiveName)}
	sealed, errorValue := box.SealStreamTo(uploader, destination.SealingKey, box.BackupPurpose(destination.CompanyID, archiveName))
	if errorValue != nil {
		return Manifest{}, errorValue
	}
	if _, errorValue := io.Copy(sealed, archive); errorValue != nil {
		return Manifest{}, fmt.Errorf("sealing %s for the company's boxes: %w", archiveName, errorValue)
	}
	if errorValue := sealed.Close(); errorValue != nil {
		return Manifest{}, errorValue
	}
	if errorValue := uploader.flush(); errorValue != nil {
		return Manifest{}, errorValue
	}
	manifest := Manifest{
		Version:     manifestVersion,
		ArchiveName: archiveName,
		SealingKey:  destination.SealingKey,
		CreatedAt:   now.UTC(),
		Parts:       uploader.parts,
	}
	document, errorValue := json.Marshal(manifest)
	if errorValue != nil {
		return Manifest{}, errorValue
	}
	return manifest, destination.Store.Put(ctx, uploader.folder+"/"+manifestFileName, document)
}

type partUploader struct {
	ctx         context.Context
	destination Destination
	folder      string
	pending     []byte
	parts       []Part
}

func (uploader *partUploader) Write(bytes []byte) (int, error) {
	uploader.pending = append(uploader.pending, bytes...)
	for len(uploader.pending) >= partBytes {
		if errorValue := uploader.upload(uploader.pending[:partBytes]); errorValue != nil {
			return 0, errorValue
		}
		uploader.pending = append([]byte{}, uploader.pending[partBytes:]...)
	}
	return len(bytes), nil
}

func (uploader *partUploader) flush() error {
	if len(uploader.pending) == 0 {
		return nil
	}
	errorValue := uploader.upload(uploader.pending)
	uploader.pending = nil
	return errorValue
}

func (uploader *partUploader) upload(part []byte) error {
	name := fmt.Sprintf("part-%06d", len(uploader.parts)+1)
	digest := sha256.Sum256(part)
	if errorValue := uploader.destination.Store.Put(uploader.ctx, uploader.folder+"/"+name, part); errorValue != nil {
		return errorValue
	}
	uploader.parts = append(uploader.parts, Part{Name: name, Size: int64(len(part)), SHA256: hex.EncodeToString(digest[:])})
	return nil
}

func archiveNamesOf(ctx context.Context, store Store, companyID string) ([]string, error) {
	listed, errorValue := store.List(ctx, companyID+"/")
	if errorValue != nil {
		return nil, errorValue
	}
	names := []string{}
	for _, object := range listed {
		if object.Name != "" {
			names = append(names, object.Name)
		}
	}
	slices.Sort(names)
	return names, nil
}

func Prune(ctx context.Context, destination Destination, keep int) ([]string, error) {
	names, errorValue := archiveNamesOf(ctx, destination.Store, destination.CompanyID)
	if errorValue != nil || keep <= 0 || len(names) <= keep {
		return nil, errorValue
	}
	pruned := names[:len(names)-keep]
	for _, archiveName := range pruned {
		if errorValue := removeFolder(ctx, destination, archiveName); errorValue != nil {
			return nil, errorValue
		}
	}
	return pruned, nil
}

func removeFolder(ctx context.Context, destination Destination, archiveName string) error {
	folder := destination.folderOf(archiveName)
	listed, errorValue := destination.Store.List(ctx, folder+"/")
	if errorValue != nil {
		return errorValue
	}
	paths := make([]string, 0, len(listed))
	for _, object := range listed {
		paths = append(paths, folder+"/"+object.Name)
	}
	return destination.Store.Remove(ctx, paths)
}

func Newest(ctx context.Context, destination Destination) (Manifest, error) {
	names, errorValue := archiveNamesOf(ctx, destination.Store, destination.CompanyID)
	if errorValue != nil {
		return Manifest{}, errorValue
	}
	for index := len(names) - 1; index >= 0; index-- {
		manifest, isComplete := completeManifest(ctx, destination, names[index])
		if isComplete && manifest.SealingKey == destination.SealingKey {
			return manifest, nil
		}
	}
	return Manifest{}, ErrNoBackup
}

func completeManifest(ctx context.Context, destination Destination, archiveName string) (Manifest, bool) {
	document, errorValue := destination.Store.Get(ctx, destination.folderOf(archiveName)+"/"+manifestFileName)
	if errorValue != nil {
		return Manifest{}, false
	}
	var manifest Manifest
	if json.Unmarshal(document, &manifest) != nil || manifest.Version != manifestVersion || manifest.ArchiveName != archiveName {
		return Manifest{}, false
	}
	return manifest, true
}

func Fetch(ctx context.Context, destination Destination, manifest Manifest, keyring box.Keyring, archivePath string) error {
	reading, writing := io.Pipe()
	go func() { writing.CloseWithError(copyParts(ctx, destination, manifest, writing)) }()
	opened, errorValue := keyring.OpenStream(reading, box.BackupPurpose(destination.CompanyID, manifest.ArchiveName))
	if errorValue != nil {
		reading.CloseWithError(errorValue)
		return errorValue
	}
	return writeArchive(archivePath, opened, reading)
}

func copyParts(ctx context.Context, destination Destination, manifest Manifest, writing io.Writer) error {
	folder := destination.folderOf(manifest.ArchiveName)
	for _, part := range manifest.Parts {
		bytes, errorValue := destination.Store.Get(ctx, folder+"/"+part.Name)
		if errorValue != nil {
			return errorValue
		}
		digest := sha256.Sum256(bytes)
		if hex.EncodeToString(digest[:]) != part.SHA256 || int64(len(bytes)) != part.Size {
			return fmt.Errorf("%s of %s is not the part that was kept", part.Name, manifest.ArchiveName)
		}
		if _, errorValue := writing.Write(bytes); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func writeArchive(archivePath string, opened io.Reader, reading *io.PipeReader) error {
	partialPath := archivePath + ".partial"
	file, errorValue := os.OpenFile(partialPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if errorValue != nil {
		reading.CloseWithError(errorValue)
		return errorValue
	}
	_, copyError := io.Copy(file, opened)
	closeError := file.Close()
	reading.Close()
	if errorValue := errors.Join(copyError, closeError); errorValue != nil {
		os.Remove(partialPath)
		return fmt.Errorf("opening the company's backup: %w", errorValue)
	}
	return os.Rename(partialPath, archivePath)
}
