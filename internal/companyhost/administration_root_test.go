package companyhost_test

import (
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/admind"
	"github.com/yeomyeonggeori/internkim/internal/companyhost"
)

func TestTheAdministrationRootHoldsWhatTheHostsAdmindWrites(t *testing.T) {
	defaults := admind.DefaultConfiguration()
	for name, path := range map[string]string{
		"state directory":        defaults.StateDirectory,
		"state databases":        defaults.TaskDatabasePath,
		"sites":                  defaults.SitesRoot,
		"site secrets":           defaults.SiteSecretDirectory,
		"identity document":      defaults.IdentityDocumentPath,
		"soul document":          defaults.SoulDocumentPath,
		"central plane app file": defaults.CentralPlaneAppURLPath,
	} {
		if !strings.HasPrefix(path, companyhost.AdministrationStateRoot+"/") {
			t.Errorf("admind keeps its %s at %s, outside %s, so a backup would miss it", name, path, companyhost.AdministrationStateRoot)
		}
	}
}
