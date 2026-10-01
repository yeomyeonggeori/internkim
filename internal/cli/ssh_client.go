package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type sshClient struct {
	user string
	pass string
	host string
	port string
}

func newSSH(user, pass, host string) *sshClient {
	return &sshClient{user: user, pass: pass, host: host, port: "22"}
}

func requireSSHPass() error {
	if _, errorValue := exec.LookPath("sshpass"); errorValue != nil {
		return errors.New("password SSH needs sshpass on PATH: brew install sshpass, or apt install sshpass")
	}
	return nil
}

func (s *sshClient) sshArgs(extra ...string) []string {
	base := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=10",
		"-o", "LogLevel=ERROR",
		"-p", s.port,
	}
	if s.pass != "" {
		base = append(base, "-o", "PreferredAuthentications=password", "-o", "PubkeyAuthentication=no")
	}
	return append(base, extra...)
}

func (s *sshClient) run(cmd string) string {
	out, _ := s.runResult(cmd)
	return out
}

func (s *sshClient) runResult(cmd string) (string, error) {
	return s.runResultWithTimeout(cmd, 60*time.Second)
}

func (s *sshClient) runResultWithTimeout(cmd string, timeout time.Duration) (string, error) {
	var args []string
	remoteCommand := s.privilegedCommand(cmd)
	if s.pass == "" {
		args = s.sshArgs(fmt.Sprintf("%s@%s", s.user, s.host), remoteCommand)
		return runSSHCommandWithRetry("ssh", args, timeout)
	}

	if errorValue := requireSSHPass(); errorValue != nil {
		return "", errorValue
	}
	args = append([]string{"-p", s.pass, "ssh"}, s.sshArgs(fmt.Sprintf("%s@%s", s.user, s.host), remoteCommand)...)
	return runSSHCommandWithRetry("sshpass", args, timeout)
}

func (s *sshClient) runInteractiveSSH(remoteArguments []string) error {
	target := fmt.Sprintf("%s@%s", s.user, s.host)
	commandName := "ssh"
	commandArguments := append(s.sshArgs(target), remoteArguments...)
	if s.pass != "" {
		if errorValue := requireSSHPass(); errorValue != nil {
			return errorValue
		}
		commandName = "sshpass"
		commandArguments = append([]string{"-p", s.pass, "ssh"}, append(s.sshArgs(target), remoteArguments...)...)
	}
	command := exec.Command(commandName, commandArguments...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func retryWhileSSHFailureIsTransient(attempt func() (string, error)) (string, error) {
	output := ""
	errorValue := retryOperation(retryOptions{
		AttemptCount: 8,
		DelayForAttempt: func(attemptIndex int) time.Duration {
			return time.Duration(attemptIndex+1) * time.Second
		},
		ShouldRetry: func(errorValue error) bool {
			return errorValue != nil && isRetryableSSHFailure(output)
		},
		SleepAfterFinalAttempt: true,
	}, func(int) error {
		attemptOutput, attemptError := attempt()
		output = attemptOutput
		return attemptError
	})
	return output, errorValue
}

func runSSHCommandWithRetry(commandName string, arguments []string, timeout time.Duration) (string, error) {
	return retryWhileSSHFailureIsTransient(func() (string, error) {
		commandContext, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		output, errorValue := exec.CommandContext(commandContext, commandName, arguments...).CombinedOutput()
		if commandContext.Err() == context.DeadlineExceeded {
			return string(output) + "\nssh command timed out", fmt.Errorf("ssh command timed out after %s: %w", timeout, commandContext.Err())
		}
		return string(output), errorValue
	})
}

func isRetryableSSHFailure(output string) bool {
	for _, phrase := range []string{
		"Connection refused",
		"Operation timed out",
		"Connection timed out",
		"no route to host",
		"Network is unreachable",
		"Permission denied, please try again.",
		"ssh command timed out",
	} {
		if strings.Contains(output, phrase) {
			return true
		}
	}
	return false
}

func (s *sshClient) privilegedCommand(command string) string {
	if s.user == "root" {
		return command
	}
	if s.pass != "" {
		return "printf '%s\n' " + quoteShellValue(s.pass) + " | sudo -S -p '' bash -lc " + quoteShellValue(command)
	}
	return "sudo -p '' bash -lc " + quoteShellValue(command)
}

func (s *sshClient) scpArgs(extra ...string) []string {
	base := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=10",
		"-o", "LogLevel=ERROR",
		"-P", s.port,
	}
	if s.pass != "" {
		base = append(base, "-o", "PreferredAuthentications=password", "-o", "PubkeyAuthentication=no")
	}
	return append(base, extra...)
}

const resumableUploadMinimumBytes = 16 * 1024 * 1024

func (s *sshClient) scp(localPath, remotePath string) error {
	if fileInfo, errorValue := os.Stat(localPath); errorValue == nil && fileInfo.Size() >= resumableUploadMinimumBytes {
		return s.rsyncSparse(localPath, remotePath)
	}
	if s.user != "root" && strings.HasPrefix(remotePath, "/") {
		temporaryRemotePath := temporaryUploadPath(remotePath)
		if err := s.scpDirect(localPath, temporaryRemotePath); err != nil {
			return err
		}
		output, err := s.runResult(moveUploadedPathCommand(temporaryRemotePath, remotePath))
		if err != nil {
			return fmt.Errorf("move uploaded file to %s: %s: %w", remotePath, strings.TrimSpace(output), err)
		}
		return nil
	}
	return s.scpDirect(localPath, remotePath)
}

func temporaryUploadPath(remotePath string) string {
	return "/tmp/internkim-upload-" + filepath.Base(remotePath)
}

func (s *sshClient) rsyncSparse(localPath string, remotePath string) error {
	uploadRemotePath := remotePath
	if s.user != "root" && strings.HasPrefix(remotePath, "/") {
		uploadRemotePath = temporaryUploadPath(remotePath)
	}
	target := fmt.Sprintf("%s@%s:%s", s.user, s.host, uploadRemotePath)
	if uploadRemotePath != remotePath {
		if output, errorValue := s.runResult("rm -f " + quoteShellValue(uploadRemotePath)); errorValue != nil {
			return fmt.Errorf("remove stale upload file %s: %s: %w", uploadRemotePath, strings.TrimSpace(output), errorValue)
		}
	}
	if errorValue := s.runRsyncSparse(localPath, remotePath, target); errorValue != nil {
		return errorValue
	}
	if uploadRemotePath == remotePath {
		return nil
	}
	moveOutput, errorValue := s.runResult(moveUploadedPathCommand(uploadRemotePath, remotePath))
	if errorValue != nil {
		return fmt.Errorf("move uploaded file to %s: %s: %w", remotePath, strings.TrimSpace(moveOutput), errorValue)
	}
	return nil
}

func (s *sshClient) runRsyncSparse(localPath string, remotePath string, target string) error {
	if s.pass != "" {
		if errorValue := requireSSHPass(); errorValue != nil {
			return errorValue
		}
	}
	output, errorValue := retryWhileSSHFailureIsTransient(func() (string, error) {
		return runCommandWithLiveOutput(s.rsyncSparseCommand(localPath, target))
	})
	if errorValue != nil {
		return fmt.Errorf("rsync sparse %s to %s failed: %s: %w", localPath, remotePath, strings.TrimSpace(output), errorValue)
	}
	return nil
}

func (s *sshClient) rsyncSparseCommand(localPath string, target string) *exec.Cmd {
	if s.pass == "" {
		return exec.Command("rsync", rsyncSparseArguments(s.rsyncSSHCommand("ssh"), localPath, target)...)
	}
	sshCommand := s.rsyncSSHCommand("sshpass -e ssh")
	command := exec.Command("rsync", rsyncSparseArguments(sshCommand, localPath, target)...)
	command.Env = append(os.Environ(), "SSHPASS="+s.pass)
	return command
}

func moveUploadedPathCommand(sourcePath string, targetPath string) string {
	return fmt.Sprintf(
		"mkdir -p %s && mv %s %s",
		quoteShellValue(filepath.Dir(targetPath)),
		quoteShellValue(sourcePath),
		quoteShellValue(targetPath),
	)
}

func rsyncSparseArguments(sshCommand string, localPath string, target string) []string {
	return []string{"-azSh", "--partial", "--progress", "-e", sshCommand, localPath, target}
}

func (s *sshClient) rsyncSSHCommand(commandName string) string {
	command := commandName + " -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=10 -o LogLevel=ERROR -p " + s.port
	if s.pass != "" {
		command += " -o PreferredAuthentications=password -o PubkeyAuthentication=no"
	}
	return command
}

func runCommandWithLiveOutput(command *exec.Cmd) (string, error) {
	var output bytes.Buffer
	command.Stdout = io.MultiWriter(os.Stdout, &output)
	command.Stderr = io.MultiWriter(os.Stderr, &output)
	errorValue := command.Run()
	return output.String(), errorValue
}

func (s *sshClient) scpDirect(localPath, remotePath string) error {
	target := fmt.Sprintf("%s@%s:%s", s.user, s.host, remotePath)
	if s.pass != "" {
		if errorValue := requireSSHPass(); errorValue != nil {
			return errorValue
		}
		output, err := runSSHCommandWithRetry("sshpass", append([]string{"-p", s.pass, "scp"}, s.scpArgs(localPath, target)...), 60*time.Second)
		if err != nil {
			return fmt.Errorf("scp %s to %s failed: %s: %w", localPath, remotePath, strings.TrimSpace(output), err)
		}
		return nil
	}
	output, err := runSSHCommandWithRetry("scp", s.scpArgs(localPath, target), 60*time.Second)
	if err != nil {
		return fmt.Errorf("scp %s to %s failed: %s: %w", localPath, remotePath, strings.TrimSpace(output), err)
	}
	return nil
}

func (s *sshClient) scpDir(localDir, remoteDir string) error {
	if s.user != "root" && strings.HasPrefix(remoteDir, "/") {
		temporaryRemoteDirectory := fmt.Sprintf("/tmp/internkim-upload-%d-%s", os.Getpid(), filepath.Base(remoteDir))
		return s.uploadDirectoryArchive(localDir, temporaryRemoteDirectory, remoteDir)
	}
	return s.scpDirDirect(localDir, remoteDir)
}

func (s *sshClient) uploadDirectoryArchive(localDir string, temporaryRemoteDirectory string, remoteDir string) error {
	extractCommand := fmt.Sprintf(
		"rm -rf %s && mkdir -p %s && tar -xzf - -C %s",
		quoteShellValue(temporaryRemoteDirectory),
		quoteShellValue(temporaryRemoteDirectory),
		quoteShellValue(temporaryRemoteDirectory),
	)
	if output, errorValue := s.runTarToRemote(localDir, extractCommand); errorValue != nil {
		return fmt.Errorf("upload directory %s to %s failed: %s: %w", localDir, remoteDir, strings.TrimSpace(output), errorValue)
	}
	output, errorValue := s.runResult(fmt.Sprintf(
		"mkdir -p %s && cp -a %s/. %s/ && rm -rf %s",
		quoteShellValue(remoteDir),
		quoteShellValue(temporaryRemoteDirectory),
		quoteShellValue(remoteDir),
		quoteShellValue(temporaryRemoteDirectory),
	))
	if errorValue != nil {
		return fmt.Errorf("move uploaded directory to %s: %s: %w", remoteDir, strings.TrimSpace(output), errorValue)
	}
	return nil
}

func (s *sshClient) runTarToRemote(localDir string, remoteCommand string) (string, error) {
	var output string
	var errorValue error
	errorValue = retryOperation(retryOptions{
		AttemptCount: 8,
		DelayForAttempt: func(attemptIndex int) time.Duration {
			return time.Duration(attemptIndex+1) * time.Second
		},
		ShouldRetry: func(errorValue error) bool {
			return errorValue != nil && isRetryableSSHFailure(output)
		},
		SleepAfterFinalAttempt: true,
	}, func(attemptIndex int) error {
		output, errorValue = s.runTarToRemoteOnce(localDir, remoteCommand)
		return errorValue
	})
	return output, errorValue
}

func (s *sshClient) runTarToRemoteOnce(localDir string, remoteCommand string) (string, error) {
	tarCommand := exec.Command("tar", "-czf", "-", "-C", localDir, ".")
	tarCommand.Env = append(os.Environ(), "COPYFILE_DISABLE=1")
	tarOutput, errorValue := tarCommand.StdoutPipe()
	if errorValue != nil {
		return "", errorValue
	}
	target := fmt.Sprintf("%s@%s", s.user, s.host)
	commandName := "ssh"
	commandArguments := s.sshArgs(target, remoteCommand)
	if s.pass != "" {
		if errorValue := requireSSHPass(); errorValue != nil {
			return "", errorValue
		}
		commandName = "sshpass"
		commandArguments = append([]string{"-e", "ssh"}, commandArguments...)
	}
	sshCommand := exec.Command(commandName, commandArguments...)
	if s.pass != "" {
		sshCommand.Env = append(os.Environ(), "SSHPASS="+s.pass)
	}
	sshCommand.Stdin = tarOutput
	var output bytes.Buffer
	sshCommand.Stdout = &output
	sshCommand.Stderr = &output
	if errorValue := sshCommand.Start(); errorValue != nil {
		return output.String(), errorValue
	}
	_ = tarOutput.Close()
	if errorValue := tarCommand.Start(); errorValue != nil {
		_ = sshCommand.Process.Kill()
		return output.String(), errorValue
	}
	tarError := tarCommand.Wait()
	sshError := sshCommand.Wait()
	if tarError != nil {
		return output.String(), tarError
	}
	return output.String(), sshError
}

func (s *sshClient) scpDirDirect(localDir, remoteDir string) error {
	target := fmt.Sprintf("%s@%s:%s", s.user, s.host, remoteDir)
	if s.pass != "" {
		if errorValue := requireSSHPass(); errorValue != nil {
			return errorValue
		}
		output, err := runSSHCommandWithRetry("sshpass", append([]string{"-p", s.pass, "scp", "-r"}, s.scpArgs(localDir+"/.", target)...), 60*time.Second)
		if err != nil {
			return fmt.Errorf("scp directory %s to %s failed: %s: %w", localDir, remoteDir, strings.TrimSpace(output), err)
		}
		return nil
	}
	output, err := runSSHCommandWithRetry("scp", append([]string{"-r"}, s.scpArgs(localDir+"/.", target)...), 60*time.Second)
	if err != nil {
		return fmt.Errorf("scp directory %s to %s failed: %s: %w", localDir, remoteDir, strings.TrimSpace(output), err)
	}
	return nil
}
