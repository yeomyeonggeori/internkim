package walstream

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jackc/pglogrepl"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/hostbackup"
	"github.com/yeomyeonggeori/internkim/internal/offsite"
)

const (
	walDirectoryName       = "pg_wal"
	segmentsPerLogicalFile = 0x100000000 / walSegmentBytes
	dataDirectoryMode      = 0o700
	dataFileMode           = 0o600
	recoverySignalName     = "recovery.signal"
	automaticSettingsName  = "postgresql.auto.conf"
	recoverySettings       = "restore_command = 'false'\nrecovery_target_timeline = 'latest'\nsynchronous_standby_names = ''\n"
)

type Rebuilt struct {
	BaseBackup BaseBackup
	ReplaysTo  pglogrepl.LSN
	Pieces     int
}

func RebuildDataDirectory(ctx context.Context, destination offsite.Destination, keyring box.Keyring, cache offsite.ChunkCache, dataDirectoryPath string) (Rebuilt, error) {
	backup, errorValue := NewestBaseBackup(ctx, destination)
	if errorValue != nil {
		return Rebuilt{}, errorValue
	}
	kept := KeptPieces{Destination: destination}
	plan, errorValue := planTheReplay(ctx, kept, backup, keyring)
	if errorValue != nil {
		return Rebuilt{}, errorValue
	}
	if errorValue := requireAnEmptyDirectory(dataDirectoryPath); errorValue != nil {
		return Rebuilt{}, errorValue
	}
	if errorValue := extractTheDataDirectory(ctx, BaseBackupDestination(destination), keyring, cache, backup, dataDirectoryPath); errorValue != nil {
		return Rebuilt{}, errorValue
	}
	walDirectoryPath := filepath.Join(dataDirectoryPath, walDirectoryName)
	if errorValue := os.MkdirAll(walDirectoryPath, dataDirectoryMode); errorValue != nil {
		return Rebuilt{}, errorValue
	}
	for _, name := range plan.Pieces {
		piece, errorValue := kept.Read(ctx, name, keyring)
		if errorValue != nil {
			return Rebuilt{}, errorValue
		}
		if errorValue := writeIntoSegments(walDirectoryPath, piece); errorValue != nil {
			return Rebuilt{}, errorValue
		}
	}
	for _, history := range plan.Histories {
		if errorValue := os.WriteFile(filepath.Join(walDirectoryPath, history.FileName()), history.Content, dataFileMode); errorValue != nil {
			return Rebuilt{}, errorValue
		}
	}
	if errorValue := askForArchiveRecovery(dataDirectoryPath); errorValue != nil {
		return Rebuilt{}, errorValue
	}
	return Rebuilt{BaseBackup: backup, ReplaysTo: plan.replaysTo(), Pieces: len(plan.Pieces)}, nil
}

func askForArchiveRecovery(dataDirectoryPath string) error {
	if errorValue := os.WriteFile(filepath.Join(dataDirectoryPath, recoverySignalName), nil, dataFileMode); errorValue != nil {
		return errorValue
	}
	settings, errorValue := os.OpenFile(filepath.Join(dataDirectoryPath, automaticSettingsName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, dataFileMode)
	if errorValue != nil {
		return errorValue
	}
	if _, errorValue := settings.WriteString(recoverySettings); errorValue != nil {
		settings.Close()
		return errorValue
	}
	return settings.Close()
}

func requireAnEmptyDirectory(directoryPath string) error {
	entries, errorValue := os.ReadDir(directoryPath)
	if errors.Is(errorValue, fs.ErrNotExist) {
		return os.MkdirAll(directoryPath, dataDirectoryMode)
	}
	if errorValue != nil {
		return errorValue
	}
	if len(entries) > 0 {
		return fmt.Errorf("%s holds %d entries; the database is rebuilt only into an empty directory", directoryPath, len(entries))
	}
	return os.Chmod(directoryPath, dataDirectoryMode)
}

func extractTheDataDirectory(ctx context.Context, destination offsite.Destination, keyring box.Keyring, cache offsite.ChunkCache, backup BaseBackup, dataDirectoryPath string) error {
	index := slices.IndexFunc(backup.Members, func(member hostbackup.Member) bool { return member.Name == DataDirectoryArchive })
	if index < 0 {
		return fmt.Errorf("the base backup %s holds no %s", backup.Name, DataDirectoryArchive)
	}
	reading, writing := io.Pipe()
	go func() {
		writing.CloseWithError(cache.WriteMember(ctx, destination, backup.ChunkManifest(), backup.Members[index], keyring, writing))
	}()
	errorValue := extractTar(reading, dataDirectoryPath)
	reading.CloseWithError(errorValue)
	return errorValue
}

func extractTar(source io.Reader, directoryPath string) error {
	archive := tar.NewReader(source)
	for {
		header, errorValue := archive.Next()
		if errors.Is(errorValue, io.EOF) {
			return nil
		}
		if errorValue != nil {
			return fmt.Errorf("reading %s: %w", DataDirectoryArchive, errorValue)
		}
		targetPath, errorValue := pathInside(directoryPath, header.Name)
		if errorValue != nil {
			return errorValue
		}
		if errorValue := extractEntry(archive, header, targetPath); errorValue != nil {
			return fmt.Errorf("writing %s from %s: %w", header.Name, DataDirectoryArchive, errorValue)
		}
	}
}

func pathInside(directoryPath, name string) (string, error) {
	cleaned := filepath.Clean(filepath.FromSlash(name))
	if filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s names %q, which lies outside the data directory", DataDirectoryArchive, name)
	}
	return filepath.Join(directoryPath, cleaned), nil
}

func extractEntry(archive *tar.Reader, header *tar.Header, targetPath string) error {
	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(targetPath, dataDirectoryMode)
	case tar.TypeReg:
		if errorValue := os.MkdirAll(filepath.Dir(targetPath), dataDirectoryMode); errorValue != nil {
			return errorValue
		}
		file, errorValue := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, dataFileMode)
		if errorValue != nil {
			return errorValue
		}
		if _, errorValue := io.Copy(file, archive); errorValue != nil {
			file.Close()
			return errorValue
		}
		return file.Close()
	case tar.TypeSymlink:
		return fmt.Errorf("it is a symbolic link to %s, and a base backup that keeps the data directory alone has none", header.Linkname)
	}
	return fmt.Errorf("it is a tar entry of kind %q, which a data directory does not hold", header.Typeflag)
}

func writeIntoSegments(walDirectoryPath string, piece Piece) error {
	position := piece.Start
	remaining := piece.Bytes
	for len(remaining) > 0 {
		offset := int64(position % walSegmentBytes)
		length := min(int64(len(remaining)), walSegmentBytes-offset)
		if errorValue := writeIntoSegment(filepath.Join(walDirectoryPath, segmentFileName(piece.Timeline, position)), offset, remaining[:length]); errorValue != nil {
			return errorValue
		}
		position += pglogrepl.LSN(length)
		remaining = remaining[length:]
	}
	return nil
}

func writeIntoSegment(segmentPath string, offset int64, contents []byte) error {
	file, errorValue := os.OpenFile(segmentPath, os.O_CREATE|os.O_WRONLY, dataFileMode)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := file.Truncate(walSegmentBytes); errorValue != nil {
		file.Close()
		return errorValue
	}
	if _, errorValue := file.WriteAt(contents, offset); errorValue != nil {
		file.Close()
		return errorValue
	}
	return file.Close()
}

func segmentFileName(timeline int32, position pglogrepl.LSN) string {
	segment := uint64(position) / walSegmentBytes
	return fmt.Sprintf("%08X%08X%08X", uint32(timeline), segment/segmentsPerLogicalFile, segment%segmentsPerLogicalFile)
}
