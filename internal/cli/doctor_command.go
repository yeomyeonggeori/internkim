package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	internkimlab "github.com/yeomyeonggeori/internkim/internal/lab"
)

type hostDependency struct {
	name        string
	purpose     string
	installHint string
}

func simulationDependencies(configuration internkimlab.Configuration) []hostDependency {
	return []hostDependency{
		{
			name:        configuration.VirtualMachine.Container.BinaryPath,
			purpose:     "container CLI lab simulation",
			installHint: "install the container CLI from https://github.com/apple/container/releases",
		},
		{
			name:        "sshpass",
			purpose:     "password SSH into the lab VM and the company host",
			installHint: "brew install sshpass, or apt install sshpass",
		},
	}
}

func ensureSimulationDependencies(configuration internkimlab.Configuration) error {
	var missingDependencies []hostDependency
	for _, dependency := range simulationDependencies(configuration) {
		if _, errorValue := exec.LookPath(dependency.name); errorValue != nil {
			missingDependencies = append(missingDependencies, dependency)
		}
	}
	if len(missingDependencies) == 0 {
		return nil
	}

	var message strings.Builder
	message.WriteString("missing host dependencies for simulation:\n")
	for _, dependency := range missingDependencies {
		message.WriteString(fmt.Sprintf("  - %s (%s)\n", dependency.name, dependency.purpose))
		message.WriteString(fmt.Sprintf("    install: %s\n", dependency.installHint))
	}
	message.WriteString("\nRun `make deps-sim` or install the commands above, then retry `./internkim setup --sim`.")
	return errors.New(message.String())
}

func runDoctor() {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		fatal(errorValue.Error())
	}

	configurationPath := internkimlab.DefaultConfigurationPath(repositoryRootPath)
	configuration, errorValue := internkimlab.LoadConfiguration(configurationPath)
	if errorValue != nil {
		fatal(errorValue.Error())
	}

	dependencies := []hostDependency{
		{name: "go", purpose: "CLI build/test", installHint: "brew install go"},
		{name: "bun", purpose: "Pages checks and browser tests", installHint: "brew install oven-sh/bun/bun"},
		{name: "bunx", purpose: "Playwright browser test runner", installHint: "brew install oven-sh/bun/bun"},
		{name: "ssh", purpose: "company host access", installHint: "included with macOS"},
	}
	dependencies = append(dependencies, simulationDependencies(configuration)...)

	hasMissingDependency := false
	for _, dependency := range dependencies {
		path, lookupError := exec.LookPath(dependency.name)
		if lookupError == nil {
			fmt.Printf("ok      %-12s %s\n", dependency.name, path)
			continue
		}
		hasMissingDependency = true
		fmt.Printf("missing %-12s %s\n", dependency.name, dependency.purpose)
		fmt.Printf("        install: %s\n", dependency.installHint)
	}

	if _, lookupError := exec.LookPath("bunx"); lookupError == nil {
		if !printPlaywrightBrowserStatus(repositoryRootPath) {
			hasMissingDependency = true
		}
	}

	if hasMissingDependency {
		os.Exit(1)
	}
}

func printPlaywrightBrowserStatus(repositoryRootPath string) bool {
	command := exec.Command("bunx", "playwright", "install", "--list")
	command.Dir = filepath.Join(repositoryRootPath, "web")
	output, errorValue := command.CombinedOutput()
	localPlaywrightPath := filepath.Join(repositoryRootPath, "web", "node_modules", "playwright-core")
	if errorValue == nil && hasPlaywrightChromiumReference(string(output), localPlaywrightPath) {
		fmt.Printf("ok      %-12s %s\n", "chromium", "Playwright browser installed")
		return true
	}

	fmt.Printf("missing %-12s %s\n", "chromium", "Playwright browser")
	fmt.Println("        install: cd web && bunx playwright install chromium")
	return false
}

func hasPlaywrightChromiumReference(output string, localPlaywrightPath string) bool {
	for _, block := range strings.Split(output, "\nPlaywright version:") {
		if strings.Contains(block, localPlaywrightPath) && strings.Contains(block, "/chromium-") {
			return true
		}
	}
	return false
}
