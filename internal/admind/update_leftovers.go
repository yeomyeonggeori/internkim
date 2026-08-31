package admind

import (
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const blueclawArtifactRetainCount = 2

// An update in flight owns its directory, so one that has not been touched in
// this long belongs to a job that is no longer running.
const abandonedUpdateAge = 6 * time.Hour

func (service *Service) releaseStagingPath(jobID string) string {
	return filepath.Join(service.Configuration.StateDirectory, "release-updates", "staging", jobID)
}

func (service *Service) forgetReleaseStaging(jobID string) {
	if errorValue := os.RemoveAll(service.releaseStagingPath(jobID)); errorValue != nil {
		log.Printf("the update staged for %s could not be cleared: %v", jobID, errorValue)
	}
}

func (service *Service) sweepUpdateLeftovers() {
	stateDirectory := service.Configuration.StateDirectory
	if stateDirectory == "" {
		return
	}
	for _, directory := range []string{
		filepath.Join(stateDirectory, "release-updates", "staging"),
		filepath.Join(stateDirectory, "blueclaw-updates", "staging"),
		filepath.Join(stateDirectory, "blueclaw-updates", "uploads"),
	} {
		removeAbandonedUpdateDirectories(directory, time.Now())
	}
	keepNewestBlueclawArtifacts(
		filepath.Join(stateDirectory, "blueclaw-updates", "artifacts"),
		blueclawArtifactRetainCount,
	)
}

func removeAbandonedUpdateDirectories(directory string, now time.Time) {
	entries, errorValue := os.ReadDir(directory)
	if errorValue != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		information, errorValue := entry.Info()
		if errorValue != nil || now.Sub(information.ModTime()) < abandonedUpdateAge {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		if errorValue := os.RemoveAll(path); errorValue != nil {
			log.Printf("an update left at %s could not be cleared: %v", path, errorValue)
			continue
		}
		log.Printf("cleared an update nobody finished at %s", path)
	}
}

func keepNewestBlueclawArtifacts(directory string, retainCount int) {
	entries, errorValue := os.ReadDir(directory)
	if errorValue != nil {
		return
	}
	kept := []os.DirEntry{}
	for _, entry := range entries {
		if entry.IsDir() {
			kept = append(kept, entry)
		}
	}
	if len(kept) <= retainCount {
		return
	}
	sort.Slice(kept, func(left int, right int) bool {
		leftInformation, leftError := kept[left].Info()
		rightInformation, rightError := kept[right].Info()
		if leftError != nil || rightError != nil {
			return kept[left].Name() > kept[right].Name()
		}
		return leftInformation.ModTime().After(rightInformation.ModTime())
	})
	for _, entry := range kept[retainCount:] {
		path := filepath.Join(directory, entry.Name())
		if errorValue := os.RemoveAll(path); errorValue != nil {
			log.Printf("an older payload at %s could not be cleared: %v", path, errorValue)
			continue
		}
		log.Printf("cleared an older payload at %s", path)
	}
}
