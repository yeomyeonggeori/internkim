package cli

import (
	"fmt"

	"github.com/yeomyeonggeori/internkim/internal/hostversion"
)

func packageVersionFromRepository(repositoryRootPath string) (string, error) {
	version, errorValue := hostversion.ForRevision(repositoryRootPath, "HEAD")
	if errorValue != nil {
		return "", errorValue
	}
	return version.Release(), nil
}

func chosenPackageVersion(requested string, repositoryRootPath string) (string, error) {
	if requested == "" {
		return packageVersionFromRepository(repositoryRootPath)
	}
	version, errorValue := hostversion.Parse(requested)
	if errorValue != nil {
		return "", errorValue
	}
	if version.Kind != hostversion.Milestone {
		return "", fmt.Errorf("%s is a date version, and packages are built as milestones: MAJOR.MINOR.PATCH or MAJOR.MINOR.PATCH+COMMITS", requested)
	}
	return version.Release(), nil
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
	version, errorValue := packageVersionFromRepository(repositoryRootPath)
	if errorValue != nil {
		return errorValue
	}
	fmt.Println(version)
	return nil
}
