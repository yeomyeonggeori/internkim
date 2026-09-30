package pacmanrepository

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"

	"github.com/klauspost/compress/zstd"
)

type packageInformation struct {
	fields map[string][]string
	files  []string
}

func (information packageInformation) value(name string) string {
	values := information.fields[name]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (information packageInformation) size() int64 {
	size, _ := strconv.ParseInt(information.value("size"), 10, 64)
	return size
}

func readPackage(contents []byte) (packageInformation, error) {
	decoder, errorValue := zstd.NewReader(bytes.NewReader(contents))
	if errorValue != nil {
		return packageInformation{}, fmt.Errorf("a pacman package is a zstd tar: %w", errorValue)
	}
	defer decoder.Close()
	reader := tar.NewReader(decoder)
	information := packageInformation{fields: map[string][]string{}}
	foundInformation := false
	for {
		header, errorValue := reader.Next()
		if errors.Is(errorValue, io.EOF) {
			break
		}
		if errorValue != nil {
			return packageInformation{}, errorValue
		}
		if header.Name == ".PKGINFO" {
			body, errorValue := io.ReadAll(reader)
			if errorValue != nil {
				return packageInformation{}, errorValue
			}
			parsePackageInformation(string(body), information.fields)
			foundInformation = true
			continue
		}
		if !strings.HasPrefix(path.Base(header.Name), ".") || strings.Contains(header.Name, "/") {
			information.files = append(information.files, header.Name)
		}
	}
	if !foundInformation {
		return packageInformation{}, errors.New("the package carries no .PKGINFO")
	}
	for _, required := range []string{"pkgname", "pkgver", "arch"} {
		if information.value(required) == "" {
			return packageInformation{}, fmt.Errorf(".PKGINFO names no %s", required)
		}
	}
	return information, nil
}

func parsePackageInformation(text string, fields map[string][]string) {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		name, value, found := strings.Cut(line, " = ")
		if found {
			fields[name] = append(fields[name], value)
		}
	}
}
