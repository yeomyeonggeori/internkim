package blueclaw

import (
	"strings"
	"testing"
)

func TestTheFormulaRefreshesAnExistingCompanyAfterAnUpgrade(t *testing.T) {
	formula, errorValue := HomebrewFormula(HomebrewFormulaRequest{Version: "1", SourceSHA256: "sum", SourceTarballURL: "https://example.com/a.tar.gz"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	postInstall := formula[strings.Index(formula, "def post_install"):]
	postInstall = postInstall[:strings.Index(postInstall, "\n  end\n")]
	for _, want := range []string{
		`if File.exist?("` + CompanyHostCurrentPath + `")`,
		` unless system "sudo", "#{HOMEBREW_PREFIX}/opt/internkim/bin/internkim", "refresh"`,
		`odie`,
	} {
		if !strings.Contains(postInstall, want) {
			t.Errorf("post_install lacks %q:\n%s", want, postInstall)
		}
	}
	if strings.Index(postInstall, "refresh") < strings.Index(postInstall, SkillPreparationVerb) {
		t.Errorf("the refresh must run after the skills are prepared:\n%s", postInstall)
	}
}
