package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/companyhost"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

type startedService struct {
	Name      string   `json:"name"`
	Arguments []string `json:"arguments"`
}

func main() {
	switch {
	case len(os.Args) == 4 && os.Args[1] == "messenger-url":
		fmt.Println(messengerURL(os.Args[2], os.Args[3]))
	case len(os.Args) == 2:
		stopOnFailure(describe(os.Args[1]))
	default:
		fmt.Fprintln(os.Stderr, "usage: describe-company-host <directory>")
		fmt.Fprintln(os.Stderr, "       describe-company-host messenger-url <company-slug> <app-url>")
		os.Exit(2)
	}
}

func stopOnFailure(errorValue error) {
	if errorValue == nil {
		return
	}
	fmt.Fprintln(os.Stderr, errorValue)
	os.Exit(1)
}

func messengerURL(companySlug string, appURL string) string {
	return companyhost.MessengerURL(companyhost.Connection{
		AppURL:  appURL,
		Company: companyhost.Company{Slug: companySlug},
	})
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
	model := blueclaw.HostEmbeddingModelDownload()
	modelDocument, errorValue := json.MarshalIndent(map[string]string{"fileName": model.ProgramName, "url": model.URL, "sha256": model.SHA256}, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	if errorValue := os.WriteFile(filepath.Join(directory, "embedding-model.json"), append(modelDocument, '\n'), 0o644); errorValue != nil {
		return errorValue
	}
	packages := []string{}
	for _, file := range blueclaw.HostFilesTheBundledSkillsRead() {
		packages = append(packages, file.DebianPackage)
	}
	return os.WriteFile(filepath.Join(directory, "packages-for-files-the-skills-read"), []byte(strings.Join(packages, "\n")+"\n"), 0o644)
}
