package box

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/durablefile"
)

const modelKeyAwaitingRestartFileName = "model-key-awaiting-restart"

func (daemon Daemon) keepModelKey(companyName string, modelKey string) error {
	kept, errorValue := os.ReadFile(daemon.Places.ModelKeyPath)
	isKept := errorValue == nil
	if errorValue != nil && !errors.Is(errorValue, os.ErrNotExist) {
		return errorValue
	}
	if !isKept || strings.TrimSpace(string(kept)) != strings.TrimSpace(modelKey) {
		if errorValue := durablefile.Write(daemon.statePath(modelKeyAwaitingRestartFileName), nil, 0o600); errorValue != nil {
			return errorValue
		}
		if errorValue := daemon.writeModelKey(modelKey, isKept); errorValue != nil {
			return errorValue
		}
	}
	return daemon.restartForANewModelKey(companyName)
}

func (daemon Daemon) writeModelKey(modelKey string, isKept bool) error {
	if isKept {
		return replaceKeepingOwner(daemon.Places.ModelKeyPath, modelKey+"\n")
	}
	return durablefile.Write(daemon.Places.ModelKeyPath, []byte(modelKey+"\n"), 0o600)
}

func (daemon Daemon) restartForANewModelKey(companyName string) error {
	awaitingPath := daemon.statePath(modelKeyAwaitingRestartFileName)
	if _, errorValue := os.Stat(awaitingPath); errorValue != nil || daemon.RestartServices == nil {
		return nil
	}
	log.Printf("%s gave this box a new model key, so its services start again to use it", companyName)
	if errorValue := daemon.RestartServices(); errorValue != nil {
		return fmt.Errorf("starting %s's services again with its new model key: %w", companyName, errorValue)
	}
	return removeIfPresent(awaitingPath)
}
