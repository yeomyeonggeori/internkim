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
		return fmt.Errorf("verify needs a subcommand: install-addresses, blueclaw-pointer or release-tree")
	}
	switch arguments[0] {
	case "install-addresses":
		return runVerifyInstallAddresses()
	case "blueclaw-pointer":
		return runVerifyBlueclawPointer(arguments[1:])
	case "release-tree":
		return runVerifyReleaseTree()
	default:
		return fmt.Errorf("unknown verify subcommand: %s", arguments[0])
	}
}

func runVerifyReleaseTree() error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	return checkReleaseTree(repositoryRootPath)
}
