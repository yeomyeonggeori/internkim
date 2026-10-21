package cli

import (
	"fmt"

	"github.com/yeomyeonggeori/internkim/internal/hostversion"
)

const releasedRevision = "origin/main"

func nextReleaseVersion(repositoryRootPath string, requestedBump string) (hostversion.Version, error) {
	bump, errorValue := hostversion.ParseBump(requestedBump)
	if errorValue != nil {
		return hostversion.Version{}, errorValue
	}
	return hostversion.NextForRevision(repositoryRootPath, releasedRevision, bump)
}

func chosenPackageVersion(requested string, repositoryRootPath string) (string, error) {
	if requested == "" {
		version, errorValue := hostversion.ForRevision(repositoryRootPath, releasedRevision)
		return version.Release(), errorValue
	}
	version, errorValue := hostversion.Parse(requested)
	if errorValue != nil {
		return "", errorValue
	}
	if version.Kind != hostversion.Semantic {
		return "", fmt.Errorf("%s is a date version, and packages are built as MAJOR.MINOR.PATCH or MAJOR.MINOR.PATCH+COMMITS", requested)
	}
	return version.Release(), nil
}

func releaseVersionToPrint(repositoryRootPath string, arguments []string) (hostversion.Version, error) {
	if bump := commandArgumentValue(arguments, "--bump", ""); bump != "" {
		return nextReleaseVersion(repositoryRootPath, bump)
	}
	return hostversion.ForRevision(repositoryRootPath, releasedRevision)
}

func runReleaseVersion(arguments []string) error {
	if of := commandArgumentValue(arguments, "--of", ""); of != "" {
		version, errorValue := hostversion.Parse(of)
		if errorValue != nil {
			return errorValue
		}
		fmt.Println(version.Release())
		return nil
	}
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	version, errorValue := releaseVersionToPrint(repositoryRootPath, arguments)
	if errorValue != nil {
		return errorValue
	}
	fmt.Println(version.Release())
	return nil
}
