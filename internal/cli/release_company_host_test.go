package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

type recordedDockerRuns struct {
	commands [][]string
}

func (recorded *recordedDockerRuns) run(name string, arguments []string) error {
	recorded.commands = append(recorded.commands, append([]string{name}, arguments...))
	return nil
}

func TestPublishCompanyHostReleaseStampsEveryBinaryWithTheImageItPushed(t *testing.T) {
	publisher := &testReleasePublisher{publicBaseURL: "https://updates.example.test"}
	recorded := &recordedDockerRuns{}
	var stampedFlags []string
	build := func(repositoryRootPath string, product releaseProduct, linkerFlags string) binaryBuilder {
		stampedFlags = append(stampedFlags, linkerFlags)
		return func(target releaseTarget, outputPath string) error {
			return os.WriteFile(outputPath, []byte(linkerFlags+" "+target.String()), 0o755)
		}
	}

	errorValue := publishCompanyHostRelease(
		"20260922T000000Z-abc123", "registry.example.test/company-host", publisher, recorded.run, ".", &bytes.Buffer{}, build,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	expectedImage := "registry.example.test/company-host:20260922T000000Z-abc123"
	if len(stampedFlags) != 1 || !strings.Contains(stampedFlags[0], companyHostAgentImageVariable+"="+expectedImage) {
		t.Fatalf("the host binaries were built with %v", stampedFlags)
	}
	if len(recorded.commands) != 2 {
		t.Fatalf("pushed %d images", len(recorded.commands))
	}
	base := strings.Join(recorded.commands[0], " ")
	agent := strings.Join(recorded.commands[1], " ")
	for _, expected := range []string{"--platform linux/amd64,linux/arm64", "--push", "--file host/Dockerfile", expectedImage + "-base"} {
		if !strings.Contains(base, expected) {
			t.Fatalf("the base image build lacks %q: %s", expected, base)
		}
	}
	for _, expected := range []string{"--file host/quickstart/Dockerfile", "HOST_IMAGE=" + expectedImage + "-base", "--tag " + expectedImage} {
		if !strings.Contains(agent, expected) {
			t.Fatalf("the agent image build lacks %q: %s", expected, agent)
		}
	}
	for _, prefix := range []string{"host/20260922T000000Z-abc123/", "host/latest/"} {
		if len(publisher.objects[prefix+"internkim-host-darwin-arm64"]) == 0 {
			t.Fatalf("%s carries no darwin/arm64 build", prefix)
		}
		if len(publisher.objects[prefix+binaryReleaseChecksumsName]) == 0 {
			t.Fatalf("%s carries no checksums", prefix)
		}
	}
}

func TestPublishCompanyHostReleaseRefusesWithoutAnImageRepository(t *testing.T) {
	publisher := &testReleasePublisher{publicBaseURL: "https://updates.example.test"}
	recorded := &recordedDockerRuns{}
	build := func(string, releaseProduct, string) binaryBuilder {
		t.Fatal("a release was built without a registry to push the server image to")
		return nil
	}

	errorValue := publishCompanyHostRelease("r1", "", publisher, recorded.run, ".", &bytes.Buffer{}, build)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "INTERNKIM_COMPANY_HOST_IMAGE_REPOSITORY") {
		t.Fatalf("publishing without an image repository returned %v", errorValue)
	}
	if len(publisher.objects) != 0 || len(recorded.commands) != 0 {
		t.Fatal("something was published without a registry to push the server image to")
	}
}
