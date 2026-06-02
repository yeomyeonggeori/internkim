package capabilityd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type mattermostAttachmentImportRequest struct {
	MessageID           string                    `json:"messageID"`
	TargetDirectoryPath string                    `json:"targetDirectoryPath"`
	InputAttachments    []platformInputAttachment `json:"inputAttachments"`
	Attachments         []platformInputAttachment `json:"attachments"`
}

type mattermostAttachmentImportResponse struct {
	InputAttachments []platformInputAttachment `json:"inputAttachments"`
}

type mattermostFileMetadata struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Extension   string `json:"extension"`
	SizeBytes   int64  `json:"size"`
	ContentType string `json:"mime_type"`
}

type mattermostAttachmentImportTarget struct {
	AgentDirectoryPath string
	HostDirectoryPath  string
}

type mattermostAttachmentDownload struct {
	Content     []byte
	ContentType string
}

func (service Service) mattermostImportAttachmentsFromRequest(ctx context.Context, reader io.Reader) (any, error) {
	payload, errorValue := io.ReadAll(reader)
	if errorValue != nil {
		return nil, errorValue
	}
	return service.mattermostImportAttachments(ctx, payload)
}

func (service Service) mattermostImportAttachments(ctx context.Context, payload json.RawMessage) (mattermostAttachmentImportResponse, error) {
	var request mattermostAttachmentImportRequest
	if errorValue := json.Unmarshal(payload, &request); errorValue != nil {
		return mattermostAttachmentImportResponse{}, errorValue
	}
	attachments := request.importAttachments()
	if len(attachments) == 0 {
		return mattermostAttachmentImportResponse{}, nil
	}
	target, errorValue := service.resolveMattermostAttachmentImportTarget(request.TargetDirectoryPath)
	if errorValue != nil {
		return mattermostAttachmentImportResponse{}, errorValue
	}
	if errorValue := os.MkdirAll(target.HostDirectoryPath, 0o755); errorValue != nil {
		return mattermostAttachmentImportResponse{}, errorValue
	}
	importedAttachments := service.importMattermostAttachments(ctx, target, request.MessageID, attachments)
	return mattermostAttachmentImportResponse{InputAttachments: importedAttachments}, nil
}

func (request mattermostAttachmentImportRequest) importAttachments() []platformInputAttachment {
	if len(request.InputAttachments) > 0 {
		return append([]platformInputAttachment{}, request.InputAttachments...)
	}
	return append([]platformInputAttachment{}, request.Attachments...)
}

func (service Service) resolveMattermostAttachmentImportTarget(agentDirectoryPath string) (mattermostAttachmentImportTarget, error) {
	agentDirectoryPath, errorValue := cleanMattermostImportAgentDirectoryPath(agentDirectoryPath)
	if errorValue != nil {
		return mattermostAttachmentImportTarget{}, errorValue
	}
	workspacePath := service.Configuration.WithDefaults().BlueclawWorkspacePath
	hostDirectoryPath := filepath.Join(workspacePath, strings.TrimPrefix(agentDirectoryPath, "/workspace/"))
	if agentDirectoryPath == "/workspace" {
		hostDirectoryPath = workspacePath
	}
	hostDirectoryPath, errorValue = cleanHostWorkspacePath(workspacePath, hostDirectoryPath)
	if errorValue != nil {
		return mattermostAttachmentImportTarget{}, errorValue
	}
	return mattermostAttachmentImportTarget{
		AgentDirectoryPath: agentDirectoryPath,
		HostDirectoryPath:  hostDirectoryPath,
	}, nil
}

func cleanMattermostImportAgentDirectoryPath(path string) (string, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return "", errors.New("targetDirectoryPath is required")
	}
	if !filepath.IsAbs(trimmedPath) {
		return "", errors.New("targetDirectoryPath must be an absolute /workspace path")
	}
	cleanPath := filepath.ToSlash(filepath.Clean(trimmedPath))
	if cleanPath != "/workspace" && !strings.HasPrefix(cleanPath, "/workspace/") {
		return "", errors.New("targetDirectoryPath must stay under /workspace")
	}
	if cleanPath == "/workspace/.blueclaw" || strings.HasPrefix(cleanPath, "/workspace/.blueclaw/") {
		return "", errors.New("targetDirectoryPath cannot use Blueclaw internal files")
	}
	return cleanPath, nil
}

func (service Service) importMattermostAttachments(ctx context.Context, target mattermostAttachmentImportTarget, fallbackMessageID string, attachments []platformInputAttachment) []platformInputAttachment {
	importedAttachments := make([]platformInputAttachment, 0, len(attachments))
	usedFilenames := map[string]bool{}
	for _, attachment := range attachments {
		importedAttachments = append(importedAttachments, service.importMattermostAttachment(ctx, target, fallbackMessageID, attachment, usedFilenames))
	}
	return importedAttachments
}

func (service Service) importMattermostAttachment(ctx context.Context, target mattermostAttachmentImportTarget, fallbackMessageID string, attachment platformInputAttachment, usedFilenames map[string]bool) platformInputAttachment {
	fileID := strings.TrimSpace(attachment.FileID)
	if fileID == "" {
		return unavailableMattermostAttachment(attachment, fallbackMessageID, "missing_file_id", "fileID is required")
	}
	metadata, errorValue := service.mattermostAttachmentMetadata(ctx, fileID)
	if errorValue != nil {
		return unavailableMattermostAttachment(attachment, fallbackMessageID, "metadata_fetch_failed", errorValue.Error())
	}
	download, errorValue := service.downloadMattermostAttachment(ctx, fileID)
	if errorValue != nil {
		return unavailableMattermostAttachment(withMattermostAttachmentMetadata(attachment, metadata, fallbackMessageID), fallbackMessageID, "download_failed", errorValue.Error())
	}
	return service.writeMattermostImportedAttachment(target, usedFilenames, fallbackMessageID, attachment, metadata, download)
}

func (service Service) writeMattermostImportedAttachment(target mattermostAttachmentImportTarget, usedFilenames map[string]bool, fallbackMessageID string, attachment platformInputAttachment, metadata mattermostFileMetadata, download mattermostAttachmentDownload) platformInputAttachment {
	filename := uniqueMattermostImportFilename(target.HostDirectoryPath, usedFilenames, mattermostImportFilename(attachment, metadata))
	hostPath := filepath.Join(target.HostDirectoryPath, filename)
	importedAttachment := withMattermostAttachmentMetadata(attachment, metadata, fallbackMessageID)
	if errorValue := os.WriteFile(hostPath, download.Content, 0o644); errorValue != nil {
		return unavailableMattermostAttachment(importedAttachment, fallbackMessageID, "write_failed", errorValue.Error())
	}
	importedAttachment.Filename = filename
	importedAttachment.ContentType = firstNonEmpty(importedAttachment.ContentType, download.ContentType)
	importedAttachment.SizeBytes = int64(len(download.Content))
	importedAttachment.Path = filepath.ToSlash(filepath.Join(target.AgentDirectoryPath, filename))
	importedAttachment.IsAvailable = true
	importedAttachment.ErrorCode = ""
	importedAttachment.Message = ""
	return importedAttachment
}

func unavailableMattermostAttachment(attachment platformInputAttachment, fallbackMessageID string, errorCode string, message string) platformInputAttachment {
	attachment.Platform = "mattermost"
	attachment.MessageID = firstNonEmpty(strings.TrimSpace(attachment.MessageID), strings.TrimSpace(fallbackMessageID))
	attachment.IsAvailable = false
	attachment.ErrorCode = strings.TrimSpace(errorCode)
	attachment.Message = strings.TrimSpace(message)
	return attachment
}

func withMattermostAttachmentMetadata(attachment platformInputAttachment, metadata mattermostFileMetadata, fallbackMessageID string) platformInputAttachment {
	attachment.Platform = "mattermost"
	attachment.FileID = firstNonEmpty(strings.TrimSpace(attachment.FileID), strings.TrimSpace(metadata.ID))
	attachment.MessageID = firstNonEmpty(strings.TrimSpace(attachment.MessageID), strings.TrimSpace(fallbackMessageID))
	attachment.Filename = firstNonEmpty(strings.TrimSpace(attachment.Filename), strings.TrimSpace(metadata.Name))
	attachment.ContentType = firstNonEmpty(strings.TrimSpace(attachment.ContentType), strings.TrimSpace(metadata.ContentType), mime.TypeByExtension("."+strings.TrimPrefix(metadata.Extension, ".")))
	if attachment.SizeBytes <= 0 {
		attachment.SizeBytes = metadata.SizeBytes
	}
	return attachment
}

func mattermostImportFilename(attachment platformInputAttachment, metadata mattermostFileMetadata) string {
	filename := safeDeviceBrowserFilename(firstNonEmpty(attachment.Filename, metadata.Name, metadata.ID, attachment.FileID))
	if filename != "" {
		return filename
	}
	return "mattermost-file"
}

func uniqueMattermostImportFilename(hostDirectoryPath string, usedFilenames map[string]bool, filename string) string {
	filename = safeDeviceBrowserFilename(filename)
	if filename == "" {
		filename = "mattermost-file"
	}
	extension := filepath.Ext(filename)
	stem := strings.TrimSuffix(filename, extension)
	for index := 1; ; index++ {
		candidate := filename
		if index > 1 {
			candidate = stem + "-" + fmt.Sprintf("%d", index) + extension
		}
		if usedFilenames[candidate] {
			continue
		}
		if _, errorValue := os.Stat(filepath.Join(hostDirectoryPath, candidate)); errorValue == nil {
			continue
		}
		usedFilenames[candidate] = true
		return candidate
	}
}

func (service Service) mattermostAttachmentMetadata(ctx context.Context, fileID string) (mattermostFileMetadata, error) {
	var metadata mattermostFileMetadata
	errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/files/"+url.PathEscape(fileID)+"/info", nil, &metadata)
	return metadata, errorValue
}

func (service Service) downloadMattermostAttachment(ctx context.Context, fileID string) (mattermostAttachmentDownload, error) {
	response, errorValue := service.mattermostRawRequest(ctx, http.MethodGet, "/api/v4/files/"+url.PathEscape(fileID))
	if errorValue != nil {
		return mattermostAttachmentDownload{}, errorValue
	}
	defer response.Body.Close()
	content, errorValue := io.ReadAll(response.Body)
	if errorValue != nil {
		return mattermostAttachmentDownload{}, errorValue
	}
	contentType := strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0])
	return mattermostAttachmentDownload{Content: content, ContentType: contentType}, nil
}

func (service Service) mattermostRawRequest(ctx context.Context, method string, path string) (*http.Response, error) {
	token := readSecretValue(service.Configuration.MattermostTokenPath)
	if token == "" {
		return nil, errors.New("mattermost bot token is not configured")
	}
	requestURL := strings.TrimRight(service.Configuration.MattermostBaseURL, "/") + path
	request, errorValue := http.NewRequestWithContext(ctx, method, requestURL, nil)
	if errorValue != nil {
		return nil, errorValue
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return nil, errorValue
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return response, nil
	}
	defer response.Body.Close()
	responseDocument, _ := io.ReadAll(response.Body)
	return nil, fmt.Errorf("http status %d: %s", response.StatusCode, strings.TrimSpace(string(responseDocument)))
}
