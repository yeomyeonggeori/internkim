package admind

import (
	"crypto/sha256"
	"net/http"
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
	Applied             bool                      `json:"applied"`
	ConversationsSeen   int                       `json:"conversationsSeen"`
	DuplicatesRemoved   int                       `json:"duplicatesRemoved"`
	FilesFlattened      int                       `json:"filesFlattened"`
	EmptyFoldersRemoved int                       `json:"emptyFoldersRemoved"`
	Actions             []attachmentCleanupAction `json:"actions"`
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
