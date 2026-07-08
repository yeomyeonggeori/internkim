package admind

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const scheduledBackupDirectory = "/root/.internkim/backups"
const scheduledBackupPassphrasePath = "/root/.internkim/secrets/backup-passphrase"
const scheduledBackupInterval = 24 * time.Hour
const scheduledBackupRetainCount = 7

func (service *Service) startScheduledBackups(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if newestScheduledBackupAge(scheduledBackupDirectory) < scheduledBackupInterval {
					continue
				}
				if errorValue := service.runScheduledBackup(ctx); errorValue != nil {
					log.Printf("scheduled backup failed: %v", errorValue)
				}
			}
		}
	}()
}

func (service *Service) runScheduledBackup(ctx context.Context) error {
	passphrase, errorValue := readOrCreateBackupPassphrase(scheduledBackupPassphrasePath)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(scheduledBackupDirectory, 0o700); errorValue != nil {
		return errorValue
	}
	timestamp := time.Now().UTC().Format("20060102T150405Z")
	plainPath := filepath.Join(scheduledBackupDirectory, ".internkim-backup-"+timestamp+".tar.gz")
	encryptedPath := filepath.Join(scheduledBackupDirectory, "internkim-backup-"+timestamp+".ikbak")

	blueclawManifest, completeBlueclawBackup := service.prepareBlueclawBackup(ctx)
	defer completeBlueclawBackup()
	if _, errorValue := service.createPlainBackup(ctx, plainPath, blueclawManifest); errorValue != nil {
		_ = os.Remove(plainPath)
		return errorValue
	}
	if errorValue := encryptFile(plainPath, encryptedPath, passphrase); errorValue != nil {
		_ = os.Remove(plainPath)
		return errorValue
	}
	_ = os.Remove(plainPath)
	pruneScheduledBackups(scheduledBackupDirectory, scheduledBackupRetainCount)
	log.Printf("scheduled backup completed: %s", encryptedPath)
	return nil
}

func readOrCreateBackupPassphrase(passphrasePath string) (string, error) {
	document, errorValue := os.ReadFile(passphrasePath)
	if errorValue == nil && strings.TrimSpace(string(document)) != "" {
		return strings.TrimSpace(string(document)), nil
	}
	passphrase := randomHex(32)
	if errorValue := os.MkdirAll(filepath.Dir(passphrasePath), 0o700); errorValue != nil {
		return "", errorValue
	}
	if errorValue := os.WriteFile(passphrasePath, []byte(passphrase+"\n"), 0o600); errorValue != nil {
		return "", errorValue
	}
	return passphrase, nil
}

func newestScheduledBackupAge(backupDirectory string) time.Duration {
	newestModifiedAt := newestScheduledBackupTime(backupDirectory)
	if newestModifiedAt.IsZero() {
		return scheduledBackupInterval
	}
	return time.Since(newestModifiedAt)
}

func newestScheduledBackupTime(backupDirectory string) time.Time {
	entries, errorValue := os.ReadDir(backupDirectory)
	if errorValue != nil {
		return time.Time{}
	}
	newest := time.Time{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ikbak") {
			continue
		}
		information, errorValue := entry.Info()
		if errorValue != nil {
			continue
		}
		if information.ModTime().After(newest) {
			newest = information.ModTime()
		}
	}
	return newest
}

func pruneScheduledBackups(backupDirectory string, retainCount int) {
	entries, errorValue := os.ReadDir(backupDirectory)
	if errorValue != nil {
		return
	}
	bundleNames := []string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".ikbak") {
			bundleNames = append(bundleNames, entry.Name())
		}
	}
	sort.Strings(bundleNames)
	for len(bundleNames) > retainCount {
		_ = os.Remove(filepath.Join(backupDirectory, bundleNames[0]))
		bundleNames = bundleNames[1:]
	}
}
