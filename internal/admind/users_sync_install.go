package admind

import (
	"context"
	"log"
	"os"

	blueclawruntime "github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

type installedFile struct {
	path     string
	contents string
	mode     os.FileMode
}

func usersSyncInstalledFiles() []installedFile {
	return []installedFile{
		{blueclawruntime.InternKimUsersSyncScriptPath, blueclawruntime.InternKimUsersSyncScript(), 0o755},
		{blueclawruntime.InternKimUsersSyncServicePath, blueclawruntime.InternKimUsersSyncServiceUnit(), 0o644},
		{blueclawruntime.InternKimUsersSyncTimerPath, blueclawruntime.InternKimUsersSyncTimerUnit(), 0o644},
	}
}

func (service *Service) keepUsersSyncInstalled(ctx context.Context) {
	rewritten, errorValue := writeFilesIfDifferent(usersSyncInstalledFiles())
	if errorValue != nil {
		log.Printf("users sync install failed: %v", errorValue)
		return
	}
	if !rewritten {
		return
	}
	log.Printf("users sync reinstalled from this release")
	service.runSystemControl(ctx, "daemon-reload")
	service.runSystemControl(ctx, "enable", "--now", "internkim-users-sync.timer")
	service.runSystemControl(ctx, "start", "internkim-users-sync.service")
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
