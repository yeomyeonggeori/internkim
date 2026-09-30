package cli

import "testing"

func TestEveryReleaseSubcommandSaysWhatItDoes(t *testing.T) {
	for _, subcommand := range releaseSubcommands {
		if subcommand.summary == "" {
			t.Errorf("release %s has no summary, so `internkim release` lists it with nothing beside it", subcommand.name)
		}
	}
}
