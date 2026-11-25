package filestream

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/hostbackup"
)

const catchUpMargin = time.Minute

func (state *following) catchUp(ctx context.Context) error {
	if state.entriesKept > fullEntryAfter {
		return state.startFullEntry(ctx)
	}
	entries, errorValue := state.Journal.sinceNewestFull(ctx, state.Keyring, nil)
	if errors.Is(errorValue, ErrNoFullEntry) {
		return state.startFullEntry(ctx)
	}
	if errorValue != nil {
		return errorValue
	}
	for _, entry := range entries {
		for _, identifier := range entry.Chunks {
			state.referenced[identifier] = true
		}
	}
	state.lastFull = entries[0].CreatedAt
	changed, errorValue := changedSince(state.Roots, entries[len(entries)-1].CreatedAt.Add(-catchUpMargin))
	if errorValue != nil {
		return errorValue
	}
	if len(changed) > 0 {
		if errorValue := state.keepChanges(ctx, changed); errorValue != nil {
			return errorValue
		}
	}
	if state.isAFullEntryDue() {
		return state.startFullEntry(ctx)
	}
	return nil
}

func changedSince(roots []hostbackup.FileRoot, since time.Time) (map[string]Change, error) {
	changed := map[string]Change{}
	for _, root := range roots {
		errorValue := filepath.WalkDir(root.Path, func(filePath string, entry fs.DirEntry, walkError error) error {
			if errors.Is(walkError, fs.ErrNotExist) {
				return nil
			}
			if walkError != nil {
				return walkError
			}
			relative, errorValue := filepath.Rel(root.Path, filePath)
			if errorValue != nil || relative == "." {
				return errorValue
			}
			relative = filepath.ToSlash(relative)
			if root.Excludes(relative) {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			information, errorValue := entry.Info()
			if errors.Is(errorValue, fs.ErrNotExist) {
				return nil
			}
			if errorValue != nil {
				return errorValue
			}
			if lastChangeOf(information).After(since) {
				change := Change{Root: root, Relative: relative}
				changed[change.archivedName()] = change
			}
			return nil
		})
		if errorValue != nil && !errors.Is(errorValue, fs.ErrNotExist) {
			return nil, errorValue
		}
	}
	return changed, nil
}
