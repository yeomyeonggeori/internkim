package setup

import (
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

// mediaBoardConnection answers like a board where everything works, except for
// the one thing a case turns off. Run returns stdout only, exactly as the real
// SSH connection does, so a step that does not observe the board cannot notice
// any of these failures.
type mediaBoardConnection struct {
	machine            string
	binaryInstalls     bool
	credentialsLand    bool
	bucketAppears      bool
	serviceComesUp     bool
	relayEnvironmentOK bool
	commands           []string
}

func aWorkingMediaBoard() *mediaBoardConnection {
	return &mediaBoardConnection{
		machine:            "aarch64",
		binaryInstalls:     true,
		credentialsLand:    true,
		bucketAppears:      true,
		serviceComesUp:     true,
		relayEnvironmentOK: true,
	}
}

func (connection *mediaBoardConnection) Run(command string) string {
	connection.commands = append(connection.commands, command)
	switch {
	case command == "uname -m":
		return connection.machine + "\n"
	case strings.HasPrefix(command, blueclaw.BuzzMediaBinaryPath+" --version"):
		if connection.binaryInstalls {
			return "Version  : v" + blueclaw.BuzzMediaVersion + "\n"
		}
		return ""
	case strings.HasPrefix(command, "test -s "+blueclaw.BuzzMediaEnvironmentFilePath):
		return answerFor(connection.credentialsLand)
	case strings.HasPrefix(command, "test -d "+blueclaw.BuzzMediaBucketPath()):
		return answerFor(connection.bucketAppears)
	case command == "systemctl is-active "+blueclaw.BuzzMediaServiceName:
		if connection.serviceComesUp {
			return "active\n"
		}
		return "failed\n"
	case command == blueclaw.BuzzMediaHealthCheckCommand():
		return answerFor(connection.serviceComesUp)
	case strings.HasPrefix(command, "grep -c '^BUZZ_S3_SECRET_KEY=.'"):
		if connection.relayEnvironmentOK {
			return "1\n"
		}
		return "0\n"
	}
	return ""
}

func answerFor(works bool) string {
	if works {
		return "ok\n"
	}
	return "no\n"
}

func (connection *mediaBoardConnection) SCP(localPath, remotePath string) error {
	return nil
}

func runMediaStep(connection *mediaBoardConnection) error {
	return StepBuzzMedia.Run(&Context{Backend: BackendSSH, SSH: connection})
}

func TestMediaStepSucceedsOnABoardWhereEveryCommandWorks(t *testing.T) {
	if errorValue := runMediaStep(aWorkingMediaBoard()); errorValue != nil {
		t.Fatalf("a board where everything works reported %v", errorValue)
	}
}

// Every one of these ran silently before: the step issued four commands through
// a connection whose Run returns stdout and no error, and returned nil whatever
// came back. A device provisioned against a dead download URL came up with no
// object store and said so nowhere.
func TestMediaStepNamesWhatFailedInsteadOfReportingSuccess(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		breakIt     func(*mediaBoardConnection)
		mustMention []string
	}{
		{
			name:        "the download does not land",
			breakIt:     func(board *mediaBoardConnection) { board.binaryInstalls = false },
			mustMention: []string{blueclaw.BuzzMediaBinaryPath, "versitygw_v" + blueclaw.BuzzMediaVersion, "https://"},
		},
		{
			name:        "the board is an architecture versity does not publish",
			breakIt:     func(board *mediaBoardConnection) { board.machine = "riscv64" },
			mustMention: []string{"riscv64", "aarch64", "x86_64"},
		},
		{
			name:        "the credentials file stays empty",
			breakIt:     func(board *mediaBoardConnection) { board.credentialsLand = false },
			mustMention: []string{blueclaw.BuzzMediaEnvironmentFilePath},
		},
		{
			name:        "the bucket directory is not created",
			breakIt:     func(board *mediaBoardConnection) { board.bucketAppears = false },
			mustMention: []string{blueclaw.BuzzMediaBucketPath()},
		},
		{
			name:        "the service does not come up",
			breakIt:     func(board *mediaBoardConnection) { board.serviceComesUp = false },
			mustMention: []string{blueclaw.BuzzMediaServiceName, "failed"},
		},
		{
			name:        "the relay never learns where the store is",
			breakIt:     func(board *mediaBoardConnection) { board.relayEnvironmentOK = false },
			mustMention: []string{blueclaw.BuzzRelayS3EnvironmentFilePath, "BUZZ_S3_SECRET_KEY"},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			board := aWorkingMediaBoard()
			testCase.breakIt(board)
			errorValue := runMediaStep(board)
			if errorValue == nil {
				t.Fatal("the step reported success on a board that cannot hold attachments")
			}
			for _, fragment := range testCase.mustMention {
				if !strings.Contains(errorValue.Error(), fragment) {
					t.Fatalf("the failure does not name %q, so the reader cannot tell what to fix: %v",
						fragment, errorValue)
				}
			}
		})
	}
}

// The download is the failure that actually happened, so the command that
// performs it is held to carrying a checksum. An unchecked curl is how a
// redirect or a cached error page becomes the media store.
func TestMediaStepVerifiesWhatItDownloads(t *testing.T) {
	board := aWorkingMediaBoard()
	board.binaryInstalls = false
	_ = runMediaStep(board)

	downloadCommand := ""
	for _, command := range board.commands {
		if strings.Contains(command, "curl") && strings.Contains(command, "versitygw") {
			downloadCommand = command
		}
	}
	if downloadCommand == "" {
		t.Fatal("the step never issued a download, so this test reads nothing")
	}
	release, _ := blueclaw.BuzzMediaReleaseFor("aarch64")
	for _, expected := range []string{release.URL, release.SHA256, "sha256sum -c -"} {
		if !strings.Contains(downloadCommand, expected) {
			t.Fatalf("the download command does not carry %q: %s", expected, downloadCommand)
		}
	}
}

// mc is gone with MinIO. The bucket is a directory the posix backend serves,
// so the step creates it with the ownership and mode the MinIO data directory
// carried rather than asking a client to make it.
func TestMediaStepCreatesTheBucketAsADirectoryNoClientMakes(t *testing.T) {
	board := aWorkingMediaBoard()
	if errorValue := runMediaStep(board); errorValue != nil {
		t.Fatal(errorValue)
	}
	issued := strings.Join(board.commands, "\n")
	if strings.Contains(issued, "/usr/local/bin/mc ") || strings.Contains(issued, "mc mb") {
		t.Fatalf("the step still drives mc, which MinIO published and no longer ships:\n%s", issued)
	}
	expected := "install -d -o root -g root -m 750 " +
		blueclaw.BuzzMediaRootPath + " " + blueclaw.BuzzMediaBucketPath()
	if !strings.Contains(issued, expected) {
		t.Fatalf("the step does not create the bucket with %q:\n%s", expected, issued)
	}
}
