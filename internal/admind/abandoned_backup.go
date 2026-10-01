package admind

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

const abandonedBackupDirectory = "/root/.internkim/backups"

// The hourly image backup that wrote these is gone, and nothing else prunes
// them: one interrupted run left an 11 GB tarball behind on the Jetson.
func removeAbandonedBackupIntermediates(backupDirectory string) {
	entries, errorValue := os.ReadDir(backupDirectory)
	if errorValue != nil {
		return
	}
	for _, entry := range entries {
		if !isAbandonedBackupIntermediate(entry) {
			continue
		}
		path := filepath.Join(backupDirectory, entry.Name())
		if errorValue := os.Remove(path); errorValue != nil {
			log.Printf("abandoned backup %s could not be removed: %v", path, errorValue)
			continue
		}
		log.Printf("removed abandoned backup %s", path)
	}
}

func isAbandonedBackupIntermediate(entry os.DirEntry) bool {
	name := entry.Name()
	return !entry.IsDir() && strings.HasPrefix(name, ".internkim-backup-") && strings.HasSuffix(name, ".tar.gz")
}
