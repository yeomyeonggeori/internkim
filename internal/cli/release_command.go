package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
)

type releaseSubcommand struct {
	name    string
	summary string
	flags   []string
	run     func(arguments []string) error
}

var releaseSubcommands = []releaseSubcommand{
	{name: "packages", summary: "Build the release directory without publishing it: deb, rpm and archlinux packages, the Homebrew bottle and formula, and their SHA256SUMS", flags: []string{"--format", "--architecture", "--out", "--version"}, run: runReleasePackages},
	{name: "host", summary: "Build the company host for Linux and Apple-silicon Macs and publish it as a GitHub Release on the stable or testing channel; stable also gives the Homebrew tap its formula", flags: []string{"--channel"}, run: runReleaseHost},
}

func runRelease() {
	if len(os.Args) < 3 || isHelpArgument(os.Args[2]) {
		printReleaseUsage()
		return
	}
	subcommand, found := findReleaseSubcommand(os.Args[2])
	if !found {
		printReleaseUsage()
		fatal(fmt.Sprintf("release has no subcommand %q", os.Args[2]))
	}
	arguments := os.Args[3:]
	if slices.ContainsFunc(arguments, isHelpArgument) {
		printReleaseUsage()
		return
	}
	if errorValue := checkReleaseArguments(subcommand, arguments); errorValue != nil {
		fatal(errorValue.Error())
	}
	if errorValue := subcommand.run(arguments); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func findReleaseSubcommand(name string) (releaseSubcommand, bool) {
	for _, subcommand := range releaseSubcommands {
		if subcommand.name == name {
			return subcommand, true
		}
	}
	return releaseSubcommand{}, false
}

func isHelpArgument(argument string) bool {
	return argument == "--help" || argument == "-h"
}

func checkReleaseArguments(subcommand releaseSubcommand, arguments []string) error {
	for index := 0; index < len(arguments); index++ {
		name, _, hasInlineValue := strings.Cut(arguments[index], "=")
		if !slices.Contains(subcommand.flags, name) {
			return fmt.Errorf("release %s does not take %q; it takes %s", subcommand.name, arguments[index], strings.Join(subcommand.flags, ", "))
		}
		if hasInlineValue {
			continue
		}
		if index+1 == len(arguments) {
			return fmt.Errorf("release %s %s needs a value", subcommand.name, name)
		}
		index++
	}
	return nil
}

func printReleaseUsage() {
	names := make([]string, 0, len(releaseSubcommands))
	for _, subcommand := range releaseSubcommands {
		names = append(names, subcommand.name)
	}
	fmt.Printf("Usage: internkim release <%s>\n\n", strings.Join(names, "|"))
	for _, subcommand := range releaseSubcommands {
		fmt.Printf("  %-13s %s\n", subcommand.name, subcommand.summary)
	}
	fmt.Println()
	fmt.Printf("release host publishes through gh, signed in to an account that can create releases on %s.\n", hostReleaseRepository)
}

func releaseBinaryRevision(repositoryRootPath string) string {
	revision := strings.TrimSpace(runCmd("git", "-C", repositoryRootPath, "rev-parse", "--short", "HEAD"))
	if revision == "" {
		return "unknown"
	}
	return revision
}

// The admin gateway's health answer is the only one that carries a revision, so it is
// the only way to see whether an upgrade moved the running process rather than the
// file. A build that leaves these at their defaults answers `unknown` and that check
// can never be made.
func admindStampFlags(buildID string, revision string) string {
	return strings.Join([]string{
		"-X", "github.com/yeomyeonggeori/internkim/internal/admind.BuildID=" + buildID,
		"-X", "github.com/yeomyeonggeori/internkim/internal/admind.GitRevision=" + revision,
	}, " ")
}

func gitRevision(repositoryRootPath string) string {
	return firstNonEmptyString(
		strings.TrimSpace(runCmd("git", "-C", repositoryRootPath, "rev-parse", "HEAD")),
		"unknown",
	)
}

func shortRevision(revision string) string {
	revision = strings.TrimSpace(revision)
	if len(revision) <= 12 {
		return revision
	}
	return revision[:12]
}

// Without the artifact this tree cannot say what a payload it would build is,
// and saying the repository's own revision instead reports a component as
// changed on every commit. Unknown is the honest answer; what is done with it
// is carryComponentsForward's business.
func releaseFileSHA256AndSize(path string) (string, int64, error) {
	file, errorValue := os.Open(path)
	if errorValue != nil {
		return "", 0, errorValue
	}
	defer file.Close()
	hash := sha256.New()
	size, errorValue := io.Copy(hash, file)
	if errorValue != nil {
		return "", 0, errorValue
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

// ReleaseComponentNames is what the deploy command offers, read off the same
// list the release builds from.
// A blob is named by the hash of what is in it, so one that is already
// published is the same bytes and sending them again buys nothing. On a slow
// uplink it costs the whole deploy: a 100 MB component takes minutes at
// 0.5 MB/s and the upload gives up before it lands, over and over, for a
// component that had not changed since the last release.
// Carrying a component forward is free — the blob is addressed by its content
// and is already published — but only honest while it still matches this tree.
// When it does not, the deploy stops and says which component to rebuild,
// rather than shipping a device that half agrees with itself.
// The drift check cannot see this one. A tree with no built payload artifact
// says it has no opinion about the payload, which is honest and is exactly when
// this happens — the payload is carried forward silently while capabilityd is
// replaced.
// A component's revision has to be the revision of what it is built from, or
// nothing can tell a stale one from a current one. chatd is built from the
// blueclaw submodule, and stamping it with this repository's HEAD made a
// submodule that had moved look like no change at all: a device ended up
// running a relay that asked for capabilities its chatd had never heard of,
// and every deploy reported success.
// Go binaries name only their main package. Listing their libraries by hand
// made the revision a guess at what the binary compiles: admind and capabilityd
// both embed the generated capability catalog, so regenerating the protocol
// changed the contract they serve while leaving both revisions untouched, and
// the release guard waved through a device whose halves disagreed.
// git tracks a submodule by its gitlink, so history moves on the plugin
// directory and never on the skills directory inside it.
// Everything the main package compiles from this module, so a change anywhere
// in its dependency graph moves its revision. Asking the toolchain keeps this
// exact as the graph changes; a list kept by hand only stays right until the
// next import.
