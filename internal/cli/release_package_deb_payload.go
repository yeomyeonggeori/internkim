package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blakesmith/ar"
)

const (
	uncompressedDebianPayloadName = "data.tar"
	compressedDebianPayloadName   = "data.tar.xz"
)

var debianPayloadCompressor = []string{"xz", "-9", "--threads=0", "--stdout"}

func compressDebianPayload(packagePath string) error {
	if _, errorValue := exec.LookPath(debianPayloadCompressor[0]); errorValue != nil {
		return fmt.Errorf("the deb's payload is compressed by %s, which is not installed here (brew install xz)", debianPayloadCompressor[0])
	}
	rewrittenPath := packagePath + ".rewriting"
	if errorValue := rewriteDebianPackage(packagePath, rewrittenPath); errorValue != nil {
		os.Remove(rewrittenPath)
		return errorValue
	}
	return os.Rename(rewrittenPath, packagePath)
}

func rewriteDebianPackage(packagePath string, rewrittenPath string) error {
	source, errorValue := os.Open(packagePath)
	if errorValue != nil {
		return errorValue
	}
	defer source.Close()
	destination, errorValue := os.Create(rewrittenPath)
	if errorValue != nil {
		return errorValue
	}
	defer destination.Close()
	reader := ar.NewReader(source)
	writer := archiveWriter{destination: destination, headers: ar.NewWriter(destination)}
	if errorValue := writer.headers.WriteGlobalHeader(); errorValue != nil {
		return errorValue
	}
	hasPayload := false
	for {
		header, errorValue := reader.Next()
		if errors.Is(errorValue, io.EOF) {
			break
		}
		if errorValue != nil {
			return fmt.Errorf("read %s: %w", packagePath, errorValue)
		}
		if strings.TrimRight(header.Name, "/") != uncompressedDebianPayloadName {
			if errorValue := copyArchiveMember(writer, header, reader); errorValue != nil {
				return errorValue
			}
			continue
		}
		if errorValue := writeCompressedPayload(writer, header, reader, filepath.Dir(rewrittenPath)); errorValue != nil {
			return errorValue
		}
		hasPayload = true
	}
	if !hasPayload {
		return fmt.Errorf("%s has no %s member to compress", packagePath, uncompressedDebianPayloadName)
	}
	return destination.Close()
}

type archiveWriter struct {
	destination io.Writer
	headers     *ar.Writer
}

func copyArchiveMember(writer archiveWriter, header *ar.Header, body io.Reader) error {
	if errorValue := writer.headers.WriteHeader(header); errorValue != nil {
		return errorValue
	}
	written, errorValue := io.Copy(writer.destination, body)
	if errorValue != nil {
		return errorValue
	}
	if written != header.Size {
		return fmt.Errorf("the deb's %s member is %d bytes and its header says %d", header.Name, written, header.Size)
	}
	if written%2 == 0 {
		return nil
	}
	_, errorValue = writer.destination.Write([]byte{'\n'})
	return errorValue
}

func writeCompressedPayload(writer archiveWriter, header *ar.Header, payload io.Reader, scratchDirectory string) error {
	compressed, errorValue := os.CreateTemp(scratchDirectory, "payload-*.xz")
	if errorValue != nil {
		return errorValue
	}
	defer os.Remove(compressed.Name())
	defer compressed.Close()
	command := exec.Command(debianPayloadCompressor[0], debianPayloadCompressor[1:]...)
	command.Stdin = payload
	command.Stdout = compressed
	commandError := &strings.Builder{}
	command.Stderr = commandError
	if errorValue := command.Run(); errorValue != nil {
		return fmt.Errorf("compress the deb's payload with %s: %v: %s", strings.Join(debianPayloadCompressor, " "), errorValue, strings.TrimSpace(commandError.String()))
	}
	information, errorValue := compressed.Stat()
	if errorValue != nil {
		return errorValue
	}
	if _, errorValue := compressed.Seek(0, io.SeekStart); errorValue != nil {
		return errorValue
	}
	renamed := *header
	renamed.Name = compressedDebianPayloadName
	renamed.Size = information.Size()
	return copyArchiveMember(writer, &renamed, compressed)
}
