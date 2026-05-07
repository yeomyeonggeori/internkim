package fleet

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

type WorkspacePathChange struct {
	Path        string
	Mode        string
	ContentHash string
}

type WorkspaceChangeBundle struct {
	BundleID    string
	BaseHead    string
	ContentHash string
	Changes     []WorkspacePathChange
}

func NewWorkspaceChangeBundle(baseHead string, changes []WorkspacePathChange) WorkspaceChangeBundle {
	normalizedChanges := normalizeWorkspacePathChanges(changes)
	contentHash := calculateWorkspaceBundleContentHash(baseHead, normalizedChanges)
	return WorkspaceChangeBundle{
		BundleID:    "bundle-" + contentHash[:16],
		BaseHead:    baseHead,
		ContentHash: contentHash,
		Changes:     normalizedChanges,
	}
}

func normalizeWorkspacePathChanges(changes []WorkspacePathChange) []WorkspacePathChange {
	normalizedChanges := make([]WorkspacePathChange, 0, len(changes))
	for _, change := range changes {
		normalizedPath := normalizeWorkspacePath(change.Path)
		if normalizedPath == "" {
			continue
		}
		mode := change.Mode
		if mode == "" {
			mode = ClassifyWorkspacePath(normalizedPath)
		}
		normalizedChanges = append(normalizedChanges, WorkspacePathChange{
			Path:        normalizedPath,
			Mode:        mode,
			ContentHash: change.ContentHash,
		})
	}
	sort.Slice(normalizedChanges, func(leftIndex int, rightIndex int) bool {
		return normalizedChanges[leftIndex].Path < normalizedChanges[rightIndex].Path
	})
	return normalizedChanges
}

func calculateWorkspaceBundleContentHash(baseHead string, changes []WorkspacePathChange) string {
	document, _ := json.Marshal(struct {
		BaseHead string
		Changes  []WorkspacePathChange
	}{
		BaseHead: baseHead,
		Changes:  changes,
	})
	digest := sha256.Sum256(document)
	return hex.EncodeToString(digest[:])
}
