package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestPublishCompanyHostReleaseStampsEveryBinaryWithTheNamedImage(t *testing.T) {
	publisher := &testReleasePublisher{publicBaseURL: "https://updates.example.test"}
	var stampedFlags []string
	build := func(repositoryRootPath string, product releaseProduct, linkerFlags string) binaryBuilder {
		stampedFlags = append(stampedFlags, linkerFlags)
		return func(target releaseTarget, outputPath string) error {
			return os.WriteFile(outputPath, []byte(linkerFlags+" "+target.String()), 0o755)
		}
	}
	expectedImage := "registry.example.test/company-host:20260922T000000Z-abc123"

	errorValue := publishCompanyHostRelease(
		"20260922T000000Z-abc123", expectedImage, publisher, ".", &bytes.Buffer{}, build,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(stampedFlags) != 1 || !strings.Contains(stampedFlags[0], companyHostAgentImageVariable+"="+expectedImage) {
		t.Fatalf("the host binaries were built with %v", stampedFlags)
	}
	for _, prefix := range []string{"host/20260922T000000Z-abc123/", "host/latest/"} {
		for _, target := range binaryReleaseTargets {
			if len(publisher.objects[prefix+companyHostProduct.ArtifactName(target)]) == 0 {
				t.Fatalf("%s carries no %s build", prefix, target)
			}
		}
		if len(publisher.objects[prefix+binaryReleaseChecksumsName]) == 0 {
			t.Fatalf("%s carries no checksums", prefix)
		}
	}
}

func TestPublishCompanyHostReleaseRefusesWithoutAnImageToStamp(t *testing.T) {
	publisher := &testReleasePublisher{publicBaseURL: "https://updates.example.test"}
	build := func(string, releaseProduct, string) binaryBuilder {
		t.Fatal("a release was built with no company server image to stamp")
		return nil
	}

	errorValue := publishCompanyHostRelease("r1", "", publisher, ".", &bytes.Buffer{}, build)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "--image") {
		t.Fatalf("publishing without an image returned %v", errorValue)
	}
	if len(publisher.objects) != 0 {
		t.Fatal("something was published with no company server image to stamp")
	}
}
