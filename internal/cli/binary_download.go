package cli

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func extractFromZip(reader io.ReaderAt, size int64, localPath, entryName string) error {
	zipReader, err := zip.NewReader(reader, size)
	if err != nil {
		return fmt.Errorf("zip open: %w", err)
	}
	for _, file := range zipReader.File {
		if filepath.Base(file.Name) != entryName {
			continue
		}
		source, err := file.Open()
		if err != nil {
			return err
		}
		defer source.Close()
		target, err := os.OpenFile(localPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return err
		}
		defer target.Close()
		_, err = io.Copy(target, source)
		return err
	}
	return fmt.Errorf("entry %q not found in archive", entryName)
}
