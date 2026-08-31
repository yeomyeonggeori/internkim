package admind

import (
	"io/fs"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

const memberCircleGroupName = "bc_circle_member"

func (service *Service) reconcileSiteSourcesToMemberCircle() {
	for _, site := range service.siteList() {
		if errorValue := service.migrateSiteSourceToMemberCircle(site); errorValue != nil {
			log.Printf("site source member-circle migration: site %s: %v", site.SiteID, errorValue)
		}
	}
}

func (service *Service) migrateSiteSourceToMemberCircle(site *SiteRecord) error {
	pathsChanged, errorValue := service.moveSiteSourceToMemberCircle(site)
	if errorValue != nil || !pathsChanged {
		return errorValue
	}
	return service.storeSite(site)
}

func (service *Service) moveSiteSourceToMemberCircle(site *SiteRecord) (bool, error) {
	siteID := strings.TrimSpace(site.SiteID)
	if siteID == "" {
		return false, nil
	}
	workspaceRoot := service.Configuration.BlueclawWorkspacePath
	targetProjectHostPath := service.siteProjectStorageHostPath(siteID)
	targetDraftHostPath := filepath.Join(targetProjectHostPath, "draft")
	if errorValue := service.ensureSiteWorkspaceStoragePath(siteID); errorValue != nil {
		return false, errorValue
	}

	workspaceSourceHostPath := locateSiteSourceHostPath(workspaceRoot, site, targetDraftHostPath)
	if workspaceSourceHostPath != "" {
		if workspaceSourceHostPath != targetDraftHostPath {
			if errorValue := relocateDirectory(workspaceSourceHostPath, targetDraftHostPath); errorValue != nil {
				return false, errorValue
			}
			_ = os.Remove(filepath.Dir(workspaceSourceHostPath))
		}
		healMemberCirclePermissions(targetDraftHostPath)
		if errorValue := service.ensureSiteWorkspaceAlias(site); errorValue != nil {
			return false, errorValue
		}
		return setMemberCircleSitePaths(site, siteID), nil
	}

	ledgerSourceHostPath := service.siteSourceLedgerPath(siteID)
	if !directoryHasEntries(ledgerSourceHostPath) {
		if errorValue := service.ensureSiteWorkspaceAlias(site); errorValue != nil {
			return false, errorValue
		}
		return setMemberCircleSitePaths(site, siteID), nil
	}
	if !looksLikeSiteDraftDirectory(targetDraftHostPath) {
		if isExistingDirectory(targetDraftHostPath) {
			if errorValue := os.RemoveAll(targetDraftHostPath); errorValue != nil {
				return false, errorValue
			}
		}
		if errorValue := copyDirectoryIntoMemberCircle(ledgerSourceHostPath, targetDraftHostPath); errorValue != nil {
			return false, errorValue
		}
	}
	healMemberCirclePermissions(targetDraftHostPath)
	if errorValue := service.ensureSiteWorkspaceAlias(site); errorValue != nil {
		return false, errorValue
	}
	return setMemberCircleSitePaths(site, siteID), nil
}

func copyDirectoryIntoMemberCircle(sourcePath string, targetPath string) error {
	if errorValue := os.MkdirAll(filepath.Dir(targetPath), 0o770); errorValue != nil {
		return errorValue
	}
	return copyDirectory(sourcePath, targetPath)
}

func directoryHasEntries(path string) bool {
	entries, errorValue := os.ReadDir(path)
	return errorValue == nil && len(entries) > 0
}

func locateSiteSourceHostPath(workspaceRoot string, site *SiteRecord, targetDraftHostPath string) string {
	siteID := ""
	slug := ""
	if site != nil {
		siteID = strings.TrimSpace(site.SiteID)
		slug = site.Slug
	}
	candidates := []string{
		targetDraftHostPath,
		filepath.Join(workspaceRoot, "circles", "member", "sites", siteWorkspaceAliasName(siteID, slug), "draft"),
		filepath.Join(workspaceRoot, "sites", siteID, "draft"),
		filepath.Join(workspaceRoot, "sites", siteID),
		filepath.Join(workspaceRoot, "circles", "member", "sites", siteID, "draft"),
	}
	if memberSiteRoot := memberSiteDraftRootHostPath(workspaceRoot, siteID); memberSiteRoot != "" {
		candidates = append(candidates, memberSiteRoot)
	}
	candidates = append(candidates, filepathGlob(filepath.Join(workspaceRoot, "private", "people", "*", "sites", siteID, "draft"))...)
	candidates = append(candidates, filepathGlob(filepath.Join(workspaceRoot, "private", "people", "*", "sites", siteID))...)
	for _, candidate := range candidates {
		if looksLikeSiteDraftDirectory(candidate) {
			return candidate
		}
	}
	return ""
}

func memberSiteDraftRootHostPath(workspaceRoot string, siteID string) string {
	siteRoot := filepath.Join(workspaceRoot, "circles", "member", "sites", strings.TrimSpace(siteID))
	if !looksLikeSiteDraftDirectory(siteRoot) {
		return ""
	}
	return siteRoot
}

func looksLikeSiteDraftDirectory(path string) bool {
	return isExistingDirectory(filepath.Join(path, "app")) ||
		isExistingDirectory(filepath.Join(path, ".internkim")) ||
		isRegularFile(filepath.Join(path, "DESIGN.md"))
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

func setMemberCircleSitePaths(site *SiteRecord, siteID string) bool {
	draftPath := siteDraftWorkspacePath(siteID, site.Slug, "")
	projectPath := siteProjectAliasWorkspacePath(siteID, site.Slug)
	appPath := filepath.ToSlash(filepath.Join(draftPath, "app"))
	if site.WorkspacePath == projectPath && site.SourceWorkspacePath == draftPath && site.DraftPath == draftPath && site.AppWorkspacePath == appPath {
		return false
	}
	site.WorkspacePath = projectPath
	site.SourceWorkspacePath = draftPath
	site.DraftPath = draftPath
	site.AppWorkspacePath = appPath
	return true
}

func healMemberCirclePermissions(rootPath string) {
	groupID, errorValue := lookupMemberCircleGroupID()
	hasMemberCircleGroup := errorValue == nil
	walkError := filepath.WalkDir(rootPath, func(path string, entry fs.DirEntry, iterationError error) error {
		if iterationError != nil {
			return nil
		}
		if hasMemberCircleGroup {
			_ = os.Lchown(path, -1, groupID)
		}
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
		log.Printf("site source member-circle permission heal incomplete for %s: %v", rootPath, walkError)
	}
}

func ensureMemberCircleDirectory(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if errorValue := os.MkdirAll(path, 0o770); errorValue != nil {
		return errorValue
	}
	if groupID, errorValue := lookupMemberCircleGroupID(); errorValue == nil {
		if errorValue := os.Lchown(path, -1, groupID); errorValue != nil {
			return errorValue
		}
	}
	return os.Chmod(path, os.ModeSetgid|0o770)
}

func lookupMemberCircleGroupID() (int, error) {
	group, errorValue := user.LookupGroup(memberCircleGroupName)
	if errorValue != nil {
		return -1, errorValue
	}
	return strconv.Atoi(group.Gid)
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
