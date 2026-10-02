package cli

import (
	"fmt"
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
