package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func runCmd(name string, args ...string) string {
	out, _ := exec.Command(name, args...).CombinedOutput()
	return string(out)
}

func fatal(text string) {
	fmt.Fprintf(os.Stderr, "\n✗ %s\n", text)
	os.Exit(1)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func commandArgumentValue(arguments []string, name string, defaultValue string) string {
	for index, argument := range arguments {
		if argument == name && index+1 < len(arguments) {
			return strings.TrimSpace(arguments[index+1])
		}
		if strings.HasPrefix(argument, name+"=") {
			return strings.TrimSpace(strings.TrimPrefix(argument, name+"="))
		}
	}
	return defaultValue
}

func splitFlagsAndPositionals(arguments []string, booleanFlags map[string]bool, valueFlags map[string]bool) ([]string, []string) {
	flagArguments := []string{}
	positionalArguments := []string{}
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		if !strings.HasPrefix(argument, "-") {
			positionalArguments = append(positionalArguments, argument)
			continue
		}
		flagBody := strings.TrimLeft(argument, "-")
		flagName := flagBody
		hasInlineValue := false
		if equalIndex := strings.Index(flagBody, "="); equalIndex >= 0 {
			flagName = flagBody[:equalIndex]
			hasInlineValue = true
		}
		switch {
		case booleanFlags[flagName]:
			flagArguments = append(flagArguments, argument)
		case valueFlags[flagName]:
			flagArguments = append(flagArguments, argument)
			if !hasInlineValue && index+1 < len(arguments) {
				index++
				flagArguments = append(flagArguments, arguments[index])
			}
		default:
			positionalArguments = append(positionalArguments, argument)
		}
	}
	return flagArguments, positionalArguments
}

type repeatedStringFlag struct {
	values []string
}

func (flagValue *repeatedStringFlag) String() string {
	return strings.Join(flagValue.values, ",")
}

func (flagValue *repeatedStringFlag) Set(value string) error {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue != "" {
		flagValue.values = append(flagValue.values, trimmedValue)
	}
	return nil
}

func (flagValue repeatedStringFlag) Values() []string {
	return append([]string{}, flagValue.values...)
}
