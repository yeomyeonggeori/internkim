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

func uniqueNonEmptyStrings(values []string) []string {
	seenValues := map[string]bool{}
	var uniqueValues []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seenValues[value] {
			continue
		}
		seenValues[value] = true
		uniqueValues = append(uniqueValues, value)
	}
	return uniqueValues
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func containsArg(flag string) bool {
	for _, a := range os.Args {
		if a == flag {
			return true
		}
	}
	return false
}

func hasFlag(flag string) bool {
	for _, argument := range os.Args {
		if argument == flag || strings.HasPrefix(argument, flag+"=") {
			return true
		}
	}
	return false
}

func argInt(flag string, fallback int) int {
	for i, a := range os.Args {
		if a == flag && i+1 < len(os.Args) {
			var v int
			fmt.Sscanf(os.Args[i+1], "%d", &v)
			return v
		}
	}
	return fallback
}

func argString(flag, fallback string) string {
	for i, a := range os.Args {
		if a == flag && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
		if strings.HasPrefix(a, flag+"=") {
			return strings.TrimPrefix(a, flag+"=")
		}
	}
	return fallback
}

func containsName(names []string, expectedName string) bool {
	for _, name := range names {
		if name == expectedName {
			return true
		}
	}
	return false
}

func appendMissingName(names []string, name string) []string {
	if containsName(names, name) {
		return names
	}
	return append(names, name)
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
