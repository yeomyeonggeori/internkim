package aptrepository

import (
	"archive/tar"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
)

const arFileMagic = "!<arch>\n"

// ControlFields are the `key: value` paragraph a .deb carries in its control
// member, in the order the package wrote them.
type ControlFields struct {
	names  []string
	values map[string]string
}

func (fields ControlFields) Value(name string) string {
	return fields.values[name]
}

func (fields ControlFields) Names() []string {
	return append([]string(nil), fields.names...)
}

func (fields *ControlFields) set(name string, value string) {
	if fields.values == nil {
		fields.values = map[string]string{}
	}
	if _, present := fields.values[name]; !present {
		fields.names = append(fields.names, name)
	}
	fields.values[name] = value
}

// ReadControlFields returns the control paragraph of a Debian binary package.
func ReadControlFields(archive []byte) (ControlFields, error) {
	controlArchive, errorValue := arMember(archive, "control.tar")
	if errorValue != nil {
		return ControlFields{}, errorValue
	}
	control, errorValue := tarMember(controlArchive, "control")
	if errorValue != nil {
		return ControlFields{}, errorValue
	}
	return ParseControlParagraph(string(control))
}

// ParseControlParagraph reads one RFC822-shaped paragraph, keeping the
// continuation lines of a folded field such as Description attached to it.
func ParseControlParagraph(text string) (ControlFields, error) {
	fields := ControlFields{}
	currentName := ""
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if (line[0] == ' ' || line[0] == '\t') && currentName != "" {
			fields.set(currentName, fields.Value(currentName)+"\n "+strings.TrimSpace(line))
			continue
		}
		name, value, found := strings.Cut(line, ":")
		if !found {
			return ControlFields{}, fmt.Errorf("control line %q is neither a field nor a continuation", line)
		}
		currentName = strings.TrimSpace(name)
		fields.set(currentName, strings.TrimSpace(value))
	}
	if fields.Value("Package") == "" {
		return ControlFields{}, errors.New("control paragraph names no Package")
	}
	return fields, nil
}

// arMember returns the body of the named `ar` member, accepting the
// compression suffix dpkg appends (control.tar.gz, control.tar.xz, …).
func arMember(archive []byte, wantedName string) ([]byte, error) {
	if !bytes.HasPrefix(archive, []byte(arFileMagic)) {
		return nil, errors.New("not an ar archive, so not a .deb")
	}
	offset := len(arFileMagic)
	for offset+60 <= len(archive) {
		header := archive[offset : offset+60]
		name := strings.TrimSuffix(strings.TrimSpace(string(header[0:16])), "/")
		size, errorValue := strconv.Atoi(strings.TrimSpace(string(header[48:58])))
		if errorValue != nil {
			return nil, fmt.Errorf("ar member %q has an unreadable size: %w", name, errorValue)
		}
		bodyStart := offset + 60
		if bodyStart+size > len(archive) {
			return nil, fmt.Errorf("ar member %q claims %d bytes the archive does not hold", name, size)
		}
		if name == wantedName || strings.HasPrefix(name, wantedName+".") {
			return decompress(name, archive[bodyStart:bodyStart+size])
		}
		offset = bodyStart + size + size%2
	}
	return nil, fmt.Errorf("the archive carries no %s member", wantedName)
}

func decompress(name string, body []byte) ([]byte, error) {
	switch path.Ext(name) {
	case "", ".tar":
		return body, nil
	case ".gz":
		reader, errorValue := gzip.NewReader(bytes.NewReader(body))
		if errorValue != nil {
			return nil, errorValue
		}
		defer reader.Close()
		return io.ReadAll(reader)
	case ".bz2":
		return io.ReadAll(bzip2.NewReader(bytes.NewReader(body)))
	case ".xz":
		return readXZ(body)
	case ".zst":
		return nil, errors.New("zstd-compressed control members are not read here; build the package with gzip or xz")
	}
	return nil, fmt.Errorf("unrecognised compression on ar member %q", name)
}

func tarMember(archive []byte, wantedName string) ([]byte, error) {
	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, errorValue := reader.Next()
		if errors.Is(errorValue, io.EOF) {
			return nil, fmt.Errorf("the control archive carries no %s", wantedName)
		}
		if errorValue != nil {
			return nil, errorValue
		}
		if path.Base(header.Name) == wantedName {
			return io.ReadAll(reader)
		}
	}
}
