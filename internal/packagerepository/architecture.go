package packagerepository

import (
	"fmt"
	"slices"
	"strings"
)

// CheckArchitecture refuses a package built for an architecture the repository
// does not publish, which would otherwise sit in the tree with no index naming it.
func CheckArchitecture(repositoryName string, packageFileName string, architecture string, published []string) error {
	if slices.Contains(published, architecture) {
		return nil
	}
	return fmt.Errorf("%s is built for %s, and the %s repository publishes %s",
		packageFileName, architecture, repositoryName, strings.Join(published, " and "))
}
