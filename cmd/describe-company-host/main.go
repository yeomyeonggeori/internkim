package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

type startedService struct {
	Name      string   `json:"name"`
	Arguments []string `json:"arguments"`
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: describe-company-host <directory>")
		os.Exit(2)
	}
	if errorValue := describe(os.Args[1]); errorValue != nil {
		fmt.Fprintln(os.Stderr, errorValue)
		os.Exit(1)
	}
}

func describe(directory string) error {
	services := []startedService{}
	for _, service := range blueclaw.CompanyHostServices(blueclaw.LinuxCompanyHostLayout()) {
		services = append(services, startedService{Name: service.Name, Arguments: service.Command})
	}
	document, errorValue := json.MarshalIndent(services, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	if errorValue := os.WriteFile(filepath.Join(directory, "services.json"), append(document, '\n'), 0o644); errorValue != nil {
		return errorValue
	}
	packages := []string{}
	for _, file := range blueclaw.HostFilesTheBundledSkillsRead() {
		packages = append(packages, file.DebianPackage)
	}
	return os.WriteFile(filepath.Join(directory, "packages-for-files-the-skills-read"), []byte(strings.Join(packages, "\n")+"\n"), 0o644)
}
