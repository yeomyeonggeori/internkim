package blueclaw

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/hostversion"
)

// The company host as a Homebrew formula. Everything it declares is read from
// this package, the same way the Linux packages' dependency lists are: the depends_on lines
// from HostHomebrewDependencies, the paths from CompanyHostLayout.
//
// What the formula does *not* do is the whole of why there is a second line.
// It supervises nothing: `brew services` holds one service block per formula and
// this bundle is many, and UserName is not among the keys Homebrew::Service can
// emit, so the one service that runs unprivileged could not be given its
// account. It cannot set the setuid bit on the POSIX helper: pouring a bottle is
// a tar extraction as an ordinary user and extraction drops the bit, `brew`
// refuses to run as root, and rubocops/caveats.rb raises on a formula that so
// much as recommends setuid. And it cannot depend on a cask. So `brew install
// internkim` delivers files, and `sudo internkim install` makes the box a host.

const (
	// HomebrewTapOwner and HomebrewTapName are what `brew tap` is given. The
	// convention turns them into github.com/<owner>/homebrew-<name>, which is
	// where the formula lives; the bottle is an asset of the GitHub Release the
	// formula names through root_url below.
	//
	// The tap is named after the organisation and not after this product,
	// because one tap holds every formula the organisation publishes, and a
	// second product is a second formula in the same tap rather than a second
	// tap for a person to add. charmbracelet/tap, supabase/tap and
	// mobile-dev-inc/tap are all this shape.
	HomebrewTapOwner = "yeomyeonggeori"
	HomebrewTapName  = "tap"

	companyPackageDescription = "Run your company's agent, messenger and web app on this computer"
)

// HomebrewTap is what a person types.
func HomebrewTap() string {
	return HomebrewTapOwner + "/" + HomebrewTapName
}

// HomebrewTapRepository is the repository `brew tap` clones, as owner/name.
func HomebrewTapRepository() string {
	return HomebrewTapOwner + "/homebrew-" + HomebrewTapName
}

// HomebrewFormulaFileName is where the formula sits inside the tap.
func HomebrewFormulaFileName() string {
	return "Formula/" + HomebrewFormulaAssetName()
}

// HomebrewFormulaAssetName is the formula as a release asset, which is the copy
// the tap is given when the release becomes stable.
func HomebrewFormulaAssetName() string {
	return CompanyPackageName + ".rb"
}

// HomebrewSourceTarballName is the tarball the formula's url names, which is
// what a Mac no bottle fits installs from. Like the Linux packages it carries no
// version, which lives in the release tag the formula's url names.
func HomebrewSourceTarballName() string {
	return CompanyPackageName + "-macos-arm64.tar.gz"
}

// HomebrewBottleFileName is what Homebrew asks root_url for. Homebrew builds
// the name itself from the formula's version and the bottle's tag
// (Bottle::Filename), so this is the one asset whose name carries a version.
func HomebrewBottleFileName(version string, bottleTag string) string {
	return fmt.Sprintf("%s-%s.%s.bottle.tar.gz", CompanyPackageName, version, bottleTag)
}

// HomebrewBottle is one built bottle: which macOS and architecture it is for,
// which Cellar it was laid out against, and what it hashes to.
type HomebrewBottle struct {
	// Tag is Homebrew's own name for a macOS version and architecture, such as
	// arm64_ventura. It names the oldest macOS every program in the keg runs on,
	// and Homebrew pours a bottle on that version and every later one.
	Tag string
	// Cellar is what the bottle may be poured into. It is :any_skip_relocation
	// because nothing in the keg names the prefix: the programs are static, and
	// the document environment is built by post_install after the pour.
	Cellar string
	SHA256 string
}

// HomebrewFormulaRequest is everything about one release the formula has to
// carry that this package cannot know.
type HomebrewFormulaRequest struct {
	Version          string
	SourceTarballURL string
	SourceSHA256     string
	BottleRootURL    string
	Bottles          []HomebrewBottle
}

// HomebrewFormula renders Formula/internkim.rb.
func HomebrewFormula(request HomebrewFormulaRequest) (string, error) {
	if request.Version == "" || request.SourceSHA256 == "" {
		return "", fmt.Errorf("a formula needs a version and the checksum of what it installs")
	}
	formula := &strings.Builder{}
	formula.WriteString("# typed: false\n# frozen_string_literal: true\n\n")
	formula.WriteString("# Rendered by `internkim release host` from internal/runtime/blueclaw.\n")
	formula.WriteString("# Edit that package, not this file: a hand edit here is a second\n")
	formula.WriteString("# declaration of the same dependency list.\n")
	formula.WriteString("class " + homebrewClassName(CompanyPackageName) + " < Formula\n")
	formula.WriteString("  desc " + rubyString(companyPackageDescription) + "\n")
	formula.WriteString("  homepage " + rubyString(CompanyPackageHomepage) + "\n")
	formula.WriteString("  url " + rubyString(request.SourceTarballURL) + "\n")
	formula.WriteString("  sha256 " + rubyString(request.SourceSHA256) + "\n")
	formula.WriteString("  license " + rubyString(companyPackageLicense) + "\n")
	formula.WriteString("  version " + rubyString(request.Version) + "\n")
	formula.WriteString(fmt.Sprintf("  version_scheme %d\n\n", hostversion.MilestoneEpoch))
	writeHomebrewBottleBlock(formula, request.BottleRootURL, request.Bottles)
	writeHomebrewDependencies(formula)
	writeHomebrewInstallBlock(formula)
	writeHomebrewCaveats(formula)
	writeHomebrewTestBlock(formula)
	formula.WriteString("end\n")
	return formula.String(), nil
}

const companyPackageLicense = "Apache-2.0"

func writeHomebrewBottleBlock(formula *strings.Builder, rootURL string, bottles []HomebrewBottle) {
	if len(bottles) == 0 {
		return
	}
	formula.WriteString("  bottle do\n")
	formula.WriteString("    root_url " + rubyString(rootURL) + "\n")
	sorted := append([]HomebrewBottle(nil), bottles...)
	sort.Slice(sorted, func(first, second int) bool { return sorted[first].Tag < sorted[second].Tag })
	for _, bottle := range sorted {
		formula.WriteString("    sha256 cellar: " + bottle.Cellar +
			", " + bottle.Tag + ": " + rubyString(bottle.SHA256) + "\n")
	}
	formula.WriteString("  end\n\n")
}

func writeHomebrewDependencies(formula *strings.Builder) {
	for _, name := range HostHomebrewDependencies() {
		formula.WriteString("  depends_on " + rubyString(name) + "\n")
	}
	formula.WriteString("  depends_on :macos\n\n")
}

// Installing the keg is a copy. The conversion environment is built and the
// skills are prepared in post_install because Homebrew runs `install` only when
// it builds from source and `post_install` after a pour as well. It is named through the opt link,
// which Homebrew points at the new keg before post_install runs, so the
// environment's interpreter path survives the next upgrade's keg.
func writeHomebrewInstallBlock(formula *strings.Builder) {
	formula.WriteString("  def install\n")
	formula.WriteString("    bin.install Dir[\"bin/*\"]\n")
	formula.WriteString("    libexec.install Dir[\"libexec/*\"]\n")
	formula.WriteString("  end\n\n")

	formula.WriteString("  def post_install\n")
	layout := MacCompanyHostLayout(homebrewPrefixInRuby)
	for _, command := range layout.InstallStepCommands() {
		formula.WriteString("    system " + rubyArguments(command.Arguments) + "\n")
	}
	writeHomebrewRefresh(formula, layout)
	formula.WriteString("  end\n\n")
}

func writeHomebrewRefresh(formula *strings.Builder, layout CompanyHostLayout) {
	refresh := []string{"sudo", layout.CommandPath(), "refresh"}
	message := "this release is installed and the company's server did not come back on it. Once that is fixed, run: sudo " + CompanyPackageName + " refresh"
	formula.WriteString("    if File.exist?(" + rubyString(CompanyHostCurrentPath) + ")\n")
	formula.WriteString("      odie " + rubyString(message) + " unless system " + rubyArguments(refresh) + "\n")
	formula.WriteString("    end\n")
}

const homebrewPrefixInRuby = "#{HOMEBREW_PREFIX}"

func rubyArguments(arguments []string) string {
	quoted := make([]string, len(arguments))
	for index, argument := range arguments {
		quoted[index] = rubyString(argument)
	}
	return strings.Join(quoted, ", ")
}

// The caveats say the two things `brew install` cannot do: the second line, and
// the casks a formula is forbidden to depend on.
func writeHomebrewCaveats(formula *strings.Builder) {
	lines := []string{
		"This installed the programs. It did not start anything: every service stays",
		"idle until this computer has a company.",
		"",
		"Give it one with the connection file you downloaded from company setup:",
		"  sudo " + CompanyPackageName + " install ~/Downloads/" + CompanyPackageName + "-host.json",
		"",
		"That step needs administrator rights because it creates the service accounts,",
		"makes the POSIX helper setuid root, and writes the LaunchDaemons into",
		CompanyHostLaunchDaemonRoot + ".",
	}
	formula.WriteString("  def caveats\n    <<~EOS\n")
	for _, line := range lines {
		formula.WriteString(strings.TrimRight("      "+line, " ") + "\n")
	}
	formula.WriteString("    EOS\n  end\n\n")
}

// `brew test` runs without root and with no company, so what it can show is that
// the programs the keg carries are the ones this machine can run.
func writeHomebrewTestBlock(formula *strings.Builder) {
	formula.WriteString("  test do\n")
	formula.WriteString("    assert_predicate libexec/" + rubyString(POSIXHelperProgramName) + ", :exist?\n")
	formula.WriteString("    assert_predicate libexec/\"skills\", :directory?\n")
	formula.WriteString("    assert_match " + rubyString(CompanyPackageName) +
		", shell_output(\"#{bin}/" + CompanyPackageName + " --help 2>&1\", 1)\n")
	formula.WriteString("  end\n")
}

// homebrewClassName is the class a formula file has to define, which Homebrew
// derives from the file name: internkim.rb defines Internkim.
func homebrewClassName(formulaName string) string {
	parts := strings.FieldsFunc(formulaName, func(character rune) bool {
		return character == '-' || character == '_' || character == '@'
	})
	name := ""
	for _, part := range parts {
		name += strings.ToUpper(part[:1]) + part[1:]
	}
	return name
}

// rubyString is a double-quoted Ruby literal. Every value that reaches it is a
// constant from this package, and quoting them anyway is what keeps that true.
func rubyString(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}
