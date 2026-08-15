package cli

import (
	"fmt"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

type blueclawRuntimeInstallPlan struct {
	artifacts             []blueclawRuntimeInstallArtifact
	shouldInstallManifest bool
}

type blueclawRuntimeInstallArtifact struct {
	name          string
	remotePath    string
	mode          string
	shouldInstall bool
	reason        string
}

func buildBlueclawRuntimeInstallPlan(
	localManifest blueclaw.RuntimeArtifactManifest,
	remoteManifestDocument string,
	remoteFilePresence map[string]bool,
	isRemoteRootfsContractOK bool,
	isRemoteManifestCurrent bool,
) blueclawRuntimeInstallPlan {
	remoteManifest, _ := blueclaw.ParseRuntimeArtifactManifest([]byte(remoteManifestDocument))
	artifacts := requiredBlueclawRuntimeInstallArtifacts()
	shouldInstallManifest := !isRemoteManifestCurrent
	for artifactIndex, artifact := range artifacts {
		localManifestFile, localError := blueclaw.FindRuntimeArtifactManifestFile(localManifest, artifact.name)
		remoteManifestFile, remoteError := blueclaw.FindRuntimeArtifactManifestFile(remoteManifest, artifact.name)
		shouldInstall, reason := blueclawRuntimeArtifactInstallDecision(
			artifact.name,
			localManifestFile,
			localError,
			remoteManifestFile,
			remoteError,
			remoteFilePresence[artifact.name],
			isRemoteRootfsContractOK,
		)
		artifacts[artifactIndex].shouldInstall = shouldInstall
		artifacts[artifactIndex].reason = reason
		shouldInstallManifest = shouldInstallManifest || shouldInstall
	}
	return blueclawRuntimeInstallPlan{artifacts: artifacts, shouldInstallManifest: shouldInstallManifest}
}

func forceBlueclawRuntimeInstallPlan(installPlan blueclawRuntimeInstallPlan) blueclawRuntimeInstallPlan {
	for artifactIndex := range installPlan.artifacts {
		installPlan.artifacts[artifactIndex].shouldInstall = true
		installPlan.artifacts[artifactIndex].reason = "forced"
	}
	installPlan.shouldInstallManifest = true
	return installPlan
}

func blueclawRuntimeArtifactInstallDecision(
	artifactName string,
	localManifestFile blueclaw.RuntimeArtifactManifestFile,
	localError error,
	remoteManifestFile blueclaw.RuntimeArtifactManifestFile,
	remoteError error,
	isRemoteFilePresent bool,
	isRemoteRootfsContractOK bool,
) (bool, string) {
	if localError != nil || remoteError != nil {
		return true, "manifest entry changed"
	}
	if !isRemoteFilePresent {
		return true, "remote file missing"
	}
	if !strings.EqualFold(localManifestFile.SHA256, remoteManifestFile.SHA256) {
		return true, "checksum changed"
	}
	if artifactName == "rootfs.ext4" && !isRemoteRootfsContractOK {
		return true, "rootfs base contract changed"
	}
	return false, ""
}

func requiredBlueclawRuntimeInstallArtifacts() []blueclawRuntimeInstallArtifact {
	return []blueclawRuntimeInstallArtifact{
		{name: "firecracker", remotePath: blueclaw.BlueclawFirecrackerPath, mode: "0755"},
		{name: "jailer", remotePath: blueclaw.BlueclawJailerPath, mode: "0755"},
		{name: "cloud-hypervisor", remotePath: blueclaw.BlueclawCloudHypervisorPath, mode: "0755"},
		{name: "virtiofsd", remotePath: blueclaw.BlueclawVirtiofsdPath, mode: "0755"},
		{name: "vmlinux.bin", remotePath: blueclaw.BlueclawKernelImagePath, mode: "0644"},
		{name: "rootfs.ext4", remotePath: blueclaw.BlueclawRootFilesystemImagePath, mode: "0644"},
	}
}

func blueclawRuntimeInstallCommand(artifact blueclawRuntimeInstallArtifact, temporaryRemotePath string) string {
	if artifact.name == "rootfs.ext4" {
		nextRemotePath := artifact.remotePath + ".next"
		return "rm -f " + quoteShellValue(nextRemotePath) +
			" && cp --sparse=always " + quoteShellValue(temporaryRemotePath) + " " + quoteShellValue(nextRemotePath) +
			" && chmod " + artifact.mode + " " + quoteShellValue(nextRemotePath) +
			" && mv -f " + quoteShellValue(nextRemotePath) + " " + quoteShellValue(artifact.remotePath)
	}
	return "install -m " + artifact.mode + " " + quoteShellValue(temporaryRemotePath) + " " + quoteShellValue(artifact.remotePath)
}

func blueclawRuntimeInstallTimeout(artifact blueclawRuntimeInstallArtifact) time.Duration {
	if artifact.name == "rootfs.ext4" {
		return 20 * time.Minute
	}
	return 60 * time.Second
}

func blueclawRuntimeInstallPlanIsCurrent(installPlan blueclawRuntimeInstallPlan) bool {
	if installPlan.shouldInstallManifest {
		return false
	}
	for _, artifact := range installPlan.artifacts {
		if artifact.shouldInstall {
			return false
		}
	}
	return true
}

func printBlueclawRuntimeInstallPlan(installPlan blueclawRuntimeInstallPlan) {
	fmt.Println()
	for _, artifact := range installPlan.artifacts {
		if artifact.shouldInstall {
			fmt.Printf("    %s installing (%s)\n", artifact.name, artifact.reason)
			continue
		}
		fmt.Printf("    %s current\n", artifact.name)
	}
	if installPlan.shouldInstallManifest {
		fmt.Println("    manifest installing (manifest changed)")
		return
	}
	fmt.Println("    manifest current")
}

func printBlueclawRuntimeArtifactInstalling(artifact blueclawRuntimeInstallArtifact) {
	if artifact.name == "rootfs.ext4" {
		fmt.Printf("    %s installing (large, this can take several minutes; %s)...\n", artifact.name, artifact.reason)
		return
	}
	fmt.Printf("    %s installing (%s)...\n", artifact.name, artifact.reason)
}

func (state *setupFlowState) remoteBlueclawRuntimeFilePresence() map[string]bool {
	output := state.sshClient.run(`for entry in \
firecracker:/usr/local/bin/firecracker:x \
jailer:/usr/local/bin/jailer:x \
vmlinux.bin:/opt/internkim/blueclaw-runtime/vmlinux.bin:s \
rootfs.ext4:/opt/internkim/blueclaw-runtime/rootfs.ext4:s; do
  name="${entry%%:*}"
  rest="${entry#*:}"
  path="${rest%:*}"
  mode="${entry##*:}"
  if { [ "$mode" = x ] && [ -x "$path" ]; } || { [ "$mode" = s ] && [ -s "$path" ]; }; then
    echo "$name=present"
  else
    echo "$name=missing"
  fi
done`)
	return parseBlueclawRuntimeFilePresence(output)
}

func parseBlueclawRuntimeFilePresence(output string) map[string]bool {
	presence := map[string]bool{}
	for _, line := range strings.Split(output, "\n") {
		name, value, isFound := strings.Cut(strings.TrimSpace(line), "=")
		if !isFound {
			continue
		}
		presence[name] = value == "present"
	}
	return presence
}

func (state *setupFlowState) transferBlueclawRuntimeArtifact(localPath string, remotePath string, artifactName string) error {
	if artifactName == "rootfs.ext4" {
		if errorValue := state.sshClient.rsyncSparse(localPath, remotePath); errorValue != nil {
			return fmt.Errorf("transfer %s with resumable rsync: %w", artifactName, errorValue)
		}
		return nil
	}
	if errorValue := state.sshClient.scp(localPath, remotePath); errorValue != nil {
		return fmt.Errorf("transfer %s: %w", artifactName, errorValue)
	}
	return nil
}
