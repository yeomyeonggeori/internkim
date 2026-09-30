package blueclaw

import (
	"fmt"
	"sort"
	"strings"

	"gitlab.com/eastriver/internkim/internal/fleetdomain"
)

// The company host as a Homebrew formula. Everything it declares is read from
// this package, the same way the .deb's control fields are: the depends_on lines
// from HostHomebrewDependencies, the paths from CompanyHostLayout.
//
// What the formula does *not* do is the whole of why there is a second line.
// It supervises nothing: `brew services` holds one service block per formula and
// this bundle is nine, and UserName is not among the keys Homebrew::Service can
// emit, so the one service that runs unprivileged could not be given its
// account. It cannot set the setuid bit on the POSIX helper: pouring a bottle is
// a tar extraction as an ordinary user and extraction drops the bit, `brew`
// refuses to run as root, and rubocops/caveats.rb raises on a formula that so
// much as recommends setuid. And it cannot depend on a cask. So `brew install
// internkim` delivers files, and `sudo internkim install` makes the box a host.

const (
	// HomebrewTapOwner and HomebrewTapName are what `brew tap` is given. The
	// convention turns them into github.com/<owner>/homebrew-<name>, which is
	// where the formula lives; the bottle comes from our own host through
	// root_url below, which is the part that matters.
	//
	// The tap is named after the organisation and not after this product,
	// because one tap holds every formula the organisation publishes: the
	// companion is a second formula in the same tap rather than a second tap
	// for a person to add. charmbracelet/tap, supabase/tap and mobile-dev-inc/tap
	// are all this shape.
	HomebrewTapOwner = "yeomyeonggeori"
	HomebrewTapName  = "tap"

	// HomebrewReleasePrefix is the object prefix the release registry serves
	// the tarballs under, beside deb/, companion/ and host/.
	HomebrewReleasePrefix = "brew"

	companyPackageDescription = "Run your company's agent, messenger and web app on this computer"
)

// HomebrewTap is what a person types.
func HomebrewTap() string {
	return HomebrewTapOwner + "/" + HomebrewTapName
}

// HomebrewTapRepositoryURL is the repository `brew tap` clones, and the one
// place the rendered formula is committed to.
func HomebrewTapRepositoryURL() string {
	return "https://github.com/" + HomebrewTapOwner + "/homebrew-" + HomebrewTapName
}

// HomebrewBottleRootURL is where a bottle is fetched from. Homebrew appends
// "<name>-<version>.<tag>.bottle.tar.gz" to it for anything that is not GitHub
// Packages (Utils::Bottles.path_resolved_basename). The zone is asked of
// fleetdomain rather than written down, which is what lets a company hosting
// its own releases point the formula at its own address.
func HomebrewBottleRootURL(zone string) string {
	return fleetdomain.Subdomain("updates", zone) + "/" + HomebrewReleasePrefix
}

// HomebrewFormulaFileName is where the formula sits inside the tap.
func HomebrewFormulaFileName() string {
	return "Formula/" + CompanyPackageName + ".rb"
}

// HomebrewSourceTarballName is the tarball the formula's url names, which is
// what a Mac whose prefix no bottle was built for installs from.
func HomebrewSourceTarballName(version string) string {
	return fmt.Sprintf("%s-%s.tar.gz", CompanyPackageName, version)
}

// HomebrewBottleFileName is what Homebrew asks root_url for.
func HomebrewBottleFileName(version string, bottleTag string) string {
	return fmt.Sprintf("%s-%s.%s.bottle.tar.gz", CompanyPackageName, version, bottleTag)
}

// HomebrewBottle is one built bottle: which macOS and architecture it is for,
// which Cellar it was laid out against, and what it hashes to.
type HomebrewBottle struct {
	// Tag is Homebrew's own name for a macOS version and architecture, such as
	// arm64_tahoe. It comes from `brew ruby -e 'puts Utils::Bottles.tag'` on
	// the machine that built it rather than from a table here, because the list
	// grows with every macOS release.
	Tag string
	// Cellar is what the bottle may be poured into. It is :any_skip_relocation
	// because nothing in the keg names the prefix: the programs are static, the
	// interpreter is python-build-standalone and finds itself through
	// @executable_path, and the one thing that would have named an absolute
	// path — the document virtualenv — is built by the install block after the
	// pour. Saying anything else makes Homebrew run fix_dynamic_linkage over
	// the keg, which cannot rewrite a Python wheel's compiled module and fails
	// the pour outright.
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
	formula.WriteString("# Rendered by `internkim release brew` from internal/runtime/blueclaw.\n")
	formula.WriteString("# Edit that package, not this file: a hand edit here is a second\n")
	formula.WriteString("# declaration of the same dependency list.\n")
	formula.WriteString("class " + homebrewClassName(CompanyPackageName) + " < Formula\n")
	formula.WriteString("  desc " + rubyString(companyPackageDescription) + "\n")
	formula.WriteString("  homepage " + rubyString(CompanyPackageHomepage) + "\n")
	formula.WriteString("  url " + rubyString(request.SourceTarballURL) + "\n")
	formula.WriteString("  sha256 " + rubyString(request.SourceSHA256) + "\n")
	formula.WriteString("  license " + rubyString(companyPackageLicense) + "\n")
	formula.WriteString("  version " + rubyString(request.Version) + "\n\n")
	writeHomebrewBottleBlock(formula, request.BottleRootURL, request.Bottles)
	writeHomebrewDependencies(formula)
	writeHomebrewInstallBlock(formula)
	writeHomebrewCaveats(formula)
	writeHomebrewTestBlock(formula)
	formula.WriteString("end\n")
	return formula.String(), nil
}

const (
	companyPackageLicense = "Apache-2.0"

	// The two names the keg and the layout agree on. They are here because the
	// formula's Ruby names them relative to libexec and CompanyHostLayout names
	// them absolutely, and a third spelling is how they drift apart.
	documentVirtualEnvironmentDirectoryName = "document-venv"
	documentRequirementsFileName            = "document-requirements.txt"
	documentWheelDirectoryName              = "document-wheels"
)

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

// Installing the keg is a copy. Building the document virtualenv is not part of
// it, and the reason is that a bottle is poured rather than installed: Homebrew
// runs `install` only when it builds from source, and runs `post_install` after
// either. A virtualenv created in `install` would exist on a Mac that built the
// formula and be missing on every Mac that poured it.
//
// It is built on the machine rather than shipped for two reasons that are both
// about absolute paths. A virtualenv's pyvenv.cfg names its interpreter
// absolutely, so one made at release time names the directory the release built
// in and nothing else. And a Python wheel's compiled module carries no header
// padding, so Homebrew's relocation cannot rewrite its install name and a bottle
// holding one refuses to pour outright: "Updated load commands do not fit in the
// header of …/_anydoc.abi3.so".
//
// Nothing is resolved on the machine, though. The keg carries the exact wheels
// the release resolved, so this is --no-index against a directory: offline,
// deterministic, and the same versions on every Mac that installs the release.
func writeHomebrewInstallBlock(formula *strings.Builder) {
	formula.WriteString("  def install\n")
	formula.WriteString("    bin.install Dir[\"bin/*\"]\n")
	formula.WriteString("    libexec.install Dir[\"libexec/*\"]\n")
	formula.WriteString("  end\n\n")

	formula.WriteString("  def post_install\n")
	formula.WriteString("    venv = libexec/" + rubyString(documentVirtualEnvironmentDirectoryName) + "\n")
	formula.WriteString("    return if (venv/\"bin/python\").exist?\n")
	formula.WriteString("\n")
	formula.WriteString("    system libexec/" + rubyString(PackageResolverName) +
		", \"venv\", \"--python\", libexec/" + rubyString(documentInterpreterDirectoryName+"/bin/python"+DocumentInterpreterMinor) + ", venv\n")
	formula.WriteString("    system libexec/" + rubyString(PackageResolverName) +
		", \"pip\", \"install\", \"--python\", venv/\"bin/python\", \"--no-index\", " +
		"\"--find-links\", libexec/" + rubyString(documentWheelDirectoryName) + ", " +
		"\"--requirements\", libexec/" + rubyString(documentRequirementsFileName) + "\n")
	formula.WriteString("  end\n\n")
}

// The default prefix on Apple silicon. It is named here only so the install
// block can tell "this is the prefix the bottle was resolved against" from
// "this is somewhere else and the interpreter has to be resolved again".
const defaultHomebrewPrefix = "/opt/homebrew"

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
	formula.WriteString("    system libexec/" + rubyString(documentVirtualEnvironmentDirectoryName+"/bin/python") +
		", \"-c\", " + rubyString(documentModulesTheConversionImports) + "\n")
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
