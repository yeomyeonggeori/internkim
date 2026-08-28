package capabilityd

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const maximumInputImagePartBytes = 8 * 1024 * 1024
const maximumInputMarkdownPreviewBytes = 120000

type mattermostAttachmentImportRequest struct {
	MessageID           string                    `json:"messageID"`
	TargetDirectoryPath string                    `json:"targetDirectoryPath"`
	InputAttachments    []platformInputAttachment `json:"inputAttachments"`
	Attachments         []platformInputAttachment `json:"attachments"`
}

type mattermostAttachmentImportResponse struct {
	InputAttachments []platformInputAttachment `json:"inputAttachments"`
	InputParts       []platformPart            `json:"inputParts,omitempty"`
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
	importedAttachments := service.importMattermostAttachments(ctx, target, request.MessageID, attachments)
	return mattermostAttachmentImportResponse{
		InputAttachments: importedAttachments,
		InputParts:       service.mattermostInputParts(ctx, importedAttachments),
	}, nil
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
	return mattermostAttachmentImportTarget{AgentDirectoryPath: agentDirectoryPath}, nil
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
	return carriedMattermostImportedAttachment(target, usedFilenames, fallbackMessageID, attachment, metadata, download)
}

// capabilityd runs beside the workspace image rather than inside it, so a file
// it writes lands where the agent's /workspace never looks. The bytes travel
// back with the path they belong at, and blueclaw writes them there as the
// person the message was sent to.
func carriedMattermostImportedAttachment(target mattermostAttachmentImportTarget, usedFilenames map[string]bool, fallbackMessageID string, attachment platformInputAttachment, metadata mattermostFileMetadata, download mattermostAttachmentDownload) platformInputAttachment {
	importedAttachment := withMattermostAttachmentMetadata(attachment, metadata, fallbackMessageID)
	filename := uniqueMattermostImportFilename(usedFilenames, mattermostImportFilename(attachment, metadata))
	importedAttachment = withMattermostImportedAttachmentPath(target, importedAttachment, download.ContentType, int64(len(download.Content)), filename)
	importedAttachment.ContentBase64 = base64.StdEncoding.EncodeToString(download.Content)
	return importedAttachment
}

func withMattermostImportedAttachmentPath(target mattermostAttachmentImportTarget, attachment platformInputAttachment, contentType string, sizeBytes int64, filename string) platformInputAttachment {
	importedAttachment := attachment
	importedAttachment.Filename = filename
	importedAttachment.ContentType = firstNonEmpty(importedAttachment.ContentType, contentType)
	importedAttachment.SizeBytes = sizeBytes
	importedAttachment.Path = filepath.ToSlash(filepath.Join(target.AgentDirectoryPath, filename))
	importedAttachment.IsAvailable = true
	importedAttachment.ErrorCode = ""
	importedAttachment.Message = ""
	return importedAttachment
}

func (service Service) mattermostInputParts(ctx context.Context, attachments []platformInputAttachment) []platformPart {
	parts := []platformPart{}
	for _, attachment := range attachments {
		part := service.mattermostInputPart(ctx, attachment)
		if strings.TrimSpace(part.Type) != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func (service Service) mattermostInputPart(ctx context.Context, attachment platformInputAttachment) platformPart {
	filePart := platformFilePart{
		Path:        strings.TrimSpace(attachment.Path),
		Filename:    strings.TrimSpace(attachment.Filename),
		ContentType: strings.TrimSpace(attachment.ContentType),
		SizeBytes:   attachment.SizeBytes,
	}
	source := platformPartSource{
		Platform:  strings.TrimSpace(attachment.Platform),
		MessageID: strings.TrimSpace(attachment.MessageID),
		FileID:    strings.TrimSpace(attachment.FileID),
	}
	if !attachment.IsAvailable {
		filePart.ConversionStatus = "unavailable"
		filePart.ConversionMessage = strings.TrimSpace(firstNonEmpty(attachment.Message, attachment.ErrorCode))
		return platformPart{Type: "file", File: &filePart, Source: source, Visibility: "llm"}
	}
	content, errorValue := base64.StdEncoding.DecodeString(attachment.ContentBase64)
	if errorValue != nil {
		filePart.ConversionStatus = "failed"
		filePart.ConversionMessage = errorValue.Error()
		return platformPart{Type: "file", File: &filePart, Source: source, Visibility: "llm"}
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(attachment.ContentType)), "image/") {
		return mattermostImageInputPart(content, attachment, filePart, source)
	}
	markdownPreview, conversionStatus, conversionMessage := service.mattermostMarkdownPreview(ctx, content, attachment)
	filePart.MarkdownPreview = markdownPreview
	filePart.ConversionStatus = conversionStatus
	filePart.ConversionMessage = conversionMessage
	return platformPart{Type: "file", File: &filePart, Source: source, Visibility: "llm"}
}

// The picture's bytes are not filled here: blueclaw's connector layer is the
// one owner of showing a message's picture to the model, whatever messenger it
// came over. This import hands back the file's content on the attachment and a
// part that names where the file will live.
func mattermostImageInputPart(content []byte, attachment platformInputAttachment, filePart platformFilePart, source platformPartSource) platformPart {
	if len(content) > maximumInputImagePartBytes {
		filePart.ConversionStatus = "image_too_large"
		filePart.ConversionMessage = "image is larger than the model input limit"
		return platformPart{Type: "file", File: &filePart, Source: source, Visibility: "llm"}
	}
	return platformPart{
		Type: "image",
		Image: &platformImagePart{
			MimeType: strings.TrimSpace(attachment.ContentType),
			Path:     strings.TrimSpace(attachment.Path),
			Filename: strings.TrimSpace(attachment.Filename),
		},
		File:       &filePart,
		Source:     source,
		Visibility: "llm",
	}
}

func (service Service) mattermostMarkdownPreview(ctx context.Context, content []byte, attachment platformInputAttachment) (string, string, string) {
	if preview, status, message, isRawText := mattermostRawTextPreview(content, attachment.Path, attachment.ContentType, attachment.SizeBytes); isRawText {
		return preview, status, message
	}
	carried := carriedWorkspaceFile{AgentPath: attachment.Path, Filename: attachment.Filename, Content: content}
	filePath, releaseFile, errorValue := carried.writeWhileReading()
	if errorValue != nil {
		return "", "failed", errorValue.Error()
	}
	defer releaseFile()
	helperResponse, _, _, errorValue := service.convertDocument(ctx, filePath, 20)
	if errorValue != nil {
		return "", "failed", errorValue.Error()
	}
	preview, isTruncated := truncateTextByBytes(strings.TrimSpace(helperResponse.Content), maximumInputMarkdownPreviewBytes)
	if preview == "" {
		return "", "empty", "document conversion returned no content"
	}
	if isTruncated {
		return preview, "truncated", "markdown preview was truncated"
	}
	return preview, "converted", ""
}

func mattermostRawTextPreview(document []byte, filePath string, contentType string, sizeBytes int64) (string, string, string, bool) {
	if !mattermostAttachmentLooksLikeRawText(filePath, contentType) {
		return "", "", "", false
	}
	if !utf8.Valid(document) || bytes.IndexByte(document, 0) >= 0 {
		return "", "", "", false
	}
	readableText := collapseInlineBase64(strings.TrimSpace(string(document)))
	content, isTruncated := truncateTextByBytes(readableText, maximumInputMarkdownPreviewBytes)
	if content == "" {
		return "", "empty", "file contains no text", true
	}
	if isTruncated || sizeBytes > int64(maximumInputMarkdownPreviewBytes) {
		return content, "truncated", "raw text preview was truncated", true
	}
	return content, "converted", "", true
}

var inlineBase64RunPattern = regexp.MustCompile(`[A-Za-z0-9+/]{256,}={0,2}`)

func collapseInlineBase64(content string) string {
	return inlineBase64RunPattern.ReplaceAllStringFunc(content, func(run string) string {
		return "[base64 data omitted: " + strconv.Itoa(len(run)) + " chars]"
	})
}

func mattermostAttachmentLooksLikeRawText(hostPath string, contentType string) bool {
	normalizedContentType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if normalizedContentType == "text/html" || normalizedContentType == "application/xhtml+xml" {
		return false
	}
	if isHTMLAttachmentExtension(hostPath) {
		return false
	}
	if strings.HasPrefix(normalizedContentType, "text/") {
		return true
	}
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(hostPath))) {
	case ".css", ".js", ".jsx", ".ts", ".tsx", ".json", ".md", ".txt", ".csv", ".xml", ".svg":
		return true
	default:
		return false
	}
}

func isHTMLAttachmentExtension(hostPath string) bool {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(hostPath))) {
	case ".html", ".htm", ".xhtml":
		return true
	default:
		return false
	}
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

func uniqueMattermostImportFilename(usedFilenames map[string]bool, filename string) string {
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
