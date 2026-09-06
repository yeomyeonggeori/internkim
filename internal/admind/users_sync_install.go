package admind

import (
	"context"
	"log"
	"os"
	"time"

	blueclawruntime "gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

type installedFile struct {
	path     string
	contents string
	mode     os.FileMode
}

func usersSyncInstalledFiles() []installedFile {
	return usersSyncInstalledFilesForMode(false)
}

func usersSyncInstalledFilesForMode(runDirectly bool) []installedFile {
	files := []installedFile{
		{blueclawruntime.InternKimUsersSyncScriptPath, blueclawruntime.InternKimUsersSyncScript(), 0o755},
	}
	if runDirectly {
		return files
	}
	return append(files,
		installedFile{blueclawruntime.InternKimUsersSyncServicePath, blueclawruntime.InternKimUsersSyncServiceUnit(), 0o644},
		installedFile{blueclawruntime.InternKimUsersSyncTimerPath, blueclawruntime.InternKimUsersSyncTimerUnit(), 0o644},
	)
}

func (service *Service) keepUsersSyncInstalled(ctx context.Context) {
	rewritten, errorValue := writeFilesIfDifferent(usersSyncInstalledFilesForMode(service.Configuration.RunUsersSyncDirectly))
	if errorValue != nil {
		log.Printf("users sync install failed: %v", errorValue)
		return
	}
	if rewritten {
		log.Printf("users sync reinstalled from this release")
	}
	if service.Configuration.RunUsersSyncDirectly {
		service.reconcileBlueclawRosterWithTimeout(ctx)
		service.runUsersSyncDirectly(ctx)
		service.keepUsersSyncRunning(ctx)
		return
	}
	if !rewritten {
		return
	}
	service.runSystemControl(ctx, "daemon-reload")
	service.runSystemControl(ctx, "enable", "--now", "internkim-users-sync.timer")
	service.runSystemControl(ctx, "start", "internkim-users-sync.service")
}

func (service *Service) keepUsersSyncRunning(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			service.runUsersSyncDirectly(ctx)
		}
	}
}

func (service *Service) runUsersSyncDirectly(ctx context.Context) {
	service.usersSyncMutex.Lock()
	defer service.usersSyncMutex.Unlock()
	runContext, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if output, errorValue := service.runCommand(runContext, blueclawruntime.InternKimUsersSyncScriptPath, "--service-acl"); errorValue != nil {
		log.Printf("users sync direct run failed: %v: %s", errorValue, output)
	}
}

func writeFilesIfDifferent(files []installedFile) (bool, error) {
	rewritten := false
	for _, file := range files {
		changed, errorValue := writeFileIfDifferent(file)
		if errorValue != nil {
			return false, errorValue
		}
		rewritten = rewritten || changed
	}
	return rewritten, nil
}

func writeFileIfDifferent(file installedFile) (bool, error) {
	current, errorValue := os.ReadFile(file.path)
	if errorValue == nil && string(current) == file.contents {
		return false, nil
	}
	if errorValue != nil && !os.IsNotExist(errorValue) {
		return false, errorValue
	}
	if errorValue := os.WriteFile(file.path, []byte(file.contents), file.mode); errorValue != nil {
		return false, errorValue
	}
	return true, os.Chmod(file.path, file.mode)
}

func (service *Service) runSystemControl(ctx context.Context, arguments ...string) {
	if output, errorValue := service.runCommand(ctx, "systemctl", arguments...); errorValue != nil {
		log.Printf("systemctl %v failed: %v: %s", arguments, errorValue, output)
	}
}
