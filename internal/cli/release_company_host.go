package cli

import (
	"io"
	"os"
)

// The published internkim-host binary is what a machine with no package manager
// installs. It carries no image reference any more: the company server is the
// programs this repository builds, supervised by systemd, and `internkim
// install` is what starts them.
const companyHostBinaryName = "internkim-host"

func runReleaseCompanyHost(arguments []string) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	releaseID := firstNonEmptyString(commandArgumentValue(arguments, "--release", ""), defaultReleaseID(repositoryRootPath))
	publisher, errorValue := releasePublisherFromEnvironment(repositoryRootPath)
	if errorValue != nil {
		return errorValue
	}
	return publishCompanyHostRelease(releaseID, publisher, repositoryRootPath, os.Stdout, crossCompileProduct)
}

func publishCompanyHostRelease(
	releaseID string,
	publisher releaseObjectPublisher,
	repositoryRootPath string,
	output io.Writer,
	crossCompile func(repositoryRootPath string, product releaseProduct, linkerFlags string) binaryBuilder,
) error {
	build := crossCompile(repositoryRootPath, companyHostProduct, "-s -w")
	return publishBinaryRelease(companyHostProduct, releaseID, publisher, build, output)
}
