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
