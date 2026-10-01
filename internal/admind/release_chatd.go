package admind

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strings"

	blueclawruntime "github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func (service *Service) installReleaseChatdService(ctx context.Context) error {
	unit, isKnown := releaseChatdUnit(service.buzzRelayPublicURL())
	if !isKnown {
		log.Printf("release chatd: no relay public address is recorded, so the unit setup wrote stays as it is")
		return service.restartReleaseChatd(ctx)
	}
	if errorValue := os.WriteFile(blueclawruntime.ChatdServicePath, []byte(unit), 0o644); errorValue != nil {
		return fmt.Errorf("write the chatd unit: %w", errorValue)
	}
	if errorValue := removeLegacyChatdTLSDropIns(); errorValue != nil {
		return errorValue
	}
	if output, errorValue := service.runCommand(ctx, "systemctl", "daemon-reload"); errorValue != nil {
		return fmt.Errorf("reload systemd: %s: %w", strings.TrimSpace(string(output)), errorValue)
	}
	return service.restartReleaseChatd(ctx)
}

func removeLegacyChatdTLSDropIns() error {
	return removeFilesIfPresent(blueclawruntime.ChatdLegacyTLSDropInPaths())
}

func removeFilesIfPresent(paths []string) error {
	for _, path := range paths {
		if errorValue := os.Remove(path); errorValue != nil && !errors.Is(errorValue, fs.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", path, errorValue)
		}
	}
	return nil
}

func releaseChatdUnit(relayPublicURL string) (string, bool) {
	if strings.TrimSpace(relayPublicURL) == "" {
		return "", false
	}
	return blueclawruntime.ChatdServiceUnit(relayPublicURL), true
}
