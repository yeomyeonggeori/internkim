package companion

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	MountResourceScopeKind = "companion_mount"
	MountModeReadWrite     = "read_write"
	MountStatusOnline      = "online"
	MountStatusPaused      = "paused"
	MountStatusRevoked     = "revoked"
)

type DirectoryPickRequest struct {
	Title string `json:"title,omitempty"`
}

type PickedDirectory struct {
	Path string `json:"path"`
}

type MountStore struct {
	mutex sync.Mutex
	path  string
	items map[string]*MountRecord
}

type MountRecord struct {
	MountID     string    `json:"mountID"`
	DisplayName string    `json:"displayName"`
	GuestPath   string    `json:"guestPath"`
	Mode        string    `json:"mode"`
	Status      string    `json:"status"`
	LocalPath   string    `json:"localPath"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type MountSnapshot struct {
	MountID     string    `json:"mountID"`
	DisplayName string    `json:"displayName"`
	GuestPath   string    `json:"guestPath"`
	Mode        string    `json:"mode"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	LastSeenAt  time.Time `json:"lastSeenAt"`
}

type MountDirectoryEntry struct {
	Name         string    `json:"name"`
	RelativePath string    `json:"relativePath"`
	Kind         string    `json:"kind"`
	SizeBytes    int64     `json:"sizeBytes"`
	ModifiedAt   time.Time `json:"modifiedAt"`
}

type MountFileInformation struct {
	RelativePath string    `json:"relativePath"`
	Kind         string    `json:"kind"`
	SizeBytes    int64     `json:"sizeBytes"`
	ModifiedAt   time.Time `json:"modifiedAt"`
}

var safeMountNamePattern = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func NewMountStore(path string) *MountStore {
	store := &MountStore{path: path, items: map[string]*MountRecord{}}
	store.load()
	return store
}

func (store *MountStore) Create(localPath string, displayName string) (MountSnapshot, error) {
	rootPath, errorValue := validateMountRoot(localPath)
	if errorValue != nil {
		return MountSnapshot{}, errorValue
	}
	now := time.Now().UTC()
	mountID := newMountID()
	record := &MountRecord{
		MountID:     mountID,
		DisplayName: firstNonEmpty(strings.TrimSpace(displayName), filepath.Base(rootPath)),
		GuestPath:   mountGuestPath(firstNonEmpty(strings.TrimSpace(displayName), filepath.Base(rootPath)), mountID),
		Mode:        MountModeReadWrite,
		Status:      MountStatusOnline,
		LocalPath:   rootPath,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	store.mutex.Lock()
	store.items[mountID] = record
	store.mutex.Unlock()
	return record.snapshot(), store.save()
}

func (store *MountStore) List() []MountSnapshot {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	mounts := []MountSnapshot{}
	for _, record := range store.items {
		if record.Status == MountStatusRevoked {
			continue
		}
		mounts = append(mounts, record.snapshot())
	}
	sort.Slice(mounts, func(leftIndex int, rightIndex int) bool {
		return mounts[leftIndex].GuestPath < mounts[rightIndex].GuestPath
	})
	return mounts
}

func (store *MountStore) Revoke(mountID string) (MountSnapshot, error) {
	record, errorValue := store.record(mountID)
	if errorValue != nil {
		return MountSnapshot{}, errorValue
	}
	store.mutex.Lock()
	record.Status = MountStatusRevoked
	record.UpdatedAt = time.Now().UTC()
	store.mutex.Unlock()
	return record.snapshot(), store.save()
}

func (store *MountStore) Pause(mountID string) (MountSnapshot, error) {
	record, errorValue := store.record(mountID)
	if errorValue != nil {
		return MountSnapshot{}, errorValue
	}
	store.mutex.Lock()
	record.Status = MountStatusPaused
	record.UpdatedAt = time.Now().UTC()
	store.mutex.Unlock()
	return record.snapshot(), store.save()
}

func (store *MountStore) Resume(mountID string) (MountSnapshot, error) {
	record, errorValue := store.record(mountID)
	if errorValue != nil {
		return MountSnapshot{}, errorValue
	}
	store.mutex.Lock()
	record.Status = MountStatusOnline
	record.UpdatedAt = time.Now().UTC()
	store.mutex.Unlock()
	return record.snapshot(), store.save()
}

func (store *MountStore) Status(mountID string) (MountSnapshot, error) {
	record, errorValue := store.record(mountID)
	if errorValue != nil {
		return MountSnapshot{}, errorValue
	}
	return record.snapshot(), nil
}

func (store *MountStore) Stat(mountID string, relativePath string) (MountFileInformation, error) {
	path, cleanRelativePath, errorValue := store.resolvePath(mountID, relativePath, false)
	if errorValue != nil {
		return MountFileInformation{}, errorValue
	}
	information, errorValue := os.Stat(path)
	if errorValue != nil {
		return MountFileInformation{}, errorValue
	}
	return mountFileInformation(cleanRelativePath, information), nil
}

func (store *MountStore) ListDirectory(mountID string, relativePath string) ([]MountDirectoryEntry, error) {
	path, _, errorValue := store.resolvePath(mountID, relativePath, false)
	if errorValue != nil {
		return nil, errorValue
	}
	entries, errorValue := os.ReadDir(path)
	if errorValue != nil {
		return nil, errorValue
	}
	result := []MountDirectoryEntry{}
	for _, entry := range entries {
		information, errorValue := entry.Info()
		if errorValue != nil {
			continue
		}
		result = append(result, mountDirectoryEntry(relativePath, entry.Name(), information))
	}
	return result, nil
}

func (store *MountStore) ReadFile(mountID string, relativePath string) (map[string]any, error) {
	path, cleanRelativePath, errorValue := store.resolvePath(mountID, relativePath, false)
	if errorValue != nil {
		return nil, errorValue
	}
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return nil, errorValue
	}
	return map[string]any{
		"relativePath":  cleanRelativePath,
		"sizeBytes":     len(document),
		"contentBase64": base64.StdEncoding.EncodeToString(document),
		"encoding":      "base64",
	}, nil
}

func (store *MountStore) WriteFile(mountID string, relativePath string, document []byte) (MountFileInformation, error) {
	path, cleanRelativePath, errorValue := store.resolvePath(mountID, relativePath, true)
	if errorValue != nil {
		return MountFileInformation{}, errorValue
	}
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return MountFileInformation{}, errorValue
	}
	if errorValue := os.WriteFile(path, document, 0o600); errorValue != nil {
		return MountFileInformation{}, errorValue
	}
	information, errorValue := os.Stat(path)
	if errorValue != nil {
		return MountFileInformation{}, errorValue
	}
	return mountFileInformation(cleanRelativePath, information), nil
}

func (store *MountStore) MakeDirectory(mountID string, relativePath string) (MountFileInformation, error) {
	path, cleanRelativePath, errorValue := store.resolvePath(mountID, relativePath, true)
	if errorValue != nil {
		return MountFileInformation{}, errorValue
	}
	if errorValue := os.MkdirAll(path, 0o700); errorValue != nil {
		return MountFileInformation{}, errorValue
	}
	information, errorValue := os.Stat(path)
	if errorValue != nil {
		return MountFileInformation{}, errorValue
	}
	return mountFileInformation(cleanRelativePath, information), nil
}

func (store *MountStore) Rename(mountID string, fromRelativePath string, toRelativePath string) (MountFileInformation, error) {
	if cleanPath, errorValue := cleanMountRelativePath(fromRelativePath); errorValue != nil || cleanPath == "." {
		return MountFileInformation{}, errors.New("mount root cannot be renamed")
	}
	fromPath, _, errorValue := store.resolvePath(mountID, fromRelativePath, false)
	if errorValue != nil {
		return MountFileInformation{}, errorValue
	}
	toPath, cleanToRelativePath, errorValue := store.resolvePath(mountID, toRelativePath, true)
	if errorValue != nil {
		return MountFileInformation{}, errorValue
	}
	if errorValue := os.MkdirAll(filepath.Dir(toPath), 0o700); errorValue != nil {
		return MountFileInformation{}, errorValue
	}
	if errorValue := os.Rename(fromPath, toPath); errorValue != nil {
		return MountFileInformation{}, errorValue
	}
	information, errorValue := os.Stat(toPath)
	if errorValue != nil {
		return MountFileInformation{}, errorValue
	}
	return mountFileInformation(cleanToRelativePath, information), nil
}

func (store *MountStore) Delete(mountID string, relativePath string, recursive bool) (map[string]any, error) {
	path, cleanRelativePath, errorValue := store.resolvePath(mountID, relativePath, false)
	if errorValue != nil {
		return nil, errorValue
	}
	if cleanRelativePath == "." {
		return nil, errors.New("mount root cannot be deleted")
	}
	if recursive {
		errorValue = os.RemoveAll(path)
	} else {
		errorValue = os.Remove(path)
	}
	if errorValue != nil {
		return nil, errorValue
	}
	return map[string]any{"relativePath": cleanRelativePath, "deleted": true}, nil
}

func (store *MountStore) Truncate(mountID string, relativePath string, sizeBytes int64) (MountFileInformation, error) {
	if sizeBytes < 0 {
		return MountFileInformation{}, errors.New("sizeBytes must be non-negative")
	}
	path, cleanRelativePath, errorValue := store.resolvePath(mountID, relativePath, false)
	if errorValue != nil {
		return MountFileInformation{}, errorValue
	}
	if errorValue := os.Truncate(path, sizeBytes); errorValue != nil {
		return MountFileInformation{}, errorValue
	}
	information, errorValue := os.Stat(path)
	if errorValue != nil {
		return MountFileInformation{}, errorValue
	}
	return mountFileInformation(cleanRelativePath, information), nil
}

func (store *MountStore) ChangeMode(mountID string, relativePath string, mode os.FileMode) (MountFileInformation, error) {
	path, cleanRelativePath, errorValue := store.resolvePath(mountID, relativePath, false)
	if errorValue != nil {
		return MountFileInformation{}, errorValue
	}
	limitedMode, errorValue := limitedMountFileMode(mode)
	if errorValue != nil {
		return MountFileInformation{}, errorValue
	}
	if errorValue := os.Chmod(path, limitedMode); errorValue != nil {
		return MountFileInformation{}, errorValue
	}
	information, errorValue := os.Stat(path)
	if errorValue != nil {
		return MountFileInformation{}, errorValue
	}
	return mountFileInformation(cleanRelativePath, information), nil
}

func (store *MountStore) ChangedSince(mountID string, relativePath string, since time.Time) ([]MountDirectoryEntry, error) {
	path, cleanRelativePath, errorValue := store.resolvePath(mountID, relativePath, false)
	if errorValue != nil {
		return nil, errorValue
	}
	entries := []MountDirectoryEntry{}
	errorValue = filepath.WalkDir(path, func(currentPath string, directoryEntry fs.DirEntry, walkError error) error {
		if walkError != nil {
			return nil
		}
		information, errorValue := directoryEntry.Info()
		if errorValue != nil || !information.ModTime().After(since) {
			return nil
		}
		relativeEntryPath, errorValue := filepath.Rel(path, currentPath)
		if errorValue != nil || relativeEntryPath == "." {
			relativeEntryPath = directoryEntry.Name()
		}
		parentPath := cleanRelativePath
		if parentPath == "." {
			parentPath = ""
		}
		entries = append(entries, mountDirectoryEntry(parentPath, relativeEntryPath, information))
		return nil
	})
	return entries, errorValue
}

func (store *MountStore) record(mountID string) (*MountRecord, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	record := store.items[strings.TrimSpace(mountID)]
	if record == nil || record.Status == MountStatusRevoked {
		return nil, errors.New("mount is unavailable")
	}
	return record, nil
}

func (store *MountStore) resolvePath(mountID string, relativePath string, allowMissingLeaf bool) (string, string, error) {
	record, errorValue := store.record(mountID)
	if errorValue != nil {
		return "", "", errorValue
	}
	if record.Status != MountStatusOnline {
		return "", "", errors.New("mount is not online")
	}
	cleanRelativePath, errorValue := cleanMountRelativePath(relativePath)
	if errorValue != nil {
		return "", "", errorValue
	}
	resolvedPath, errorValue := resolveContainedMountPath(record.LocalPath, cleanRelativePath, allowMissingLeaf)
	return resolvedPath, cleanRelativePath, errorValue
}

func (store *MountStore) save() error {
	if strings.TrimSpace(store.path) == "" {
		return nil
	}
	store.mutex.Lock()
	records := []*MountRecord{}
	for _, record := range store.items {
		records = append(records, record)
	}
	store.mutex.Unlock()
	document, errorValue := json.MarshalIndent(map[string]any{"mounts": records}, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(filepath.Dir(store.path), 0o700); errorValue != nil {
		return errorValue
	}
	temporaryPath := store.path + ".tmp"
	if errorValue := os.WriteFile(temporaryPath, document, 0o600); errorValue != nil {
		return errorValue
	}
	return os.Rename(temporaryPath, store.path)
}

func (store *MountStore) load() {
	if strings.TrimSpace(store.path) == "" {
		return
	}
	document, errorValue := os.ReadFile(store.path)
	if errorValue != nil {
		return
	}
	var state struct {
		Mounts []*MountRecord `json:"mounts"`
	}
	if errorValue := json.Unmarshal(document, &state); errorValue != nil {
		return
	}
	for _, record := range state.Mounts {
		if record != nil && record.MountID != "" {
			store.items[record.MountID] = record
		}
	}
}

func (record MountRecord) snapshot() MountSnapshot {
	return MountSnapshot{
		MountID:     record.MountID,
		DisplayName: record.DisplayName,
		GuestPath:   record.GuestPath,
		Mode:        record.Mode,
		Status:      record.Status,
		CreatedAt:   record.CreatedAt,
		LastSeenAt:  record.UpdatedAt,
	}
}

func validateMountRoot(localPath string) (string, error) {
	trimmedPath := strings.TrimSpace(localPath)
	if trimmedPath == "" {
		return "", errors.New("directory path is required")
	}
	rootPath, errorValue := filepath.Abs(trimmedPath)
	if errorValue != nil {
		return "", errorValue
	}
	resolvedRootPath, errorValue := filepath.EvalSymlinks(rootPath)
	if errorValue != nil {
		return "", errorValue
	}
	information, errorValue := os.Stat(resolvedRootPath)
	if errorValue != nil {
		return "", errorValue
	}
	if !information.IsDir() {
		return "", errors.New("mount root must be a directory")
	}
	return resolvedRootPath, nil
}

func cleanMountRelativePath(relativePath string) (string, error) {
	trimmedPath := strings.TrimSpace(relativePath)
	if trimmedPath == "" || trimmedPath == "." {
		return ".", nil
	}
	if filepath.IsAbs(trimmedPath) {
		return "", errors.New("mount path must be relative")
	}
	cleanPath := filepath.Clean(trimmedPath)
	if cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return "", errors.New("mount path must stay inside the mounted directory")
	}
	return cleanPath, nil
}

func resolveContainedMountPath(rootPath string, relativePath string, allowMissingLeaf bool) (string, error) {
	candidatePath := filepath.Join(rootPath, relativePath)
	resolvePath := candidatePath
	if allowMissingLeaf {
		resolvedPath, errorValue := resolveContainedMissingPath(rootPath, candidatePath)
		if errorValue != nil {
			return "", errorValue
		}
		return resolvedPath, nil
	}
	resolvedPath, errorValue := filepath.EvalSymlinks(resolvePath)
	if errorValue != nil {
		return "", errorValue
	}
	cleanRootPath, errorValue := filepath.EvalSymlinks(rootPath)
	if errorValue != nil {
		return "", errorValue
	}
	if !isPathInside(cleanRootPath, resolvedPath) {
		return "", errors.New("mount path escapes the mounted directory")
	}
	return resolvedPath, nil
}

func resolveContainedMissingPath(rootPath string, candidatePath string) (string, error) {
	cleanRootPath, errorValue := filepath.EvalSymlinks(rootPath)
	if errorValue != nil {
		return "", errorValue
	}
	existingPath := candidatePath
	missingParts := []string{}
	for {
		resolvedPath, errorValue := filepath.EvalSymlinks(existingPath)
		if errorValue == nil {
			if !isPathInside(cleanRootPath, resolvedPath) {
				return "", errors.New("mount path escapes the mounted directory")
			}
			for index := len(missingParts) - 1; index >= 0; index-- {
				resolvedPath = filepath.Join(resolvedPath, missingParts[index])
			}
			return resolvedPath, nil
		}
		parentPath := filepath.Dir(existingPath)
		if parentPath == existingPath {
			return "", errorValue
		}
		missingParts = append(missingParts, filepath.Base(existingPath))
		existingPath = parentPath
	}
}

func isPathInside(rootPath string, path string) bool {
	relativePath, errorValue := filepath.Rel(rootPath, path)
	return errorValue == nil && relativePath != ".." && !strings.HasPrefix(relativePath, ".."+string(filepath.Separator))
}

func mountGuestPath(displayName string, mountID string) string {
	safeName := strings.Trim(safeMountNamePattern.ReplaceAllString(displayName, "-"), "-")
	if safeName == "" {
		safeName = "mounted-folder"
	}
	shortMountID := mountID
	if len(shortMountID) > 8 {
		shortMountID = shortMountID[:8]
	}
	return "/workspace/mounts/" + safeName + "-" + shortMountID
}

func newMountID() string {
	document := make([]byte, 16)
	if _, errorValue := rand.Read(document); errorValue != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(document)
}

func mountDirectoryEntry(parentRelativePath string, name string, information fs.FileInfo) MountDirectoryEntry {
	relativePath := name
	if strings.TrimSpace(parentRelativePath) != "" && parentRelativePath != "." {
		relativePath = filepath.ToSlash(filepath.Join(parentRelativePath, name))
	}
	return MountDirectoryEntry{
		Name:         name,
		RelativePath: relativePath,
		Kind:         mountFileKind(information),
		SizeBytes:    information.Size(),
		ModifiedAt:   information.ModTime().UTC(),
	}
}

func mountFileInformation(relativePath string, information fs.FileInfo) MountFileInformation {
	return MountFileInformation{
		RelativePath: filepath.ToSlash(relativePath),
		Kind:         mountFileKind(information),
		SizeBytes:    information.Size(),
		ModifiedAt:   information.ModTime().UTC(),
	}
}

func mountFileKind(information fs.FileInfo) string {
	if information.IsDir() {
		return "directory"
	}
	if information.Mode()&os.ModeSymlink != 0 {
		return "symlink"
	}
	return "file"
}

func limitedMountFileMode(mode os.FileMode) (os.FileMode, error) {
	switch mode.Perm() {
	case 0o600, 0o640, 0o644, 0o700, 0o750, 0o755:
		return mode.Perm(), nil
	default:
		return 0, errors.New("mode must be one of 0600, 0640, 0644, 0700, 0750, or 0755")
	}
}
