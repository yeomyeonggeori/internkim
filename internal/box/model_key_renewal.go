package box

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/durablefile"
)

const (
	modelKeyAwaitingRestartFileName = "model-key-awaiting-restart"
	modelKeyFromCompanyFileName     = "model-key-from-company"
)

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
	if errorValue := daemon.noteModelKeyFromCompany(); errorValue != nil {
		return errorValue
	}
	return daemon.restartForANewModelKey(companyName)
}

func (daemon Daemon) writeModelKey(modelKey string, isKept bool) error {
	if isKept {
		return replaceKeepingOwner(daemon.Places.ModelKeyPath, modelKey+"\n")
	}
	return durablefile.Write(daemon.Places.ModelKeyPath, []byte(modelKey+"\n"), 0o600)
}

func (daemon Daemon) noteModelKeyFromCompany() error {
	notePath := daemon.statePath(modelKeyFromCompanyFileName)
	if _, errorValue := os.Stat(notePath); !errors.Is(errorValue, os.ErrNotExist) {
		return errorValue
	}
	return durablefile.Write(notePath, nil, 0o600)
}

func (daemon Daemon) giveBackModelKey(companyName string) error {
	notePath := daemon.statePath(modelKeyFromCompanyFileName)
	_, errorValue := os.Stat(notePath)
	if errors.Is(errorValue, os.ErrNotExist) {
		return daemon.restartForANewModelKey(companyName)
	}
	if errorValue != nil {
		return errorValue
	}
	if errorValue := durablefile.Write(daemon.statePath(modelKeyAwaitingRestartFileName), nil, 0o600); errorValue != nil {
		return errorValue
	}
	if errorValue := removeIfPresent(daemon.Places.ModelKeyPath); errorValue != nil {
		return errorValue
	}
	if errorValue := os.Remove(notePath); errorValue != nil {
		return errorValue
	}
	log.Printf("%s removed the model key it gave this box", companyName)
	return daemon.restartForANewModelKey(companyName)
}

func (daemon Daemon) followTheModelPayer(session Session) error {
	if !session.RestartsForModelPayer {
		return nil
	}
	if errorValue := durablefile.Write(daemon.statePath(modelKeyAwaitingRestartFileName), nil, 0o600); errorValue != nil {
		return errorValue
	}
	return daemon.restartForANewModelKey(session.Configuration.Company.Name)
}

func (daemon Daemon) restartForANewModelKey(companyName string) error {
	awaitingPath := daemon.statePath(modelKeyAwaitingRestartFileName)
	if _, errorValue := os.Stat(awaitingPath); errorValue != nil || daemon.RestartServices == nil {
		return nil
	}
	log.Printf("%s changed this box's model key or who pays for its models, so its services start again to follow it", companyName)
	if errorValue := daemon.RestartServices(); errorValue != nil {
		return fmt.Errorf("starting %s's services again with its changed model key: %w", companyName, errorValue)
	}
	return removeIfPresent(awaitingPath)
}
