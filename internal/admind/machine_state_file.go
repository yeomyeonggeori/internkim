package admind

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
)

func (service *Service) machineStatePath(name string) string {
	return filepath.Join(filepath.Dir(service.stateDatabasePath()), name)
}

func (service *Service) readMachineState(
	ctx context.Context,
	name string,
	into any,
	whenUnreadable string,
) (bool, error) {
	document, errorValue := os.ReadFile(service.machineStatePath(name))
	if errors.Is(errorValue, os.ErrNotExist) {
		return false, nil
	}
	if errorValue != nil {
		return false, errorValue
	}
	if json.Unmarshal(document, into) != nil {
		slog.WarnContext(ctx, whenUnreadable, "path", service.machineStatePath(name))
		return false, nil
	}
	return true, nil
}

func (service *Service) writeMachineState(name string, value any) error {
	document, errorValue := json.Marshal(value)
	if errorValue != nil {
		return errorValue
	}
	path := service.machineStatePath(name)
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return errorValue
	}
	return writeFileAtomically(path, document, 0o600)
}
