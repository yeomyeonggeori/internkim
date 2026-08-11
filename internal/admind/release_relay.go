package admind

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	blueclawruntime "gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

// The relay reads settings a device only has once its company is set up, so the
// unit carries ConditionPathExists and systemd leaves it alone until then. That
// is why a deployment can ship the relay to every device without deciding which
// of them runs one.
func (service *Service) installReleaseRelayService(ctx context.Context) error {
	if errorValue := os.MkdirAll(filepath.Dir(blueclawruntime.RelayServicePath), 0o755); errorValue != nil {
		return fmt.Errorf("relay unit directory: %w", errorValue)
	}
	if errorValue := os.WriteFile(blueclawruntime.RelayServicePath, []byte(blueclawruntime.RelayServiceUnit()), 0o644); errorValue != nil {
		return fmt.Errorf("write the relay unit: %w", errorValue)
	}
	if output, errorValue := service.runCommand(ctx, "systemctl", "daemon-reload"); errorValue != nil {
		return fmt.Errorf("reload systemd: %s: %w", strings.TrimSpace(string(output)), errorValue)
	}
	if output, errorValue := service.runCommand(ctx, "systemctl", "enable", blueclawruntime.RelayServiceName); errorValue != nil {
		return fmt.Errorf("enable the relay: %s: %w", strings.TrimSpace(string(output)), errorValue)
	}
	if !relaySettingsExist() {
		return nil
	}
	if output, errorValue := service.runCommand(ctx, "systemctl", "restart", blueclawruntime.RelayServiceName); errorValue != nil {
		return fmt.Errorf("restart the relay: %s: %w", strings.TrimSpace(string(output)), errorValue)
	}
	return nil
}

func relaySettingsExist() bool {
	information, errorValue := os.Stat(blueclawruntime.RelayEnvironmentFilePath)
	return errorValue == nil && information.Size() > 0
}
