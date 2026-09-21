package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"gitlab.com/eastriver/internkim/host/quickstart"
)

const (
	companyHostBinaryName         = "internkim-host"
	companyHostImagePlatforms     = "linux/amd64,linux/arm64"
	companyHostAgentImageVariable = "gitlab.com/eastriver/internkim/internal/companyhost.AgentImage"
)

type commandRunner func(name string, arguments []string) error

func runReleaseCompanyHost(arguments []string) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	releaseID := firstNonEmptyString(commandArgumentValue(arguments, "--release", ""), defaultReleaseID(repositoryRootPath))
	imageRepository := strings.TrimSpace(os.Getenv("INTERNKIM_COMPANY_HOST_IMAGE_REPOSITORY"))
	publisher, errorValue := releasePublisherFromEnvironment(repositoryRootPath)
	if errorValue != nil {
		return errorValue
	}
	return publishCompanyHostRelease(releaseID, imageRepository, publisher, dockerCommandRunner(repositoryRootPath), repositoryRootPath, os.Stdout, crossCompileProduct)
}

func publishCompanyHostRelease(
	releaseID string,
	imageRepository string,
	publisher releaseObjectPublisher,
	run commandRunner,
	repositoryRootPath string,
	output io.Writer,
	crossCompile func(repositoryRootPath string, product releaseProduct, linkerFlags string) binaryBuilder,
) error {
	agentImage, errorValue := publishCompanyHostImage(releaseID, imageRepository, run, output)
	if errorValue != nil {
		return errorValue
	}
	linkerFlags := "-s -w -X " + companyHostAgentImageVariable + "=" + agentImage
	build := crossCompile(repositoryRootPath, companyHostProduct, linkerFlags)
	return publishBinaryRelease(companyHostProduct, releaseID, publisher, build, output)
}

func publishCompanyHostImage(releaseID, imageRepository string, run commandRunner, output io.Writer) (string, error) {
	if imageRepository == "" {
		return "", fmt.Errorf("set INTERNKIM_COMPANY_HOST_IMAGE_REPOSITORY to the registry repository the company server image is published to")
	}
	baseImage := imageRepository + ":" + releaseID + "-base"
	agentImage := imageRepository + ":" + releaseID
	buildArguments := [][]string{
		{
			"buildx", "build", "--platform", companyHostImagePlatforms,
			"--tag", baseImage, "--file", "host/Dockerfile", "--push", ".",
		},
		{
			"buildx", "build", "--platform", companyHostImagePlatforms,
			"--tag", agentImage, "--build-arg", "HOST_IMAGE=" + baseImage,
			"--build-arg", "BUZZ_IMAGE=" + quickstart.MessengerImage(),
			"--file", "host/quickstart/Dockerfile", "--push", ".",
		},
	}
	for _, arguments := range buildArguments {
		if errorValue := run("docker", arguments); errorValue != nil {
			return "", errorValue
		}
	}
	fmt.Fprintf(output, "pushed %s\n", agentImage)
	return agentImage, nil
}

func dockerCommandRunner(repositoryRootPath string) commandRunner {
	return func(name string, arguments []string) error {
		command := exec.Command(name, arguments...)
		command.Dir = repositoryRootPath
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		return command.Run()
	}
}
