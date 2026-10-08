package blueclaw

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

// The programs the company host runs that are packaged by nobody: the browsers the
// skills drive, the S3 gateway the messenger stores attachments through, the
// toolchains the agent and the document skills shell out to, and the llama.cpp server
// that embeds every memory and skill. They arrive as pinned downloads, and this is
// where the pins live, with the embedding model's weights beside them. Which programs
// is not decided here: host_dependencies.go declares them ArrivesAsPayload and
// TestThePinsCoverExactlyThePayloadProgramsDeclared holds this table to that list in
// both directions, because a program declared and not pinned ships a package
// `internkim install` refuses at its first check.
//
// `tools/verify-vendored-downloads` reads this file for the URLs it probes, so a
// release whose upstream has withdrawn a file is found before a box is.
type HostPayloadDownload struct {
	ProgramName string
	Version     string
	URL         string
	SHA256      string
	// PathInsideArchive is empty when the download is the program itself.
	PathInsideArchive      string
	DirectoryInsideArchive string
}

const (
	deviceBrowserVersion = "1.1.5"
	agentBrowserVersion  = "0.32.3"
	bunVersion           = "1.4.2"
	uvVersion            = "0.11.11"
	llamaCppVersion      = "b11476"
)

const (
	embeddingModelRepository = "ggml-org/embeddinggemma-2-GGUF"
	embeddingModelRevision   = "bfcd298762cc34d0357ece5ebdd31791a3a374d8"
	embeddingModelFileName   = "embeddinggemma-2-Q8_0.gguf"
	embeddingModelSHA256     = "2188ac1deca4b77dffefd603c2776a9d76d9d74ec01841392982ebb840b09135"
)

// The machines this release pins a binary for. A target names the operating
// system as well as the architecture, because most of these programs publish
// a differently named asset per platform and one of them capitalises it
// differently too.
const (
	HostPayloadLinuxArm64  = "linux-arm64"
	HostPayloadLinuxAmd64  = "linux-amd64"
	HostPayloadDarwinArm64 = "darwin-arm64"
)

// hostPayloadTargetByDebianArchitecture maps what dpkg calls an architecture to
// the target that carries its pins. The .deb is built per Debian architecture
// and there is no Debian package for a Mac, so this covers the Linux ones alone.
var hostPayloadTargetByDebianArchitecture = map[string]string{
	"arm64": HostPayloadLinuxArm64,
	"amd64": HostPayloadLinuxAmd64,
}

// hostPayloadMachineByTarget maps a target to what `uname -m` calls it on the
// board the device's provisioning step asks.
var hostPayloadMachineByTarget = map[string]string{
	HostPayloadLinuxArm64: "aarch64",
	HostPayloadLinuxAmd64: "x86_64",
}

var hostPayloadDownloadsByTarget = map[string][]HostPayloadDownload{
	HostPayloadLinuxArm64: {
		deviceBrowserPayload("aarch64-unknown-linux-gnu", "76abe24ad32a42e190dd09d3a475f57b91cda66cd8cbf547f602f3aa28963a7b"),
		agentBrowserPayload("agent-browser-linux-arm64", "87fd2efb67995fc433569f0383260bfee44a785d6d45ca07c77179c45b70de18"),
		mediaServerPayload("Linux", "arm64", "b34051d33f5a9c457f790896acb7bd7d7e15ad8d92efb70616b924f37e401910"),
		bunPayload("linux-aarch64", "54328bbc2d9c8e0c9f892c544d66c57a83b84139e34909e5ee81758f1ac8fda7"),
		packageResolverPayload("aarch64-unknown-linux-gnu", "155fe4d3b3cb4bfce118ab4b1380f71515ae874d13d9858171b4f9c26e16684d"),
	},
	HostPayloadLinuxAmd64: {
		deviceBrowserPayload("x86_64-unknown-linux-gnu", "7128ca9b9f7e7bb5ab58b1c6cbf0910a2e22008f4662ea87bd6b8ab8493e3181"),
		agentBrowserPayload("agent-browser-linux-x64", "243f6e01c4b7dea53ad07d9754df99033c614582d5c685c529a1cb81cafc3ab1"),
		mediaServerPayload("Linux", "x86_64", "2ba2c734d10d2c4e651d03182cb4b246656bc735a2f282db7b0b73fba6073467"),
		bunPayload("linux-x64", "36368faef7527875d5ffa52e53cd48021741f2a83eb6208a8dd64068d422a913"),
		packageResolverPayload("x86_64-unknown-linux-gnu", "a767848254391855c96df271e9ca8b7f72dd172d310460447853d25d907b9ae0"),
	},
	HostPayloadDarwinArm64: {
		deviceBrowserPayload("aarch64-apple-darwin", "8ce3bff3d003b4e04b366908ad14656456e1bf347b2be251b734cf1d60d654a6"),
		agentBrowserPayload("agent-browser-darwin-arm64", "b639605f496b629ebb2cdab30f1e070e004efd945b9cd0baf1981acfab64a151"),
		mediaServerPayload("Darwin", "arm64", "4953096f65a9c0d62ab184fb6b2ba7c2435229205cf00a56cb62cd4bf6b216ca"),
		bunPayload("darwin-aarch64", "90987a3a16d7db556d886ac3d551e7b6d3edf0a1cf43acaed622e8676be1d12f"),
		packageResolverPayload("aarch64-apple-darwin", "3a185bf8f46a7b7c8b910d111825907b1638d0ae503cb3c333ae205772354046"),
		embeddingServerPayload("macos-arm64", "577634a1b8a59e8dabe02ba10de1e610be0574dfaf1cf3020e6dd42853ed877e"),
	},
}

func deviceBrowserPayload(targetTriple string, checksum string) HostPayloadDownload {
	return HostPayloadDownload{
		ProgramName:       DeviceBrowserName,
		Version:           deviceBrowserVersion,
		URL:               "https://github.com/lexmount/moli/releases/download/v" + deviceBrowserVersion + "/moli-" + targetTriple + ".tar.gz",
		SHA256:            checksum,
		PathInsideArchive: "moli-v" + deviceBrowserVersion + "-" + targetTriple + "/moli",
	}
}

func agentBrowserPayload(assetName string, checksum string) HostPayloadDownload {
	return HostPayloadDownload{
		ProgramName: AgentBrowserName,
		Version:     agentBrowserVersion,
		URL:         "https://github.com/vercel-labs/agent-browser/releases/download/v" + agentBrowserVersion + "/" + assetName,
		SHA256:      checksum,
	}
}

func mediaServerPayload(operatingSystem string, releaseTarget string, checksum string) HostPayloadDownload {
	assetName := fmt.Sprintf("versitygw_v%s_%s_%s.tar.gz", BuzzMediaVersion, operatingSystem, releaseTarget)
	return HostPayloadDownload{
		ProgramName:       BuzzMediaProgramName,
		Version:           BuzzMediaVersion,
		URL:               "https://github.com/versity/versitygw/releases/download/v" + BuzzMediaVersion + "/" + assetName,
		SHA256:            checksum,
		PathInsideArchive: strings.TrimSuffix(assetName, ".tar.gz") + "/" + BuzzMediaProgramName,
	}
}

// bun publishes a zip rather than a tarball, which is the only reason anything that
// opens these archives has to ask what kind it is holding.
func bunPayload(platformName string, checksum string) HostPayloadDownload {
	assetName := "bun-" + platformName + ".zip"
	return HostPayloadDownload{
		ProgramName:       BunProgramName,
		Version:           bunVersion,
		URL:               "https://github.com/oven-sh/bun/releases/download/bun-v" + bunVersion + "/" + assetName,
		SHA256:            checksum,
		PathInsideArchive: strings.TrimSuffix(assetName, ".zip") + "/" + BunProgramName,
	}
}

// uv's tarball carries uvx beside uv; the package installs the one the skills run.
func packageResolverPayload(targetTriple string, checksum string) HostPayloadDownload {
	return HostPayloadDownload{
		ProgramName:       PackageResolverName,
		Version:           uvVersion,
		URL:               "https://github.com/astral-sh/uv/releases/download/" + uvVersion + "/uv-" + targetTriple + ".tar.gz",
		SHA256:            checksum,
		PathInsideArchive: "uv-" + targetTriple + "/" + PackageResolverName,
	}
}

func embeddingServerPayload(assetTarget string, checksum string) HostPayloadDownload {
	return HostPayloadDownload{
		ProgramName:            EmbeddingServerProgramName,
		Version:                llamaCppVersion,
		URL:                    "https://github.com/ggml-org/llama.cpp/releases/download/" + llamaCppVersion + "/llama-" + llamaCppVersion + "-bin-" + assetTarget + ".tar.gz",
		SHA256:                 checksum,
		DirectoryInsideArchive: "llama-" + llamaCppVersion,
	}
}

func HostEmbeddingModelDownload() HostPayloadDownload {
	return HostPayloadDownload{
		ProgramName: embeddingModelFileName,
		Version:     embeddingModelRevision,
		URL:         "https://huggingface.co/" + embeddingModelRepository + "/resolve/" + embeddingModelRevision + "/" + embeddingModelFileName,
		SHA256:      embeddingModelSHA256,
	}
}

// HostProgramsBuiltFromSource are payload programs a Linux package carries from
// tools/prepare-llama-server instead of from a pin, because the upstream Linux
// build needs a newer glibc than HostGlibcMinimum.
func HostProgramsBuiltFromSource() []string {
	return []string{EmbeddingServerProgramName}
}

// EmbeddingServerArtifactPathFor is where tools/prepare-llama-server leaves the
// server built for one Debian architecture.
func EmbeddingServerArtifactPathFor(debianArchitecture string) string {
	return ".dependency/llama-server-linux-" + debianArchitecture
}

// HostPayloadDownloads is what the host fetches for one Debian architecture. An
// architecture nobody publishes for is an error rather than an empty list, because a
// box that installed none of these answers and does nothing.
func HostPayloadDownloads(debianArchitecture string) ([]HostPayloadDownload, error) {
	target, errorValue := HostPayloadTargetForDebianArchitecture(debianArchitecture)
	if errorValue != nil {
		return nil, errorValue
	}
	return HostPayloadDownloadsForTarget(target)
}

// HostPayloadTargetForDebianArchitecture names the target whose pins a .deb of
// one architecture carries.
func HostPayloadTargetForDebianArchitecture(debianArchitecture string) (string, error) {
	target, isPublished := hostPayloadTargetByDebianArchitecture[debianArchitecture]
	if !isPublished {
		return "", fmt.Errorf(
			"no vendored binary is pinned for %s; the architectures are %v",
			debianArchitecture, hostPayloadDebianArchitectures())
	}
	return target, nil
}

// HostPayloadDownloadsForTarget is the same programs for a machine named by
// operating system as well as architecture, which is what the Homebrew release
// asks for: the Darwin assets are named differently from the Linux ones and one
// of them capitalises the platform.
func HostPayloadDownloadsForTarget(target string) ([]HostPayloadDownload, error) {
	downloads, isPublished := hostPayloadDownloadsByTarget[target]
	if !isPublished {
		return nil, fmt.Errorf(
			"no vendored binary is pinned for %s; the targets are %v", target, hostPayloadTargets())
	}
	return append([]HostPayloadDownload(nil), downloads...), nil
}

func hostPayloadTargets() []string {
	targets := []string{}
	for target := range hostPayloadDownloadsByTarget {
		targets = append(targets, target)
	}
	sort.Strings(targets)
	return targets
}

func hostPayloadDebianArchitectures() []string {
	architectures := []string{}
	for architecture := range hostPayloadTargetByDebianArchitecture {
		architectures = append(architectures, architecture)
	}
	sort.Strings(architectures)
	return architectures
}

// hostPayloadDownloadFor finds one program's pin by the name `uname -m` reports.
func hostPayloadDownloadFor(machine string, programName string) (HostPayloadDownload, bool) {
	for target, itsMachine := range hostPayloadMachineByTarget {
		if itsMachine != machine {
			continue
		}
		for _, download := range hostPayloadDownloadsByTarget[target] {
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
	for _, target := range hostPayloadTargets() {
		machine, isALinuxBoard := hostPayloadMachineByTarget[target]
		if !isALinuxBoard {
			continue
		}
		if _, isPublished := hostPayloadDownloadFor(machine, BuzzMediaProgramName); isPublished {
			machines = append(machines, machine)
		}
	}
	sort.Strings(machines)
	return machines
}
