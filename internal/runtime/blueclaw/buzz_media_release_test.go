package blueclaw_test

import (
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func TestEveryPublishedMachineResolvesToAPinnedTarball(t *testing.T) {
	for _, machine := range blueclaw.BuzzMediaPublishedMachines() {
		release, isPublished := blueclaw.BuzzMediaReleaseFor(machine)
		if !isPublished {
			t.Fatalf("%s is listed as published and resolves to nothing", machine)
		}
		if len(release.SHA256) != 64 {
			t.Fatalf("%s resolves to checksum %q, which is not a sha256; an unchecked download is how "+
				"the media store becomes whatever answered the request", machine, release.SHA256)
		}
		if !strings.Contains(release.URL, "v"+blueclaw.BuzzMediaVersion+"/") ||
			!strings.Contains(release.AssetName, blueclaw.BuzzMediaVersion) {
			t.Fatalf("%s resolves to %s at %s, which does not name version %s",
				machine, release.AssetName, release.URL, blueclaw.BuzzMediaVersion)
		}
	}
	if _, isPublished := blueclaw.BuzzMediaReleaseFor("riscv64"); isPublished {
		t.Fatal("an architecture versity does not publish resolved to a release, so the provisioning step " +
			"would download a tarball that does not exist instead of saying which architectures exist")
	}
}
