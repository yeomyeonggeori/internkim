package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func TestAHostPackagesAdmindBuildsFromACloneWithNoVault(t *testing.T) {
	t.Setenv("SUPABASE_URL", "")
	t.Setenv("SUPABASE_PUBLISHABLE_KEY", "")
	if stamped := admindStampFlags("build", "revision"); strings.Contains(stamped, "centralplane") {
		t.Fatalf("a host package's admind carries a central plane: %s", stamped)
	}

	repositoryRootPath, errorValue := filepath.Abs("../..")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	admind := packagedGoProgram{Name: blueclaw.AdmindName, Package: "./cmd/" + blueclaw.AdmindName}
	outputPath := filepath.Join(t.TempDir(), blueclaw.AdmindName)
	if errorValue := crossCompilePackagedProgram(repositoryRootPath, admind, packageTargets[0], "1.2.3", outputPath); errorValue != nil {
		t.Fatalf("release packages could not build admind with no SUPABASE_URL or SUPABASE_PUBLISHABLE_KEY: %v", errorValue)
	}
}
