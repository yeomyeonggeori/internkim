package capabilityd

import (
	"encoding/base64"
	"errors"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

type platformFileSpec struct {
	DevicePath    string `json:"devicePath"`
	Filename      string `json:"filename,omitempty"`
	ContentType   string `json:"contentType,omitempty"`
	SizeBytes     int64  `json:"sizeBytes,omitempty"`
	Title         string `json:"title,omitempty"`
	ContentBase64 string `json:"contentBase64,omitempty"`
}

type platformFile struct {
	DevicePath  string
	Filename    string
	ContentType string
	SizeBytes   int64
	Title       string
}

func (service Service) validatePlatformFiles(attachments []platformFileSpec) ([]platformFile, error) {
	files := []platformFile{}
	for _, attachment := range attachments {
		file, errorValue := service.validatePlatformFile(attachment)
		if errorValue != nil {
			return nil, errorValue
		}
		files = append(files, file)
	}
	return files, nil
}

func (service Service) validatePlatformFile(attachment platformFileSpec) (platformFile, error) {
	configuration := service.Configuration.WithDefaults()
	if strings.TrimSpace(attachment.ContentBase64) != "" {
		return service.materializeInlinePlatformFile(configuration, attachment)
	}
	devicePath := strings.TrimSpace(attachment.DevicePath)
	if devicePath == "" {
		return platformFile{}, errors.New("attachment devicePath is required")
	}
	if !isPathUnderDirectory(configuration.CompanionFileDirectory, devicePath) {
		return platformFile{}, errors.New("attachment devicePath is outside the companion file directory")
	}
	information, errorValue := os.Stat(devicePath)
	if errorValue != nil {
		return platformFile{}, errors.New("attachment file is unavailable")
	}
	if !information.Mode().IsRegular() {
		return platformFile{}, errors.New("attachment is not a regular file")
	}
	if attachment.SizeBytes > 0 && attachment.SizeBytes != information.Size() {
		return platformFile{}, errors.New("attachment size does not match file")
	}
	filename := safeDeviceBrowserFilename(firstNonEmpty(attachment.Filename, filepath.Base(devicePath)))
	if filename == "" {
		return platformFile{}, errors.New("attachment filename is required")
	}
	contentType := strings.TrimSpace(attachment.ContentType)
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(filename))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return platformFile{
		DevicePath:  devicePath,
		Filename:    filename,
		ContentType: contentType,
		SizeBytes:   information.Size(),
		Title:       firstNonEmpty(attachment.Title, strings.TrimSuffix(filename, filepath.Ext(filename))),
	}, nil
}

func (service Service) materializeInlinePlatformFile(configuration Configuration, attachment platformFileSpec) (platformFile, error) {
	filename := safeDeviceBrowserFilename(firstNonEmpty(attachment.Filename, filepath.Base(strings.TrimSpace(attachment.DevicePath))))
	if filename == "" {
		return platformFile{}, errors.New("attachment filename is required")
	}
	document, errorValue := base64.StdEncoding.DecodeString(strings.TrimSpace(attachment.ContentBase64))
	if errorValue != nil {
		return platformFile{}, errors.New("attachment payload is not valid base64")
	}
	if attachment.SizeBytes > 0 && attachment.SizeBytes != int64(len(document)) {
		return platformFile{}, errors.New("attachment size does not match file")
	}
	if errorValue := os.MkdirAll(configuration.CompanionFileDirectory, 0o700); errorValue != nil {
		return platformFile{}, errorValue
	}
	devicePath := filepath.Join(configuration.CompanionFileDirectory, filename)
	if errorValue := os.WriteFile(devicePath, document, 0o600); errorValue != nil {
		return platformFile{}, errorValue
	}
	contentType := strings.TrimSpace(attachment.ContentType)
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(filename))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return platformFile{
		DevicePath:  devicePath,
		Filename:    filename,
		ContentType: contentType,
		SizeBytes:   int64(len(document)),
		Title:       firstNonEmpty(attachment.Title, strings.TrimSuffix(filename, filepath.Ext(filename))),
	}, nil
}

func isPathUnderDirectory(directory string, path string) bool {
	resolvedDirectory, directoryError := filepath.EvalSymlinks(filepath.Clean(directory))
	resolvedPath, pathError := filepath.EvalSymlinks(filepath.Clean(path))
	if directoryError != nil || pathError != nil {
		return false
	}
	relativePath, errorValue := filepath.Rel(resolvedDirectory, resolvedPath)
	return errorValue == nil && relativePath != "." && !strings.HasPrefix(relativePath, "..") && !filepath.IsAbs(relativePath)
}
