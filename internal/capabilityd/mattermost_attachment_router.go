package capabilityd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gitlab.com/eastriver/internkim/internal/mattermostdefaults"
)

type mattermostImportRecord struct {
	FileID    string `json:"fileID"`
	PostID    string `json:"postID"`
	ChannelID string `json:"channelID"`
}

type mattermostImportRecordStore struct {
	Records []mattermostImportRecord `json:"records"`
}

type mattermostCircleChannelEntry struct {
	CircleID    string `json:"circleID"`
	ChannelName string `json:"channelName"`
}

type mattermostCircleSyncPolicy struct {
	MattermostPrivateChannels []mattermostCircleChannelEntry `json:"mattermostPrivateChannels"`
}

type mattermostCircleChannelPolicyDocument struct {
	CircleSync mattermostCircleSyncPolicy `json:"circleSync"`
}

var mattermostImportStoreMutex sync.Mutex

func (service Service) routeUploadedMattermostAttachments(ctx context.Context, post mattermostPost, channelType string, channelName string, botUserID string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf("mattermost attachment router: recovered from panic: postID=%s: %v", post.ID, recovered)
		}
	}()
	if strings.TrimSpace(post.Type) != "" {
		return
	}
	if strings.TrimSpace(post.UserID) == strings.TrimSpace(botUserID) {
		return
	}
	if len(post.FileIDs) == 0 {
		return
	}

	destinationAgentPath, isRoutable := service.resolveUploadDestinationDirectory(ctx, post, channelType, channelName)
	if !isRoutable {
		return
	}

	existingRecords := service.loadMattermostImportRecords()
	recordedFileIDs := importedFileIDSet(existingRecords)

	target, errorValue := service.resolveMattermostAttachmentImportTarget(destinationAgentPath)
	if errorValue != nil {
		log.Printf("mattermost attachment router: resolve target failed: postID=%s path=%s: %v", post.ID, destinationAgentPath, errorValue)
		return
	}
	if errorValue := os.MkdirAll(target.HostDirectoryPath, 0o755); errorValue != nil {
		log.Printf("mattermost attachment router: mkdir failed: path=%s: %v", target.HostDirectoryPath, errorValue)
		return
	}

	usedFilenames := map[string]bool{}
	newRecords := []mattermostImportRecord{}

	for _, fileID := range post.FileIDs {
		fileID = strings.TrimSpace(fileID)
		if fileID == "" || recordedFileIDs[fileID] {
			continue
		}
		attachment := platformInputAttachment{
			Platform:  "mattermost",
			FileID:    fileID,
			MessageID: post.ID,
		}
		metadata, errorValue := service.mattermostAttachmentMetadata(ctx, fileID)
		if errorValue != nil {
			log.Printf("mattermost attachment router: metadata failed: fileID=%s postID=%s: %v", fileID, post.ID, errorValue)
			continue
		}
		download, errorValue := service.downloadMattermostAttachment(ctx, fileID)
		if errorValue != nil {
			log.Printf("mattermost attachment router: download failed: fileID=%s postID=%s: %v", fileID, post.ID, errorValue)
			continue
		}
		imported := service.writeMattermostImportedAttachment(target, usedFilenames, post.ID, attachment, metadata, download)
		if !imported.IsAvailable {
			log.Printf("mattermost attachment router: write failed: fileID=%s postID=%s: %s", fileID, post.ID, imported.Message)
			continue
		}
		log.Printf("mattermost attachment router: imported: fileID=%s postID=%s path=%s", fileID, post.ID, imported.Path)
		newRecords = append(newRecords, mattermostImportRecord{
			FileID:    fileID,
			PostID:    post.ID,
			ChannelID: post.ChannelID,
		})
	}

	if len(newRecords) == 0 {
		return
	}

	updatedRecords := append(existingRecords, newRecords...)
	if errorValue := service.saveMattermostImportRecords(updatedRecords); errorValue != nil {
		log.Printf("mattermost attachment router: save records failed: %v", errorValue)
	}
}

func importedFileIDSet(records []mattermostImportRecord) map[string]bool {
	fileIDs := map[string]bool{}
	for _, record := range records {
		if strings.TrimSpace(record.FileID) != "" {
			fileIDs[record.FileID] = true
		}
	}
	return fileIDs
}

func (service Service) resolveUploadDestinationDirectory(ctx context.Context, post mattermostPost, channelType string, channelName string) (string, bool) {
	kind, isKnown := classifyMattermostUploadChannel(channelType, channelName)
	if !isKnown {
		return "", false
	}

	switch kind {
	case "dm":
		resolution, errorValue := service.fetchPlatformDMRecipientResolution(ctx, post.UserID)
		if errorValue != nil {
			return "", false
		}
		if resolution.Recipient == nil || strings.TrimSpace(resolution.Recipient.PersonID) == "" {
			return "", false
		}
		return "/workspace/private/people/" + strings.TrimSpace(resolution.Recipient.PersonID), true

	case "public":
		return "/workspace/shared/public", true

	case "circle-candidate":
		circleID, isFound := service.resolveCircleForMattermostChannel(ctx, channelName)
		if !isFound {
			return "", false
		}
		return "/workspace/circles/" + circleID, true

	default:
		return "", false
	}
}

func classifyMattermostUploadChannel(channelType string, channelName string) (string, bool) {
	if strings.EqualFold(channelType, "D") {
		return "dm", true
	}
	normalizedName := strings.ToLower(strings.TrimSpace(channelName))
	switch normalizedName {
	case mattermostdefaults.FlowChannelName,
		mattermostdefaults.CalendarChannelName,
		mattermostdefaults.AttendanceChannelName:
		return "", false
	case mattermostdefaults.TownSquareChannelName,
		mattermostdefaults.OffTopicChannelName:
		return "public", true
	}
	if strings.EqualFold(channelType, "O") || strings.EqualFold(channelType, "P") {
		return "circle-candidate", true
	}
	return "", false
}

func (service Service) resolveCircleForMattermostChannel(ctx context.Context, channelName string) (string, bool) {
	policyDocument, errorValue := service.fetchMattermostCircleChannelPolicy(ctx)
	if errorValue != nil {
		return "", false
	}
	for _, entry := range policyDocument.CircleSync.MattermostPrivateChannels {
		if strings.EqualFold(strings.TrimSpace(entry.ChannelName), strings.TrimSpace(channelName)) {
			return strings.TrimSpace(entry.CircleID), true
		}
	}
	return "", false
}

func (service Service) fetchMattermostCircleChannelPolicy(ctx context.Context) (mattermostCircleChannelPolicyDocument, error) {
	endpoint := strings.TrimRight(firstNonEmpty(service.Configuration.BlueclawBaseURL, DefaultConfiguration().BlueclawBaseURL), "/") + "/admin/api/policy"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if errorValue != nil {
		return mattermostCircleChannelPolicyDocument{}, errorValue
	}
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return mattermostCircleChannelPolicyDocument{}, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return mattermostCircleChannelPolicyDocument{}, fmt.Errorf("circle channel policy lookup failed with status %d", response.StatusCode)
	}
	var policyDocument mattermostCircleChannelPolicyDocument
	if errorValue := json.NewDecoder(response.Body).Decode(&policyDocument); errorValue != nil {
		return mattermostCircleChannelPolicyDocument{}, errorValue
	}
	return policyDocument, nil
}

func (service Service) mattermostImportStorePath() string {
	workspacePath := service.Configuration.WithDefaults().BlueclawWorkspacePath
	return filepath.Join(workspacePath, ".blueclaw", "mattermost-imports.json")
}

func (service Service) loadMattermostImportRecords() []mattermostImportRecord {
	mattermostImportStoreMutex.Lock()
	defer mattermostImportStoreMutex.Unlock()
	return service.readMattermostImportRecords()
}

func (service Service) readMattermostImportRecords() []mattermostImportRecord {
	document, errorValue := os.ReadFile(service.mattermostImportStorePath())
	if errorValue != nil {
		return []mattermostImportRecord{}
	}
	var store mattermostImportRecordStore
	if errorValue := json.Unmarshal(document, &store); errorValue != nil {
		return []mattermostImportRecord{}
	}
	return store.Records
}

func (service Service) saveMattermostImportRecords(records []mattermostImportRecord) error {
	mattermostImportStoreMutex.Lock()
	defer mattermostImportStoreMutex.Unlock()
	return service.writeMattermostImportRecords(records)
}

func (service Service) writeMattermostImportRecords(records []mattermostImportRecord) error {
	storePath := service.mattermostImportStorePath()
	if errorValue := os.MkdirAll(filepath.Dir(storePath), 0o755); errorValue != nil {
		return errorValue
	}
	store := mattermostImportRecordStore{Records: records}
	document, errorValue := json.MarshalIndent(store, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	return os.WriteFile(storePath, document, 0o644)
}
