package fleet

import "testing"

func TestClassifyWorkspacePath(t *testing.T) {
	testCases := []struct {
		path string
		mode string
	}{
		{"/root/.blueclaw/workspace/skills/presentation/SKILL.md", WorkspaceSyncModeContent},
		{"/workspace/sites/demo/app/dist/index.html", WorkspaceSyncModeContent},
		{"/workspace/.blueclaw/postgres/base/1", WorkspaceSyncModeSealedSnapshot},
		{"/workspace/.blueclaw/graphiti/kuzu/data.kz", WorkspaceSyncModeSealedSnapshot},
		{"/workspace/.blueclaw/runtime/current/bin/blueclaw", WorkspaceSyncModeRuntimeCache},
		{"/workspace/.blueclaw/tmp/socket.sock", WorkspaceSyncModeEphemeral},
		{"/workspace/private/people/user-1/tmp/job-1/file.pdf", WorkspaceSyncModeEphemeral},
		{"/workspace/private/people/user-1/artifacts/report/file.pdf", WorkspaceSyncModeContent},
		{"/workspace/sessions/session-1/transcript.jsonl", WorkspaceSyncModeAppendLog},
		{"/workspace/sessions/session-1/scratch.tmp", WorkspaceSyncModeEphemeral},
	}

	for _, testCase := range testCases {
		if mode := ClassifyWorkspacePath(testCase.path); mode != testCase.mode {
			t.Fatalf("expected %s for %s, got %s", testCase.mode, testCase.path, mode)
		}
	}
}

func TestNewWorkspaceChangeBundleIsStableAndClassifiesPaths(t *testing.T) {
	firstBundle := NewWorkspaceChangeBundle("head-1", []WorkspacePathChange{
		{Path: "/workspace/sites/demo/app/dist/index.html", ContentHash: "sha256-a"},
		{Path: "/workspace/.blueclaw/postgres/base/1", ContentHash: "sha256-b"},
	})
	secondBundle := NewWorkspaceChangeBundle("head-1", []WorkspacePathChange{
		{Path: "/root/.blueclaw/workspace/.blueclaw/postgres/base/1", ContentHash: "sha256-b"},
		{Path: "sites/demo/app/dist/index.html", ContentHash: "sha256-a"},
	})

	if firstBundle.BundleID != secondBundle.BundleID {
		t.Fatalf("expected stable bundle id, got %q and %q", firstBundle.BundleID, secondBundle.BundleID)
	}
	if firstBundle.Changes[0].Mode != WorkspaceSyncModeSealedSnapshot && firstBundle.Changes[1].Mode != WorkspaceSyncModeSealedSnapshot {
		t.Fatalf("expected one sealed snapshot change, got %+v", firstBundle.Changes)
	}
}
