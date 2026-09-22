package blueclaw

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

// Five programs the company host runs are packaged by nobody: the two browsers the
// skills drive, the S3 gateway the messenger stores attachments through, and the two
// toolchains the agent and the document skills shell out to. They arrive as pinned
// downloads, and this is where the pins live. Which five is not decided here:
// host_dependencies.go declares them ArrivesAsPayload and
// TestThePinsCoverExactlyThePayloadProgramsDeclared holds this table to that list in
// both directions, because a program declared and not pinned ships a package
// `internkim install` refuses at its first check.
//
// They used to live in host/Dockerfile, which is the only place that could hold them
// while the image was the only thing that installed them. A .deb cannot read a
// Dockerfile and neither can the device's provisioning step, so the pin is raised here
// and the Dockerfile is held to it by TestTheHostImagePinsWhatTheDeclarationPins. Bump
// a version here; the image that has not followed fails rather than building an image
// carrying a different binary than the package does.
//
// `tools/verify-vendored-downloads` reads this file for the URLs it probes, so a
// release whose upstream has withdrawn a file is found before a box is.
type HostPayloadDownload struct {
	ProgramName string
	Version     string
	URL         string
	SHA256      string
	// PathInsideArchive is empty when the download is the program itself.
	PathInsideArchive string
	// DockerfileVersionArgument and DockerfileChecksumVariable are what
	// host/Dockerfile calls this pin, which is what lets one test read the image
	// in both directions rather than trusting that it was updated too.
	DockerfileVersionArgument  string
	DockerfileChecksumVariable string
}

const (
	deviceBrowserVersion = "1.1.5"
	agentBrowserVersion  = "0.32.3"
	bunVersion           = "1.4.2"
	uvVersion            = "0.11.11"
)

// hostPayloadMachineByDebianArchitecture maps what dpkg calls an architecture to what
// `uname -m` calls it. The package is built per Debian architecture and the device's
// provisioning step asks the board, so both names address the same pins.
var hostPayloadMachineByDebianArchitecture = map[string]string{
	"arm64": "aarch64",
	"amd64": "x86_64",
}

var hostPayloadDownloadsByDebianArchitecture = map[string][]HostPayloadDownload{
	"arm64": {
		deviceBrowserPayload("aarch64-unknown-linux-gnu", "76abe24ad32a42e190dd09d3a475f57b91cda66cd8cbf547f602f3aa28963a7b"),
		agentBrowserPayload("agent-browser-linux-arm64", "87fd2efb67995fc433569f0383260bfee44a785d6d45ca07c77179c45b70de18"),
		mediaServerPayload("arm64", "b34051d33f5a9c457f790896acb7bd7d7e15ad8d92efb70616b924f37e401910"),
		bunPayload("aarch64", "54328bbc2d9c8e0c9f892c544d66c57a83b84139e34909e5ee81758f1ac8fda7"),
		packageResolverPayload("aarch64-unknown-linux-gnu", "155fe4d3b3cb4bfce118ab4b1380f71515ae874d13d9858171b4f9c26e16684d"),
	},
	"amd64": {
		deviceBrowserPayload("x86_64-unknown-linux-gnu", "7128ca9b9f7e7bb5ab58b1c6cbf0910a2e22008f4662ea87bd6b8ab8493e3181"),
		agentBrowserPayload("agent-browser-linux-x64", "243f6e01c4b7dea53ad07d9754df99033c614582d5c685c529a1cb81cafc3ab1"),
		mediaServerPayload("x86_64", "2ba2c734d10d2c4e651d03182cb4b246656bc735a2f282db7b0b73fba6073467"),
		bunPayload("x64", "36368faef7527875d5ffa52e53cd48021741f2a83eb6208a8dd64068d422a913"),
		packageResolverPayload("x86_64-unknown-linux-gnu", "a767848254391855c96df271e9ca8b7f72dd172d310460447853d25d907b9ae0"),
	},
}

func deviceBrowserPayload(targetTriple string, checksum string) HostPayloadDownload {
	return HostPayloadDownload{
		ProgramName:                DeviceBrowserName,
		Version:                    deviceBrowserVersion,
		URL:                        "https://github.com/lexmount/moli/releases/download/v" + deviceBrowserVersion + "/moli-" + targetTriple + ".tar.gz",
		SHA256:                     checksum,
		PathInsideArchive:          "moli-v" + deviceBrowserVersion + "-" + targetTriple + "/moli",
		DockerfileVersionArgument:  "MOLI_VERSION",
		DockerfileChecksumVariable: "moliSHA256",
	}
}

func agentBrowserPayload(assetName string, checksum string) HostPayloadDownload {
	return HostPayloadDownload{
		ProgramName:                AgentBrowserName,
		Version:                    agentBrowserVersion,
		URL:                        "https://github.com/vercel-labs/agent-browser/releases/download/v" + agentBrowserVersion + "/" + assetName,
		SHA256:                     checksum,
		DockerfileVersionArgument:  "AGENT_BROWSER_VERSION",
		DockerfileChecksumVariable: "agentBrowserSHA256",
	}
}

func mediaServerPayload(releaseTarget string, checksum string) HostPayloadDownload {
	assetName := fmt.Sprintf("versitygw_v%s_Linux_%s.tar.gz", BuzzMediaVersion, releaseTarget)
	return HostPayloadDownload{
		ProgramName:                BuzzMediaProgramName,
		Version:                    BuzzMediaVersion,
		URL:                        "https://github.com/versity/versitygw/releases/download/v" + BuzzMediaVersion + "/" + assetName,
		SHA256:                     checksum,
		PathInsideArchive:          strings.TrimSuffix(assetName, ".tar.gz") + "/" + BuzzMediaProgramName,
		DockerfileVersionArgument:  "VERSITYGW_VERSION",
		DockerfileChecksumVariable: "versitygwSHA256",
	}
}

// bun publishes a zip rather than a tarball, which is the only reason anything that
// opens these archives has to ask what kind it is holding.
func bunPayload(machineName string, checksum string) HostPayloadDownload {
	assetName := "bun-linux-" + machineName + ".zip"
	return HostPayloadDownload{
		ProgramName:                BunProgramName,
		Version:                    bunVersion,
		URL:                        "https://github.com/oven-sh/bun/releases/download/bun-v" + bunVersion + "/" + assetName,
		SHA256:                     checksum,
		PathInsideArchive:          strings.TrimSuffix(assetName, ".zip") + "/" + BunProgramName,
		DockerfileVersionArgument:  "BUN_VERSION",
		DockerfileChecksumVariable: "bunSHA256",
	}
}

// uv's tarball carries uvx beside uv; the package installs the one the skills run.
func packageResolverPayload(targetTriple string, checksum string) HostPayloadDownload {
	return HostPayloadDownload{
		ProgramName:                PackageResolverName,
		Version:                    uvVersion,
		URL:                        "https://github.com/astral-sh/uv/releases/download/" + uvVersion + "/uv-" + targetTriple + ".tar.gz",
		SHA256:                     checksum,
		PathInsideArchive:          "uv-" + targetTriple + "/" + PackageResolverName,
		DockerfileVersionArgument:  "UV_VERSION",
		DockerfileChecksumVariable: "uvSHA256",
	}
}

// HostPayloadDownloads is what the host fetches for one Debian architecture. An
// architecture nobody publishes for is an error rather than an empty list, because a
// box that installed none of these answers and does nothing.
func HostPayloadDownloads(debianArchitecture string) ([]HostPayloadDownload, error) {
	downloads, isPublished := hostPayloadDownloadsByDebianArchitecture[debianArchitecture]
	if !isPublished {
		return nil, fmt.Errorf(
			"no vendored binary is pinned for %s; the architectures are %v",
			debianArchitecture, hostPayloadArchitectures())
	}
	return append([]HostPayloadDownload(nil), downloads...), nil
}

func hostPayloadArchitectures() []string {
	architectures := []string{}
	for architecture := range hostPayloadDownloadsByDebianArchitecture {
		architectures = append(architectures, architecture)
	}
	sort.Strings(architectures)
	return architectures
}

// hostPayloadDownloadFor finds one program's pin by the name `uname -m` reports.
func hostPayloadDownloadFor(machine string, programName string) (HostPayloadDownload, bool) {
	for architecture, itsMachine := range hostPayloadMachineByDebianArchitecture {
		if itsMachine != machine {
			continue
		}
		for _, download := range hostPayloadDownloadsByDebianArchitecture[architecture] {
			if download.ProgramName == programName {
				return download, true
			}
		}
	}
	return HostPayloadDownload{}, false
}

// MediaServerRelease is one architecture's pinned versitygw tarball.
type MediaServerRelease struct {
	Machine   string
	AssetName string
	SHA256    string
	URL       string
	// PathInsideArchive is where the program sits in the tarball. versity nests it
	// under a directory named for the release, so an extraction that asks for a bare
	// `versitygw` fails with "Not found in archive" rather than installing anything.
	PathInsideArchive string
}

// BuzzMediaReleaseFor resolves what `uname -m` reports to the tarball this release
// installs. Missing means the board is an architecture versity does not publish, which
// the caller reports instead of installing nothing.
func BuzzMediaReleaseFor(machine string) (MediaServerRelease, bool) {
	download, isPublished := hostPayloadDownloadFor(machine, BuzzMediaProgramName)
	if !isPublished {
		return MediaServerRelease{}, false
	}
	return MediaServerRelease{
		Machine:           machine,
		AssetName:         path.Base(download.URL),
		SHA256:            download.SHA256,
		URL:               download.URL,
		PathInsideArchive: download.PathInsideArchive,
	}, true
}

// BuzzMediaBucketPath is the directory the posix backend serves as the bucket.
func BuzzMediaBucketPath() string {
	return BuzzMediaRootPath + "/" + BuzzMediaBucket
}

// BuzzMediaPublishedMachines lists the architectures this release pins, for error
// messages that have to say what was expected.
func BuzzMediaPublishedMachines() []string {
	machines := []string{}
	for _, architecture := range hostPayloadArchitectures() {
		machine := hostPayloadMachineByDebianArchitecture[architecture]
		if _, isPublished := hostPayloadDownloadFor(machine, BuzzMediaProgramName); isPublished {
			machines = append(machines, machine)
		}
	}
	sort.Strings(machines)
	return machines
}
