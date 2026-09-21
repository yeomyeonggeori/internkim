package cli

import (
	"bytes"
	"fmt"
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

func TestPushCompanyHostImageBuildsBothLayersForEveryPlatformTheHostRuns(t *testing.T) {
	recorded := &recordedDockerRuns{}
	var output bytes.Buffer

	agentImage, errorValue := pushCompanyHostImage("20260922T000000Z-abc123", "registry.example.test/company-host", recorded.run, &output)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	expected := "registry.example.test/company-host:20260922T000000Z-abc123"
	if agentImage != expected {
		t.Fatalf("pushed %s", agentImage)
	}
	if len(recorded.commands) != 2 {
		t.Fatalf("ran %d builds", len(recorded.commands))
	}
	base := strings.Join(recorded.commands[0], " ")
	agent := strings.Join(recorded.commands[1], " ")
	for _, wanted := range []string{"--platform linux/amd64,linux/arm64", "--push", "--file host/Dockerfile", "--tag " + expected + "-base"} {
		if !strings.Contains(base, wanted) {
			t.Fatalf("the base image build lacks %q: %s", wanted, base)
		}
	}
	for _, wanted := range []string{"--platform linux/amd64,linux/arm64", "--push", "--file host/quickstart/Dockerfile", "HOST_IMAGE=" + expected + "-base", "--tag " + expected} {
		if !strings.Contains(agent, wanted) {
			t.Fatalf("the agent image build lacks %q: %s", wanted, agent)
		}
	}
	if !strings.Contains(output.String(), "internkim release host --image "+expected) {
		t.Fatalf("the output does not hand the reference to the next command: %q", output.String())
	}
}

func TestPushCompanyHostImageCarriesThePinnedMessengerImage(t *testing.T) {
	recorded := &recordedDockerRuns{}
	if _, errorValue := pushCompanyHostImage("r1", "registry.example.test/company-host", recorded.run, &bytes.Buffer{}); errorValue != nil {
		t.Fatal(errorValue)
	}
	agent := strings.Join(recorded.commands[1], " ")
	if !strings.Contains(agent, "BUZZ_IMAGE=ghcr.io/block/buzz:") {
		t.Fatalf("the agent image was not built against the pinned messenger image: %s", agent)
	}
}

func TestPushCompanyHostImageRefusesWithoutARepository(t *testing.T) {
	recorded := &recordedDockerRuns{}
	_, errorValue := pushCompanyHostImage("r1", "", recorded.run, &bytes.Buffer{})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "INTERNKIM_COMPANY_HOST_IMAGE_REPOSITORY") {
		t.Fatalf("pushing without a repository returned %v", errorValue)
	}
	if len(recorded.commands) != 0 {
		t.Fatalf("ran %d builds with nowhere to push them", len(recorded.commands))
	}
}

func TestPushCompanyHostImageStopsAtTheFirstFailedBuild(t *testing.T) {
	var attempts int
	failing := func(name string, arguments []string) error {
		attempts++
		return fmt.Errorf("buildx exited 1")
	}
	if _, errorValue := pushCompanyHostImage("r1", "registry.example.test/company-host", failing, &bytes.Buffer{}); errorValue == nil {
		t.Fatal("a failed build reported a pushed image")
	}
	if attempts != 1 {
		t.Fatalf("kept going after a failed build: %d attempts", attempts)
	}
}
