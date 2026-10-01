package hostbackup

import (
	"archive/tar"
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

const (
	tarBlockSize        = 512
	writeBufferBytes    = 1 << 20
	archiveFileMode     = 0o600
	memberModifiedFloor = time.Second
)

type Writer struct {
	file    *os.File
	members []Member
}

func Create(path string) (*Writer, error) {
	file, errorValue := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, archiveFileMode)
	if errorValue != nil {
		return nil, errorValue
	}
	return &Writer{file: file}, nil
}

func (writer *Writer) AddMember(name string, produce func(io.Writer) error) error {
	if errorValue := validateMemberName(name); errorValue != nil {
		return errorValue
	}
	headerOffset, errorValue := writer.file.Seek(0, io.SeekCurrent)
	if errorValue != nil {
		return errorValue
	}
	if _, errorValue := writer.file.Write(make([]byte, tarBlockSize)); errorValue != nil {
		return errorValue
	}
	member, errorValue := writer.streamMember(name, produce)
	if errorValue != nil {
		return fmt.Errorf("writing %s: %w", name, errorValue)
	}
	header, errorValue := memberHeaderBlock(name, member.Size)
	if errorValue != nil {
		return errorValue
	}
	if _, errorValue := writer.file.WriteAt(header, headerOffset); errorValue != nil {
		return errorValue
	}
	writer.members = append(writer.members, member)
	return nil
}

func (writer *Writer) streamMember(name string, produce func(io.Writer) error) (Member, error) {
	buffered := bufio.NewWriterSize(writer.file, writeBufferBytes)
	hasher := sha256.New()
	counter := &countingWriter{}
	if errorValue := produce(io.MultiWriter(buffered, hasher, counter)); errorValue != nil {
		return Member{}, errorValue
	}
	if _, errorValue := buffered.Write(make([]byte, paddingAfter(counter.size))); errorValue != nil {
		return Member{}, errorValue
	}
	if errorValue := buffered.Flush(); errorValue != nil {
		return Member{}, errorValue
	}
	return Member{Name: name, Size: counter.size, SHA256: hex.EncodeToString(hasher.Sum(nil))}, nil
}

func (writer *Writer) Finish(manifest Manifest) (Manifest, error) {
	manifest.Format = FormatName
	manifest.FormatVersion = FormatVersion
	manifest.Members = writer.members
	document, errorValue := json.MarshalIndent(manifest, "", "  ")
	if errorValue != nil {
		return Manifest{}, errorValue
	}
	tarWriter := tar.NewWriter(writer.file)
	header := memberHeader(ManifestName, int64(len(document)))
	if errorValue := tarWriter.WriteHeader(header); errorValue != nil {
		return Manifest{}, errorValue
	}
	if _, errorValue := tarWriter.Write(document); errorValue != nil {
		return Manifest{}, errorValue
	}
	if errorValue := tarWriter.Close(); errorValue != nil {
		return Manifest{}, errorValue
	}
	if errorValue := writer.file.Sync(); errorValue != nil {
		return Manifest{}, errorValue
	}
	return manifest, writer.file.Close()
}

func (writer *Writer) Abandon() {
	writer.file.Close()
	os.Remove(writer.file.Name())
}

func memberHeader(name string, size int64) *tar.Header {
	return &tar.Header{
		Typeflag: tar.TypeReg,
		Name:     name,
		Size:     size,
		Mode:     archiveFileMode,
		ModTime:  time.Now().Truncate(memberModifiedFloor),
		Uname:    "root",
		Gname:    "root",
		Format:   tar.FormatGNU,
	}
}

func memberHeaderBlock(name string, size int64) ([]byte, error) {
	var block bytes.Buffer
	if errorValue := tar.NewWriter(&block).WriteHeader(memberHeader(name, size)); errorValue != nil {
		return nil, errorValue
	}
	if block.Len() != tarBlockSize {
		return nil, fmt.Errorf("the header of %s took %d bytes, not one %d-byte block", name, block.Len(), tarBlockSize)
	}
	return block.Bytes(), nil
}

func paddingAfter(size int64) int64 {
	remainder := size % tarBlockSize
	if remainder == 0 {
		return 0
	}
	return tarBlockSize - remainder
}

type countingWriter struct {
	size int64
}

func (counter *countingWriter) Write(document []byte) (int, error) {
	counter.size += int64(len(document))
	return len(document), nil
}
