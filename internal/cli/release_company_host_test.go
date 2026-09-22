package cli

import (
	"bytes"
	"os"
	"testing"
)

func TestPublishCompanyHostReleasePublishesEveryTargetAndItsChecksums(t *testing.T) {
	publisher := &testReleasePublisher{publicBaseURL: "https://updates.example.test"}
	build := func(repositoryRootPath string, product releaseProduct, linkerFlags string) binaryBuilder {
		return func(target releaseTarget, outputPath string) error {
			return os.WriteFile(outputPath, []byte(linkerFlags+" "+target.String()), 0o755)
		}
	}

	if errorValue := publishCompanyHostRelease("20260922T000000Z-abc123", publisher, ".", &bytes.Buffer{}, build); errorValue != nil {
		t.Fatal(errorValue)
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
