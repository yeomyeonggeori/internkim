package admind

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	blueclawruntime "github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func TestWriteFileIfDifferentOnlyRewritesStaleContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "internkim-users-sync")
	file := installedFile{path: path, contents: "current\n", mode: 0o755}

	written, errorValue := writeFileIfDifferent(file)
	if errorValue != nil || !written {
		t.Fatalf("first write = %v, %v; want a write", written, errorValue)
	}
	written, errorValue = writeFileIfDifferent(file)
	if errorValue != nil || written {
		t.Fatalf("second write = %v, %v; want no write", written, errorValue)
	}

	if errorValue := os.WriteFile(path, []byte("stale\n"), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	written, errorValue = writeFileIfDifferent(file)
	if errorValue != nil || !written {
		t.Fatalf("write over stale content = %v, %v; want a write", written, errorValue)
	}
	contents, errorValue := os.ReadFile(path)
	if errorValue != nil || string(contents) != file.contents {
		t.Fatalf("contents = %q, %v", contents, errorValue)
	}
	info, errorValue := os.Stat(path)
	if errorValue != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("mode = %v, %v", info.Mode().Perm(), errorValue)
	}
}

func TestUsersSyncInstallCarriesTheScriptThisReleaseWasBuiltWith(t *testing.T) {
	directory := t.TempDir()
	service := NewService(Configuration{StateDirectory: directory, AdminUIPath: directory})
	commands := [][]string{}
	service.RunCommand = func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		commands = append(commands, append([]string{name}, arguments...))
		return nil, nil
	}

	scriptPath := filepath.Join(directory, "internkim-users-sync")
	if _, errorValue := writeFilesIfDifferent([]installedFile{
		{scriptPath, blueclawruntime.InternKimUsersSyncScript(), 0o755},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	contents, errorValue := os.ReadFile(scriptPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(string(contents), "/api/agent/member") {
		t.Fatal("the installed script does not read the company directory")
	}
	if strings.Contains(string(contents), "/api/users?fleet_id=") {
		t.Fatal("the installed script still reads the fleet user index")
	}

	service.runSystemControl(context.Background(), "daemon-reload")
	if len(commands) != 1 || commands[0][0] != "systemctl" || commands[0][1] != "daemon-reload" {
		t.Fatalf("commands = %#v", commands)
	}
}

func TestUsersSyncStatePathMatchesWhatTheScriptWrites(t *testing.T) {
	if !strings.Contains(blueclawruntime.InternKimUsersSyncScript(), `STATE_PATH="`+blueclawruntime.InternKimUsersSyncStatePath+`"`) {
		t.Fatalf("the installed script no longer writes InternKimUsersSyncStatePath (%s)", blueclawruntime.InternKimUsersSyncStatePath)
	}
	if DefaultConfiguration().UsersSyncStatePath != blueclawruntime.InternKimUsersSyncStatePath {
		t.Fatalf("UsersSyncStatePath default = %s, want %s", DefaultConfiguration().UsersSyncStatePath, blueclawruntime.InternKimUsersSyncStatePath)
	}
}
