package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

func TestPublishCompanionReleaseUploadsEveryTargetUnderTheReleaseAndLatest(t *testing.T) {
	publisher := &testReleasePublisher{publicBaseURL: "https://updates.example.test"}
	var output bytes.Buffer
	build := func(target companionReleaseTarget, outputPath string) error {
		return os.WriteFile(outputPath, []byte("binary for "+target.String()), 0o755)
	}

	if errorValue := publishCompanionRelease("20260918T000000Z-abc123", publisher, build, &output); errorValue != nil {
		t.Fatal(errorValue)
	}

	for _, target := range companionReleaseTargets {
		expected := []byte("binary for " + target.String())
		for _, prefix := range []string{"companion/20260918T000000Z-abc123/", "companion/latest/"} {
			if !bytes.Equal(publisher.objects[prefix+target.BinaryName()], expected) {
				t.Fatalf("%s%s was not uploaded", prefix, target.BinaryName())
			}
		}
	}
	darwinDigest := sha256.Sum256([]byte("binary for darwin/arm64"))
	checksums := string(publisher.objects["companion/latest/SHA256SUMS"])
	if !strings.Contains(checksums, hex.EncodeToString(darwinDigest[:])+"  internkim-companion-darwin-arm64\n") {
		t.Fatalf("checksums lack the darwin/arm64 line:\n%s", checksums)
	}
	if !bytes.Equal(publisher.objects["companion/20260918T000000Z-abc123/SHA256SUMS"], []byte(checksums)) {
		t.Fatal("the release prefix holds different checksums from latest")
	}
	if !strings.Contains(output.String(), "published companion 20260918T000000Z-abc123 -> https://updates.example.test/companion/latest/") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestPublishCompanionReleaseStopsAtTheFirstBuildFailure(t *testing.T) {
	publisher := &testReleasePublisher{publicBaseURL: "https://updates.example.test"}
	build := func(target companionReleaseTarget, outputPath string) error {
		return os.ErrPermission
	}

	if errorValue := publishCompanionRelease("r1", publisher, build, &bytes.Buffer{}); errorValue == nil {
		t.Fatal("a failed build published anyway")
	}
	if len(publisher.objects) != 0 {
		t.Fatalf("uploaded %d objects after a failed build", len(publisher.objects))
	}
}

func TestCompanionBuildEnvironmentKeepsCgoForDarwinOnly(t *testing.T) {
	darwin := strings.Join(companionBuildEnvironment(companionReleaseTarget{OperatingSystem: "darwin", Architecture: "arm64"}), " ")
	if darwin != "GOOS=darwin GOARCH=arm64 CGO_ENABLED=1" {
		t.Fatalf("darwin environment = %q", darwin)
	}
	linux := strings.Join(companionBuildEnvironment(companionReleaseTarget{OperatingSystem: "linux", Architecture: "amd64"}), " ")
	if linux != "GOOS=linux GOARCH=amd64 CGO_ENABLED=0" {
		t.Fatalf("linux environment = %q", linux)
	}
}
