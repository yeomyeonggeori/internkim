package cli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// downloadBinary downloads a binary (or extracts one from a tar.gz) to localPath.
// tarEntry is the filename inside the archive to extract; empty means direct binary download.
func downloadBinary(url, localPath, tarEntry string) error {
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	if tarEntry == "" {
		// Direct binary download
		f, err := os.OpenFile(localPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(f, resp.Body)
		return err
	}

	if strings.HasSuffix(strings.ToLower(url), ".zip") {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return extractFromZip(bytes.NewReader(body), int64(len(body)), localPath, tarEntry)
	}

	// Extract specific file from tar.gz
	return extractFromTarGz(resp.Body, localPath, tarEntry)
}

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

func extractFromTarGz(r io.Reader, localPath, entryName string) error {
	gzReader, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip open: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}
		if filepath.Base(header.Name) == entryName {
			f, err := os.OpenFile(localPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(f, tarReader)
			return err
		}
	}
	return fmt.Errorf("entry %q not found in archive", entryName)
}
