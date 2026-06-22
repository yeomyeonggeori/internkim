package admind

import (
	"io/fs"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const staffCircleGroupName = "bc_circle_staff"

func (service *Service) reconcileSiteSourcesToStaffCircle() {
	for _, site := range service.siteList() {
		if errorValue := service.migrateSiteSourceToStaffCircle(site); errorValue != nil {
			log.Printf("site source staff-circle migration: site %s: %v", site.SiteID, errorValue)
		}
	}
}

func (service *Service) migrateSiteSourceToStaffCircle(site *SiteRecord) error {
	siteID := strings.TrimSpace(site.SiteID)
	if siteID == "" {
		return nil
	}
	workspaceRoot := service.Configuration.BlueclawWorkspacePath
	targetDraftHostPath := filepath.Join(workspaceRoot, "circles", "staff", "sites", siteID, "draft")

	currentSourceHostPath := locateSiteSourceHostPath(workspaceRoot, siteID, targetDraftHostPath)
	if currentSourceHostPath == "" {
		return nil
	}

	if currentSourceHostPath != targetDraftHostPath {
		if errorValue := relocateDirectory(currentSourceHostPath, targetDraftHostPath); errorValue != nil {
			return errorValue
		}
		_ = os.Remove(filepath.Dir(currentSourceHostPath))
	}

	healStaffCirclePermissions(targetDraftHostPath)
	return service.recordStaffCircleSitePaths(site, siteID)
}

func locateSiteSourceHostPath(workspaceRoot string, siteID string, targetDraftHostPath string) string {
	candidates := []string{
		targetDraftHostPath,
		filepath.Join(workspaceRoot, "sites", siteID, "draft"),
		filepath.Join(workspaceRoot, "sites", siteID),
	}
	candidates = append(candidates, filepathGlob(filepath.Join(workspaceRoot, "private", "people", "*", "sites", siteID, "draft"))...)
	candidates = append(candidates, filepathGlob(filepath.Join(workspaceRoot, "private", "people", "*", "sites", siteID))...)
	for _, candidate := range candidates {
		if isExistingDirectory(candidate) {
			return candidate
		}
	}
	return ""
}

func relocateDirectory(sourcePath string, targetPath string) error {
	if errorValue := os.MkdirAll(filepath.Dir(targetPath), 0o770); errorValue != nil {
		return errorValue
	}
	if errorValue := os.Rename(sourcePath, targetPath); errorValue == nil {
		return nil
	}
	if errorValue := copyDirectory(sourcePath, targetPath); errorValue != nil {
		return errorValue
	}
	return os.RemoveAll(sourcePath)
}

func (service *Service) recordStaffCircleSitePaths(site *SiteRecord, siteID string) error {
	draftPath := siteDraftWorkspacePath(siteID, "")
	projectPath := siteProjectWorkspacePath(siteID)
	appPath := filepath.ToSlash(filepath.Join(draftPath, "app"))
	if site.WorkspacePath == projectPath && site.SourceWorkspacePath == draftPath && site.DraftPath == draftPath && site.AppWorkspacePath == appPath {
		return nil
	}
	site.WorkspacePath = projectPath
	site.SourceWorkspacePath = draftPath
	site.DraftPath = draftPath
	site.AppWorkspacePath = appPath
	return service.storeSite(site)
}

func healStaffCirclePermissions(rootPath string) {
	groupID, errorValue := lookupStaffCircleGroupID()
	if errorValue != nil {
		return
	}
	if directoryHasGroup(rootPath, groupID) {
		return
	}
	walkError := filepath.WalkDir(rootPath, func(path string, entry fs.DirEntry, iterationError error) error {
		if iterationError != nil {
			return nil
		}
		_ = os.Lchown(path, -1, groupID)
		if entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() {
			_ = os.Chmod(path, os.ModeSetgid|0o770)
			return nil
		}
		_ = os.Chmod(path, 0o660)
		return nil
	})
	if walkError != nil {
		log.Printf("site source staff-circle permission heal incomplete for %s: %v", rootPath, walkError)
	}
}

func lookupStaffCircleGroupID() (int, error) {
	group, errorValue := user.LookupGroup(staffCircleGroupName)
	if errorValue != nil {
		return -1, errorValue
	}
	return strconv.Atoi(group.Gid)
}

func directoryHasGroup(path string, groupID int) bool {
	information, errorValue := os.Stat(path)
	if errorValue != nil {
		return false
	}
	systemInformation, isStat := information.Sys().(*syscall.Stat_t)
	if !isStat {
		return false
	}
	return int(systemInformation.Gid) == groupID
}

func isExistingDirectory(path string) bool {
	information, errorValue := os.Stat(path)
	return errorValue == nil && information.IsDir()
}

func filepathGlob(pattern string) []string {
	matches, errorValue := filepath.Glob(pattern)
	if errorValue != nil {
		return nil
	}
	return matches
}
