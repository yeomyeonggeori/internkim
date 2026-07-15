package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

func main() {
	modelName := flag.String("model", "google/gemini-3.5-flash", "default language model")
	capabilitySocketPath := flag.String("socket", "/run/internkim/capability.sock", "capability unix socket path")
	databaseConnectionString := flag.String("dsn", "", "postgres connection string")
	migrationDirectoryPath := flag.String("migrations", "/opt/blueclaw/migrations", "migration directory path")
	workspaceRootPath := flag.String("workspace", "/workspace", "workspace root path")
	posixHelperPath := flag.String("posix-helper", "", "posix helper path (empty skips POSIX user synchronization)")
	mattermostBaseURL := flag.String("mattermost-url", "http://mattermost:8065", "mattermost base URL")
	adminEmail := flag.String("admin-email", "admin@example.test", "admin email for policy")
	outputDirectory := flag.String("out", ".", "output directory")
	capabilityContractOutputPath := flag.String("contract-out", "", "capability contract output path")
	flag.Parse()

	runtimeDocument, errorValue := blueclaw.BlueclawRuntimeConfigDocumentWithOptions(blueclaw.RuntimeConfigOptions{
		DirectExecution:          true,
		ModelName:                *modelName,
		CapabilitySocketPath:     *capabilitySocketPath,
		DatabaseConnectionString: *databaseConnectionString,
		MigrationDirectoryPath:   *migrationDirectoryPath,
		WorkspaceRootPath:        *workspaceRootPath,
		POSIXHelperPath:          *posixHelperPath,
		MattermostBaseURL:        *mattermostBaseURL,
	})
	if errorValue != nil {
		fmt.Fprintln(os.Stderr, errorValue)
		os.Exit(1)
	}

	policyDocument, errorValue := blueclaw.BlueclawPolicyDocument(*adminEmail)
	if errorValue != nil {
		fmt.Fprintln(os.Stderr, errorValue)
		os.Exit(1)
	}

	writeDocument(filepath.Join(*outputDirectory, "runtime.json"), runtimeDocument)
	writeDocument(filepath.Join(*outputDirectory, "policy.json"), policyDocument)
	if *capabilityContractOutputPath != "" {
		capabilityContractDocument, errorValue := blueclaw.CapabilityContractDocument()
		if errorValue != nil {
			fmt.Fprintln(os.Stderr, errorValue)
			os.Exit(1)
		}
		writeDocument(*capabilityContractOutputPath, capabilityContractDocument)
	}
}

func writeDocument(path string, document string) {
	if errorValue := os.WriteFile(path, []byte(document), 0o644); errorValue != nil {
		fmt.Fprintln(os.Stderr, errorValue)
		os.Exit(1)
	}
	fmt.Println("wrote", path)
}
