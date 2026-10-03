package cli

import (
	"fmt"
	"os"
	"strings"
)

func runVerify() {
	if errorValue := runVerifyArguments(os.Args[2:]); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runVerifyArguments(arguments []string) error {
	if len(arguments) == 0 || strings.HasPrefix(arguments[0], "-") {
		return fmt.Errorf("verify needs a subcommand: install-addresses or blueclaw-pointer")
	}
	switch arguments[0] {
	case "install-addresses":
		return runVerifyInstallAddresses()
	case "blueclaw-pointer":
		return runVerifyBlueclawPointer(arguments[1:])
	default:
		return fmt.Errorf("unknown verify subcommand: %s", arguments[0])
	}
}
