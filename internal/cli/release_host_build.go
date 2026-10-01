package cli

import (
	"fmt"
	"runtime/debug"
)

type cliBuild struct {
	Revision   string
	IsModified bool
}

func thisCLIBuild() cliBuild {
	information, isRecorded := debug.ReadBuildInfo()
	if !isRecorded {
		return cliBuild{}
	}
	build := cliBuild{}
	for _, setting := range information.Settings {
		switch setting.Key {
		case "vcs.revision":
			build.Revision = setting.Value
		case "vcs.modified":
			build.IsModified = setting.Value == "true"
		}
	}
	return build
}

func staleBuildReason(build cliBuild, head string) string {
	const rebuild = "; run `make build` and release again"
	switch {
	case build.Revision == "":
		return "this internkim records no commit it was built from, and it writes the package's units and scripts from its own code" + rebuild
	case build.Revision != head:
		return fmt.Sprintf("this internkim was built from %s and the tree is at %s, and it writes the package's units and scripts from its own code%s", shortRevision(build.Revision), shortRevision(head), rebuild)
	case build.IsModified:
		return "this internkim was built from a tree with uncommitted changes, which would ship in the package's units and scripts" + rebuild
	}
	return ""
}

func refuseAStaleCLI(repositoryRootPath string) error {
	if reason := staleBuildReason(thisCLIBuild(), gitRevision(repositoryRootPath)); reason != "" {
		return fmt.Errorf("release host refuses to publish: %s", reason)
	}
	return nil
}
