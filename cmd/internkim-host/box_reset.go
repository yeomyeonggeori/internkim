package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/companyhost"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func runBoxReset(arguments []string) error {
	if len(arguments) != 0 {
		return fmt.Errorf("box reset takes no arguments")
	}
	if !companyhost.ThisMachineKeepsABoxSessionFresh() {
		return fmt.Errorf("%s, which does the reset, runs only on Linux", blueclaw.BoxServiceName)
	}
	if errorValue := companyhost.RequireAdministrator(); errorValue != nil {
		return errorValue
	}
	if errorValue := askTheBoxToReset(blueclaw.CompanyHostBoxStatePath, restartTheBox); errorValue != nil {
		return errorValue
	}
	fmt.Printf("%s is resetting this computer, and it belongs to no company once the central plane answers.\njournalctl -u %s -f follows it.\n", blueclaw.BoxServiceName, blueclaw.BoxServiceName)
	return nil
}

func askTheBoxToReset(stateDirectoryPath string, restart func() error) error {
	if errorValue := box.AskForReset(stateDirectoryPath); errorValue != nil {
		return errorValue
	}
	if errorValue := restart(); errorValue != nil {
		restarting := fmt.Errorf("restarting %s, which does the reset: %w", blueclaw.BoxServiceName, errorValue)
		return errors.Join(restarting, box.WithdrawResetRequest(stateDirectoryPath))
	}
	return nil
}

func restartTheBox() error {
	return thisComputer{}.Run("systemctl", []string{"restart", blueclaw.BoxServiceName + ".service"}, nil, io.Discard)
}
