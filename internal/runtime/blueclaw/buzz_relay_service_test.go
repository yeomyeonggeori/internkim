package blueclaw

import (
	"strings"
	"testing"
)

func TestBuzzMediaServiceUnitServesTheBucketDirectory(t *testing.T) {
	unit := BuzzMediaServiceUnit()
	for _, expected := range []string{
		"EnvironmentFile=" + BuzzMediaEnvironmentFilePath,
		BuzzMediaBinaryPath + " --port " + BuzzMediaAddress,
		"--health " + BuzzMediaHealthPath,
		"posix " + BuzzMediaRootPath,
	} {
		if !strings.Contains(unit, expected) {
			t.Fatalf("media unit missing %q, got:\n%s", expected, unit)
		}
	}
}

// The one incompatibility measured against versitygw 1.8.0 is on a
// versioning-enabled bucket: re-deleting an exact version that is already gone
// answers InvalidArgument instead of reporting it absent, which stalls a
// resumed tenant-deletion chunk. Without --versioning-dir the gateway refuses
// PutBucketVersioning outright ("Versioning has not been configured for the
// gateway"), so the failure cannot be reached. Nothing in this repository has
// ever enabled bucket versioning, and buzz-deletion works on unversioned
// buckets because ListObjectVersions returns null-version entries that
// exact-version DeleteObjects removes. Turning versioning on is a change of
// behaviour with a known broken edge, not an improvement.
func TestBuzzMediaUnitDoesNotEnableVersioning(t *testing.T) {
	if strings.Contains(BuzzMediaServiceUnit(), "--versioning-dir") {
		t.Fatal("the buzz-media unit passes --versioning-dir, which lets an operator enable bucket " +
			"versioning on a gateway whose exact-version delete is not idempotent on retry; " +
			"read this test's comment before deleting it")
	}
}
