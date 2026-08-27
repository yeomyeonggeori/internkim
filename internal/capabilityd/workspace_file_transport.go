package capabilityd

import (
	"encoding/base64"
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

// A person's file arrives as its content, read where that person's identity
// exists. This daemon runs beside the workspace as root with nobody to become,
// so it never opens one: whether somebody may have a file was already answered
// by the kernel, as them.
type carriedWorkspaceFile struct {
	AgentPath string
	Filename  string
	Content   []byte
}

func carriedWorkspaceFileOf(request capabilities.ToolInvokeRequest) (carriedWorkspaceFile, error) {
	carried := request.Transport.WorkspaceFile
	if carried == nil {
		return carriedWorkspaceFile{}, errors.New("this file was named but not carried; the caller reads it as the person who asked and sends its content")
	}
	content, errorValue := base64.StdEncoding.DecodeString(carried.ContentBase64)
	if errorValue != nil {
		return carriedWorkspaceFile{}, errorValue
	}
	return carriedWorkspaceFile{
		AgentPath: carried.WorkspacePath,
		Filename:  carried.Filename,
		Content:   content,
	}, nil
}

// The conversion helpers and the image encoders take a file, so a carried one
// is put back on disk beside this daemon for as long as the call lasts. It is
// this daemon's own copy in its own temporary directory, not a reach into
// anybody's workspace.
func (file carriedWorkspaceFile) writeWhileReading() (string, func(), error) {
	directoryPath, errorValue := os.MkdirTemp("", "internkim-carried-file-")
	if errorValue != nil {
		return "", func() {}, errorValue
	}
	filename := filepath.Base(file.Filename)
	if filename == "" || filename == "." || filename == string(filepath.Separator) {
		filename = filepath.Base(file.AgentPath)
	}
	filePath := filepath.Join(directoryPath, filename)
	if errorValue := os.WriteFile(filePath, file.Content, 0o600); errorValue != nil {
		os.RemoveAll(directoryPath)
		return "", func() {}, errorValue
	}
	return filePath, func() { os.RemoveAll(directoryPath) }, nil
}

func detectCarriedFileContentType(file carriedWorkspaceFile) string {
	sniffed := file.Content
	if len(sniffed) > 512 {
		sniffed = sniffed[:512]
	}
	return http.DetectContentType(sniffed)
}
