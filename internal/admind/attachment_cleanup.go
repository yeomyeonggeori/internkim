package admind

import (
	"context"
	"crypto/sha256"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type attachmentCleanupAction struct {
	Action string `json:"action"`
	From   string `json:"from"`
	To     string `json:"to,omitempty"`
}

type attachmentCleanupReport struct {
	Applied            bool                      `json:"applied"`
	ConversationsSeen  int                       `json:"conversationsSeen"`
	DuplicatesRemoved  int                       `json:"duplicatesRemoved"`
	FilesFlattened     int                       `json:"filesFlattened"`
	EmptyFoldersRemoved int                      `json:"emptyFoldersRemoved"`
	Actions            []attachmentCleanupAction `json:"actions"`
}

func (service *Service) handleAttachmentCleanup(responseWriter http.ResponseWriter, request *http.Request, apply bool) {
	report := service.cleanupAttachmentInboxes(apply)
	service.writeJSON(responseWriter, report)
}

func (service *Service) cleanupAttachmentInboxes(apply bool) attachmentCleanupReport {
	report := attachmentCleanupReport{Applied: apply, Actions: []attachmentCleanupAction{}}
	for _, conversationDirectory := range service.attachmentConversationDirectories() {
		cleanupConversationAttachments(conversationDirectory, service.Configuration.BlueclawWorkspacePath, apply, &report)
	}
	return report
}

func (service *Service) attachmentConversationDirectories() []string {
	workspaceRoot := service.Configuration.BlueclawWorkspacePath
	scopeGlobs := []string{
		filepath.Join(workspaceRoot, "circles", "*"),
		filepath.Join(workspaceRoot, "private", "people", "*"),
		filepath.Join(workspaceRoot, "shared", "*"),
	}
	conversationDirectories := []string{}
	for _, scopeGlob := range scopeGlobs {
		scopeRoots, _ := filepath.Glob(scopeGlob)
		for _, scopeRoot := range scopeRoots {
			platformRoots, _ := filepath.Glob(filepath.Join(scopeRoot, "inbox", "*"))
			for _, platformRoot := range platformRoots {
				conversationRoots, _ := filepath.Glob(filepath.Join(platformRoot, "*"))
				for _, conversationRoot := range conversationRoots {
					if information, errorValue := os.Stat(conversationRoot); errorValue == nil && information.IsDir() {
						conversationDirectories = append(conversationDirectories, conversationRoot)
					}
				}
			}
		}
	}
	return conversationDirectories
}

func cleanupConversationAttachments(conversationDirectory string, workspaceRoot string, apply bool, report *attachmentCleanupReport) {
	files := attachmentFilesByDepth(conversationDirectory)
	if len(files) == 0 {
		return
	}
	report.ConversationsSeen++

	keptNameByContentHash := map[[32]byte]string{}
	usedNames := map[string]bool{}

	for _, filePath := range files {
		content, errorValue := os.ReadFile(filePath)
		if errorValue != nil {
			continue
		}
		contentHash := sha256.Sum256(content)
		isDirectChild := filepath.Dir(filePath) == conversationDirectory

		if _, isDuplicate := keptNameByContentHash[contentHash]; isDuplicate {
			report.DuplicatesRemoved++
			report.Actions = append(report.Actions, attachmentCleanupAction{Action: "delete", From: workspaceRelativePath(workspaceRoot, filePath)})
			if apply {
				_ = os.Remove(filePath)
			}
			continue
		}

		if isDirectChild {
			keptNameByContentHash[contentHash] = filepath.Base(filePath)
			usedNames[filepath.Base(filePath)] = true
			continue
		}

		targetName := uniqueAttachmentName(conversationDirectory, usedNames, filepath.Base(filePath))
		targetPath := filepath.Join(conversationDirectory, targetName)
		report.FilesFlattened++
		report.Actions = append(report.Actions, attachmentCleanupAction{
			Action: "flatten",
			From:   workspaceRelativePath(workspaceRoot, filePath),
			To:     workspaceRelativePath(workspaceRoot, targetPath),
		})
		if apply {
			_ = os.Rename(filePath, targetPath)
		}
		keptNameByContentHash[contentHash] = targetName
		usedNames[targetName] = true
	}

	if apply {
		report.EmptyFoldersRemoved += removeEmptyChildDirectories(conversationDirectory)
	}
}

func attachmentFilesByDepth(conversationDirectory string) []string {
	files := []string{}
	_ = filepath.Walk(conversationDirectory, func(path string, information os.FileInfo, errorValue error) error {
		if errorValue != nil || information.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	sort.Slice(files, func(leftIndex int, rightIndex int) bool {
		leftDepth := strings.Count(files[leftIndex], string(os.PathSeparator))
		rightDepth := strings.Count(files[rightIndex], string(os.PathSeparator))
		if leftDepth != rightDepth {
			return leftDepth < rightDepth
		}
		return files[leftIndex] < files[rightIndex]
	})
	return files
}

func uniqueAttachmentName(directory string, usedNames map[string]bool, name string) string {
	extension := filepath.Ext(name)
	stem := strings.TrimSuffix(name, extension)
	for index := 1; ; index++ {
		candidate := name
		if index > 1 {
			candidate = stem + "-" + strconv.Itoa(index) + extension
		}
		if usedNames[candidate] {
			continue
		}
		if _, errorValue := os.Stat(filepath.Join(directory, candidate)); errorValue == nil {
			continue
		}
		return candidate
	}
}

func removeEmptyChildDirectories(root string) int {
	removed := 0
	directories := []string{}
	_ = filepath.Walk(root, func(path string, information os.FileInfo, errorValue error) error {
		if errorValue != nil || !information.IsDir() || path == root {
			return nil
		}
		directories = append(directories, path)
		return nil
	})
	sort.Slice(directories, func(leftIndex int, rightIndex int) bool {
		return len(directories[leftIndex]) > len(directories[rightIndex])
	})
	for _, directory := range directories {
		entries, errorValue := os.ReadDir(directory)
		if errorValue == nil && len(entries) == 0 {
			if os.Remove(directory) == nil {
				removed++
			}
		}
	}
	return removed
}

func workspaceRelativePath(workspaceRoot string, path string) string {
	relativePath, errorValue := filepath.Rel(workspaceRoot, path)
	if errorValue != nil {
		return path
	}
	return filepath.ToSlash(relativePath)
}

type attachmentMigrationAction struct {
	Action string `json:"action"`
	From   string `json:"from"`
	To     string `json:"to,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type attachmentMigrationReport struct {
	Applied             bool                        `json:"applied"`
	DirectoriesSeen     int                         `json:"directoriesSeen"`
	FilesMoved          int                         `json:"filesMoved"`
	DuplicatesRemoved   int                         `json:"duplicatesRemoved"`
	EmptyFoldersRemoved int                         `json:"emptyFoldersRemoved"`
	Actions             []attachmentMigrationAction `json:"actions"`
}

func (service *Service) handleAttachmentMigration(responseWriter http.ResponseWriter, request *http.Request, apply bool) {
	report := service.migrateAttachmentInboxes(request.Context(), apply)
	service.writeJSON(responseWriter, report)
}

func (service *Service) migrateAttachmentInboxes(ctx context.Context, apply bool) attachmentMigrationReport {
	report := attachmentMigrationReport{Applied: apply, Actions: []attachmentMigrationAction{}}
	workspaceRoot := service.Configuration.BlueclawWorkspacePath
	for _, scopeGlob := range []string{
		filepath.Join(workspaceRoot, "circles", "*"),
		filepath.Join(workspaceRoot, "private", "people", "*"),
	} {
		scopeRoots, _ := filepath.Glob(scopeGlob)
		for _, scopeRoot := range scopeRoots {
			service.migrateScopeMattermostInbox(ctx, scopeRoot, workspaceRoot, apply, &report)
		}
	}
	return report
}

func (service *Service) migrateScopeMattermostInbox(ctx context.Context, scopeRoot, workspaceRoot string, apply bool, report *attachmentMigrationReport) {
	platformRoot := filepath.Join(scopeRoot, "inbox", "mattermost")
	conversationDirs, _ := filepath.Glob(filepath.Join(platformRoot, "*"))
	channelNameCache := map[string]string{}
	botToken := readTrimmedFile(service.Configuration.MattermostBotTokenPath)
	for _, sourceDir := range conversationDirs {
		information, errorValue := os.Stat(sourceDir)
		if errorValue != nil || !information.IsDir() {
			continue
		}
		dirName := filepath.Base(sourceDir)
		channelID := mattermostChannelIDFromInboxDirName(dirName)
		if channelID == "" {
			continue
		}
		report.DirectoriesSeen++
		if _, isCached := channelNameCache[channelID]; !isCached {
			channelNameCache[channelID] = service.fetchMattermostChannelSafeName(ctx, botToken, channelID)
		}
		channelSafeName := channelNameCache[channelID]
		if channelSafeName == "" {
			report.Actions = append(report.Actions, attachmentMigrationAction{Action: "skip", From: workspaceRelativePath(workspaceRoot, sourceDir), Reason: "channel not found"})
			continue
		}
		targetDir := filepath.Join(platformRoot, channelSafeName)
		if targetDir == sourceDir {
			continue
		}
		if apply {
			_ = os.MkdirAll(targetDir, 0o755)
		}
		mergeInboxDirectory(sourceDir, targetDir, workspaceRoot, apply, report)
	}
	if apply {
		report.EmptyFoldersRemoved += removeEmptyChildDirectories(platformRoot)
	}
}

func mattermostChannelIDFromInboxDirName(dirName string) string {
	const mattermostIDLength = 26
	if after, ok := strings.CutPrefix(dirName, "thread-"); ok {
		if len(after) >= mattermostIDLength+1 && after[mattermostIDLength] == '-' {
			return after[:mattermostIDLength]
		}
	}
	if after, ok := strings.CutPrefix(dirName, "channel-"); ok {
		if len(after) == mattermostIDLength {
			return after
		}
	}
	return ""
}

func (service *Service) fetchMattermostChannelSafeName(ctx context.Context, botToken string, channelID string) string {
	var channel struct {
		Name string `json:"name"`
	}
	errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/channels/"+url.PathEscape(channelID), botToken, nil, &channel)
	if errorValue != nil || strings.TrimSpace(channel.Name) == "" {
		return ""
	}
	return safeInboxDirSegment(channel.Name)
}

func safeInboxDirSegment(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	result := strings.Builder{}
	for _, character := range name {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-' || character == '_' {
			result.WriteRune(character)
		} else {
			result.WriteRune('-')
		}
	}
	return strings.Trim(result.String(), "-_")
}

func mergeInboxDirectory(sourceDir, targetDir, workspaceRoot string, apply bool, report *attachmentMigrationReport) {
	entries, _ := os.ReadDir(sourceDir)
	usedNames := map[string]bool{}
	existingEntries, _ := os.ReadDir(targetDir)
	keptByHash := map[[32]byte]string{}
	for _, entry := range existingEntries {
		if entry.IsDir() {
			continue
		}
		usedNames[entry.Name()] = true
		content, errorValue := os.ReadFile(filepath.Join(targetDir, entry.Name()))
		if errorValue == nil {
			keptByHash[sha256.Sum256(content)] = entry.Name()
		}
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		sourcePath := filepath.Join(sourceDir, entry.Name())
		content, errorValue := os.ReadFile(sourcePath)
		if errorValue != nil {
			continue
		}
		contentHash := sha256.Sum256(content)
		if _, isDuplicate := keptByHash[contentHash]; isDuplicate {
			report.DuplicatesRemoved++
			report.Actions = append(report.Actions, attachmentMigrationAction{Action: "delete", From: workspaceRelativePath(workspaceRoot, sourcePath)})
			if apply {
				_ = os.Remove(sourcePath)
			}
			continue
		}
		targetName := uniqueAttachmentName(targetDir, usedNames, entry.Name())
		targetPath := filepath.Join(targetDir, targetName)
		report.FilesMoved++
		report.Actions = append(report.Actions, attachmentMigrationAction{Action: "move", From: workspaceRelativePath(workspaceRoot, sourcePath), To: workspaceRelativePath(workspaceRoot, targetPath)})
		if apply {
			_ = os.Rename(sourcePath, targetPath)
		}
		keptByHash[contentHash] = targetName
		usedNames[targetName] = true
	}
}
