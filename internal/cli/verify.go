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
		return fmt.Errorf("usage: internkim verify blueclaw-pointer [base]")
	}
	switch arguments[0] {
	case "blueclaw-pointer":
		return runVerifyBlueclawPointer(arguments[1:])
	default:
		return fmt.Errorf("unknown verify subcommand: %s", arguments[0])
	}
}
