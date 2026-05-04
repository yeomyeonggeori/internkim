package capabilityd

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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

func (service Service) uploadMattermostAttachments(ctx context.Context, channelID string, attachments []platformFileSpec) ([]string, error) {
	files, errorValue := service.validatePlatformFiles(attachments)
	if errorValue != nil {
		return nil, errorValue
	}
	fileIDs := []string{}
	for _, file := range files {
		fileID, errorValue := service.uploadMattermostFile(ctx, channelID, file)
		if errorValue != nil {
			return nil, errorValue
		}
		fileIDs = append(fileIDs, fileID)
	}
	return fileIDs, nil
}

func (service Service) uploadMattermostFile(ctx context.Context, channelID string, file platformFile) (string, error) {
	configuration := service.Configuration.WithDefaults()
	token := readSecretValue(configuration.MattermostTokenPath)
	if token == "" {
		return "", errors.New("mattermost bot token is not configured")
	}
	document, errorValue := os.ReadFile(file.DevicePath)
	if errorValue != nil {
		return "", errors.New("attachment file is unavailable")
	}
	var requestBody bytes.Buffer
	multipartWriter := multipart.NewWriter(&requestBody)
	if errorValue := multipartWriter.WriteField("channel_id", channelID); errorValue != nil {
		return "", errorValue
	}
	fileWriter, errorValue := multipartWriter.CreateFormFile("files", file.Filename)
	if errorValue != nil {
		return "", errorValue
	}
	if _, errorValue := fileWriter.Write(document); errorValue != nil {
		return "", errorValue
	}
	if errorValue := multipartWriter.Close(); errorValue != nil {
		return "", errorValue
	}
	requestURL := strings.TrimRight(configuration.MattermostBaseURL, "/") + "/api/v4/files"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, &requestBody)
	if errorValue != nil {
		return "", errorValue
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return "", errorValue
	}
	defer response.Body.Close()
	responseDocument, _ := io.ReadAll(response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", errors.New(string(responseDocument))
	}
	var uploadResponse struct {
		FileInfos []struct {
			ID string `json:"id"`
		} `json:"file_infos"`
	}
	if errorValue := json.Unmarshal(responseDocument, &uploadResponse); errorValue != nil {
		return "", errorValue
	}
	if len(uploadResponse.FileInfos) == 0 || strings.TrimSpace(uploadResponse.FileInfos[0].ID) == "" {
		return "", errors.New("mattermost file upload returned no file id")
	}
	return uploadResponse.FileInfos[0].ID, nil
}

func (service Service) postSlackReplyWithAttachments(ctx context.Context, handle platformHandle, request replyRequest) (string, error) {
	files, errorValue := service.validatePlatformFiles(request.Attachments)
	if errorValue != nil {
		return "", errorValue
	}
	slackFiles := []map[string]string{}
	for _, file := range files {
		fileID, errorValue := service.uploadSlackFileBytes(ctx, file)
		if errorValue != nil {
			return "", errorValue
		}
		slackFiles = append(slackFiles, map[string]string{"id": fileID, "title": firstNonEmpty(file.Title, file.Filename)})
	}
	body := map[string]any{
		"channel_id": handle.ChannelID,
		"files":      slackFiles,
	}
	if strings.TrimSpace(handle.ThreadTimestamp) != "" {
		body["thread_ts"] = handle.ThreadTimestamp
	}
	if strings.TrimSpace(request.Message) != "" {
		body["initial_comment"] = request.Message
	}
	var response struct {
		IsOK  bool   `json:"ok"`
		Error string `json:"error"`
		Files []struct {
			ID string `json:"id"`
		} `json:"files"`
	}
	if errorValue := service.slackRequest(ctx, http.MethodPost, "/files.completeUploadExternal", body, &response); errorValue != nil {
		return "", errorValue
	}
	if !response.IsOK {
		return "", errors.New("slack file complete failed: " + response.Error)
	}
	dispatchIDs := []string{}
	for _, file := range response.Files {
		if strings.TrimSpace(file.ID) != "" {
			dispatchIDs = append(dispatchIDs, file.ID)
		}
	}
	return strings.Join(dispatchIDs, ","), nil
}

func (service Service) uploadSlackFileBytes(ctx context.Context, file platformFile) (string, error) {
	var response struct {
		IsOK      bool   `json:"ok"`
		UploadURL string `json:"upload_url"`
		FileID    string `json:"file_id"`
		Error     string `json:"error"`
	}
	if errorValue := service.slackRequest(ctx, http.MethodPost, "/files.getUploadURLExternal", map[string]any{
		"filename": file.Filename,
		"length":   file.SizeBytes,
	}, &response); errorValue != nil {
		return "", errorValue
	}
	if !response.IsOK {
		return "", errors.New("slack file upload url failed: " + response.Error)
	}
	document, errorValue := os.ReadFile(file.DevicePath)
	if errorValue != nil {
		return "", errors.New("attachment file is unavailable")
	}
	uploadRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, response.UploadURL, bytes.NewReader(document))
	if errorValue != nil {
		return "", errorValue
	}
	uploadRequest.Header.Set("Content-Type", file.ContentType)
	uploadRequest.Header.Set("Content-Length", strconv.FormatInt(file.SizeBytes, 10))
	uploadResponse, errorValue := service.httpClient().Do(uploadRequest)
	if errorValue != nil {
		return "", errorValue
	}
	defer uploadResponse.Body.Close()
	if uploadResponse.StatusCode < 200 || uploadResponse.StatusCode >= 300 {
		responseDocument, _ := io.ReadAll(uploadResponse.Body)
		return "", errors.New(string(responseDocument))
	}
	return response.FileID, nil
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
