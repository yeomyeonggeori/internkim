package cli

import (
	"fmt"
	"os"
	"strings"
)

func releaseBinaryBuildFlags(repositoryRootPath string) ([]string, error) {
	revision := releaseBinaryRevision(repositoryRootPath)
	stamped, errorValue := deviceAdmindStampFlags(revision, revision)
	if errorValue != nil {
		return nil, errorValue
	}
	return []string{"-ldflags", stamped}, nil
}

// deviceAdmindStampFlags is for the device's admind, which can start with neither a
// flag nor a record file naming its central plane and so carries one compiled in.
// A host package learns its plane from the company's connection file, so the
// packages and the keg build from a clone with no vault at all.
func deviceAdmindStampFlags(buildID string, revision string) (string, error) {
	centralPlaneFlags, errorValue := centralPlaneStampFlags()
	if errorValue != nil {
		return "", errorValue
	}
	return admindStampFlags(buildID, revision) + " " + strings.Join(centralPlaneFlags, " "), nil
}

func centralPlaneStampFlags() ([]string, error) {
	projectURL := os.Getenv("SUPABASE_URL")
	publishableKey := os.Getenv("SUPABASE_PUBLISHABLE_KEY")
	var missing []string
	if projectURL == "" {
		missing = append(missing, "SUPABASE_URL")
	}
	if publishableKey == "" {
		missing = append(missing, "SUPABASE_PUBLISHABLE_KEY")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("%s not set: admind is built carrying the central plane they name, so run this under `monkeys run @production`", strings.Join(missing, " and "))
	}
	return []string{
		"-X", "github.com/yeomyeonggeori/internkim/internal/centralplane.DefaultProjectURL=" + projectURL,
		"-X", "github.com/yeomyeonggeori/internkim/internal/centralplane.DefaultPublishableKey=" + publishableKey,
	}, nil
}
