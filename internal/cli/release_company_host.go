package cli

import (
	"fmt"
	"io"
	"os"
)

const (
	companyHostBinaryName         = "internkim-host"
	companyHostAgentImageVariable = "gitlab.com/eastriver/internkim/internal/companyhost.AgentImage"
)

func runReleaseCompanyHost(arguments []string) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	releaseID := firstNonEmptyString(commandArgumentValue(arguments, "--release", ""), defaultReleaseID(repositoryRootPath))
	agentImage := commandArgumentValue(arguments, "--image", "")
	publisher, errorValue := releasePublisherFromEnvironment(repositoryRootPath)
	if errorValue != nil {
		return errorValue
	}
	return publishCompanyHostRelease(releaseID, agentImage, publisher, repositoryRootPath, os.Stdout, crossCompileProduct)
}

func publishCompanyHostRelease(
	releaseID string,
	agentImage string,
	publisher releaseObjectPublisher,
	repositoryRootPath string,
	output io.Writer,
	crossCompile func(repositoryRootPath string, product releaseProduct, linkerFlags string) binaryBuilder,
) error {
	if agentImage == "" {
		return fmt.Errorf("name the company server image with --image; a host build only ever runs the one it was stamped with")
	}
	linkerFlags := "-s -w -X " + companyHostAgentImageVariable + "=" + agentImage
	build := crossCompile(repositoryRootPath, companyHostProduct, linkerFlags)
	return publishBinaryRelease(companyHostProduct, releaseID, publisher, build, output)
}
