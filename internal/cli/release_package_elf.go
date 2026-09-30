package cli

import (
	"debug/elf"
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

type elfFacts struct {
	Machine     string
	NewestGlibc []int
}

func inspectELF(path string) (elfFacts, bool, error) {
	file, errorValue := elf.Open(path)
	var formatError *elf.FormatError
	if errors.As(errorValue, &formatError) {
		return elfFacts{}, false, nil
	}
	if errorValue != nil {
		return elfFacts{}, false, errorValue
	}
	defer file.Close()
	machine, isTargeted := architectureOfMachine(file.Machine)
	if !isTargeted {
		return elfFacts{}, true, fmt.Errorf("%s is built for %s, a machine this package does not target", path, file.Machine)
	}
	symbols, errorValue := file.ImportedSymbols()
	if errorValue != nil && !errors.Is(errorValue, elf.ErrNoSymbols) {
		return elfFacts{}, true, fmt.Errorf("read the symbols %s imports: %w", path, errorValue)
	}
	return elfFacts{Machine: machine, NewestGlibc: newestGlibcVersion(symbols)}, true, nil
}

func architectureOfMachine(machine elf.Machine) (string, bool) {
	switch machine {
	case elf.EM_AARCH64:
		return "arm64", true
	case elf.EM_X86_64:
		return "amd64", true
	}
	return "", false
}

func newestGlibcVersion(symbols []elf.ImportedSymbol) []int {
	newest := []int{}
	for _, symbol := range symbols {
		version, isGlibc := parseGlibcVersion(symbol.Version)
		if isGlibc && compareVersions(version, newest) > 0 {
			newest = version
		}
	}
	return newest
}

func parseGlibcVersion(name string) ([]int, bool) {
	numbers, isGlibc := strings.CutPrefix(name, "GLIBC_")
	if !isGlibc {
		return nil, false
	}
	version := []int{}
	for _, part := range strings.Split(numbers, ".") {
		number, errorValue := strconv.Atoi(part)
		if errorValue != nil {
			return nil, false
		}
		version = append(version, number)
	}
	return version, true
}

func compareVersions(left []int, right []int) int {
	for index := 0; index < len(left) || index < len(right); index++ {
		difference := versionPart(left, index) - versionPart(right, index)
		if difference != 0 {
			return difference
		}
	}
	return 0
}

func versionPart(version []int, index int) int {
	if index < len(version) {
		return version[index]
	}
	return 0
}

func requireELFFits(path string, target packageTarget) error {
	facts, isELF, errorValue := inspectELF(path)
	if errorValue != nil || !isELF {
		return errorValue
	}
	if facts.Machine != target.Architecture {
		return fmt.Errorf("%s is a %s binary and this package is %s", path, facts.Machine, target.Architecture)
	}
	floor, _ := parseGlibcVersion("GLIBC_" + blueclaw.HostGlibcMinimum)
	if compareVersions(facts.NewestGlibc, floor) > 0 {
		return fmt.Errorf("%s needs glibc %s and the package runs on %s", path, joinVersion(facts.NewestGlibc), blueclaw.HostGlibcMinimum)
	}
	return nil
}

func joinVersion(version []int) string {
	parts := make([]string, len(version))
	for index, number := range version {
		parts[index] = strconv.Itoa(number)
	}
	return strings.Join(parts, ".")
}

func requireMessengerBinary(sourcePath string, name string, target packageTarget) error {
	facts, isELF, errorValue := inspectELF(sourcePath)
	if errors.Is(errorValue, fs.ErrNotExist) {
		return fmt.Errorf("%s is not at %s; build it with `tools/prepare-buzz-relay --target linux-%s`: %w", name, sourcePath, target.Architecture, errorValue)
	}
	if errorValue != nil {
		return errorValue
	}
	if !isELF {
		return fmt.Errorf("%s at %s is not a Linux binary; rebuild it with `tools/prepare-buzz-relay --target linux-%s`", name, sourcePath, target.Architecture)
	}
	if facts.Machine != target.Architecture {
		return fmt.Errorf(
			"%s at %s is a %s binary and this package is %s; `tools/prepare-buzz-relay --target linux-%s` builds that one",
			name, sourcePath, facts.Machine, target.Architecture, target.Architecture)
	}
	return nil
}

func requirePackagedProgramsFit(packaged []packagedFile, target packageTarget) error {
	for _, file := range packaged {
		if file.IsSymbolicLink || file.IsDirectoryTree {
			continue
		}
		if errorValue := requireELFFits(file.SourcePath, target); errorValue != nil {
			return errorValue
		}
	}
	return nil
}
