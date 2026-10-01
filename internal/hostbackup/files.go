package hostbackup

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type FileRoot struct {
	Role     string
	Path     string
	Excluded []string
}

type FilesReport struct {
	Files    int
	Bytes    int64
	Warnings []string
}

type fileTreeWriter struct {
	tarWriter  *tar.Writer
	snapshots  Snapshotter
	report     FilesReport
	companions map[string]bool
}

func WriteFiles(output io.Writer, roots []FileRoot, snapshots Snapshotter) (FilesReport, error) {
	writer := &fileTreeWriter{tarWriter: tar.NewWriter(output), snapshots: snapshots, companions: map[string]bool{}}
	for _, root := range roots {
		if errorValue := writer.writeRoot(root); errorValue != nil {
			return FilesReport{}, errorValue
		}
	}
	return writer.report, writer.tarWriter.Close()
}

func (writer *fileTreeWriter) writeRoot(root FileRoot) error {
	resolved, errorValue := filepath.EvalSymlinks(root.Path)
	if errorValue != nil {
		return errorValue
	}
	root.Path = resolved
	return filepath.WalkDir(root.Path, func(filePath string, entry fs.DirEntry, walkError error) error {
		if errors.Is(walkError, fs.ErrNotExist) {
			writer.warn("%s disappeared while it was being read", filePath)
			return nil
		}
		if walkError != nil {
			return walkError
		}
		relative, errorValue := filepath.Rel(root.Path, filePath)
		if errorValue != nil {
			return errorValue
		}
		relative = filepath.ToSlash(relative)
		if isExcluded(relative, root.Excluded) {
			return skipped(entry)
		}
		return writer.writeEntry(filePath, path.Join(root.Role, relative))
	})
}

func isExcluded(relative string, patterns []string) bool {
	for _, pattern := range patterns {
		if isMatched, _ := path.Match(pattern, relative); isMatched {
			return true
		}
	}
	return false
}

func skipped(entry fs.DirEntry) error {
	if entry.IsDir() {
		return filepath.SkipDir
	}
	return nil
}

func (writer *fileTreeWriter) writeEntry(filePath string, archivedName string) error {
	if writer.companions[filePath] {
		return nil
	}
	information, errorValue := os.Lstat(filePath)
	if errors.Is(errorValue, fs.ErrNotExist) {
		writer.warn("%s disappeared while it was being read", filePath)
		return nil
	}
	if errorValue != nil {
		return errorValue
	}
	switch {
	case information.IsDir():
		return writer.writeHeader(information, archivedName+"/", "")
	case information.Mode()&fs.ModeSymlink != 0:
		target, errorValue := os.Readlink(filePath)
		if errorValue != nil {
			return errorValue
		}
		return writer.writeHeader(information, archivedName, target)
	case information.Mode().IsRegular():
		return writer.writeRegularFile(filePath, archivedName, information)
	}
	writer.warn("%s is a %s and was left out", filePath, information.Mode().Type())
	return nil
}

func (writer *fileTreeWriter) writeHeader(information fs.FileInfo, archivedName string, linkTarget string) error {
	header, errorValue := tar.FileInfoHeader(information, linkTarget)
	if errorValue != nil {
		return errorValue
	}
	header.Name = archivedName
	return writer.tarWriter.WriteHeader(header)
}

func (writer *fileTreeWriter) writeRegularFile(filePath string, archivedName string, information fs.FileInfo) error {
	isDatabase, errorValue := isSQLiteDatabase(filePath)
	if errorValue != nil {
		return writer.vanishedOr(filePath, errorValue)
	}
	if isDatabase {
		return writer.writeSQLiteDatabase(filePath, archivedName, information)
	}
	return writer.copyFile(filePath, filePath, archivedName, information)
}

func (writer *fileTreeWriter) writeSQLiteDatabase(filePath string, archivedName string, information fs.FileInfo) error {
	for _, suffix := range sqliteCompanionSuffixes {
		writer.companions[filePath+suffix] = true
	}
	snapshotPath, errorValue := writer.snapshots.Snapshot(filePath)
	if errorValue != nil {
		writer.warn("%s was copied as it lay, because SQLite would not take a consistent copy of it: %v", filePath, errorValue)
		writer.unmarkCompanions(filePath)
		return writer.copyFile(filePath, filePath, archivedName, information)
	}
	defer os.Remove(snapshotPath)
	snapshot, errorValue := os.Stat(snapshotPath)
	if errorValue != nil {
		return errorValue
	}
	return writer.copyFile(snapshotPath, filePath, archivedName, sizedAs{FileInfo: information, size: snapshot.Size()})
}

func (writer *fileTreeWriter) unmarkCompanions(filePath string) {
	for _, suffix := range sqliteCompanionSuffixes {
		delete(writer.companions, filePath+suffix)
	}
}

func (writer *fileTreeWriter) copyFile(sourcePath string, originalPath string, archivedName string, information fs.FileInfo) error {
	file, errorValue := os.Open(sourcePath)
	if errorValue != nil {
		return writer.vanishedOr(originalPath, errorValue)
	}
	defer file.Close()
	if errorValue := writer.writeHeader(information, archivedName, ""); errorValue != nil {
		return errorValue
	}
	copied, errorValue := io.CopyN(writer.tarWriter, file, information.Size())
	if errors.Is(errorValue, io.EOF) {
		writer.warn("%s shrank while it was being read; the rest of it is zeros in the backup", originalPath)
		_, errorValue = io.CopyN(writer.tarWriter, zeros{}, information.Size()-copied)
	}
	if errorValue != nil {
		return fmt.Errorf("reading %s: %w", originalPath, errorValue)
	}
	writer.report.Files++
	writer.report.Bytes += information.Size()
	return nil
}

func (writer *fileTreeWriter) vanishedOr(filePath string, errorValue error) error {
	if errors.Is(errorValue, fs.ErrNotExist) {
		writer.warn("%s disappeared while it was being read", filePath)
		return nil
	}
	return errorValue
}

func (writer *fileTreeWriter) warn(format string, arguments ...any) {
	writer.report.Warnings = append(writer.report.Warnings, fmt.Sprintf(format, arguments...))
}

type sizedAs struct {
	fs.FileInfo
	size int64
}

func (information sizedAs) Size() int64 {
	return information.size
}

type zeros struct{}

func (zeros) Read(buffer []byte) (int, error) {
	clear(buffer)
	return len(buffer), nil
}

var sqliteHeader = []byte("SQLite format 3\x00")

var sqliteCompanionSuffixes = []string{"-wal", "-shm", "-journal"}

func isSQLiteDatabase(filePath string) (bool, error) {
	file, errorValue := os.Open(filePath)
	if errorValue != nil {
		return false, errorValue
	}
	defer file.Close()
	header := make([]byte, len(sqliteHeader))
	if _, errorValue := io.ReadFull(file, header); errorValue != nil {
		return false, nil
	}
	return bytes.Equal(header, sqliteHeader), nil
}

func splitArchivedName(name string) (string, string, error) {
	cleaned := path.Clean(strings.TrimSuffix(name, "/"))
	if path.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", "", fmt.Errorf("%s names a place outside the backup's roots", name)
	}
	role, relative, _ := strings.Cut(cleaned, "/")
	if relative == "" {
		relative = "."
	}
	return role, relative, nil
}
