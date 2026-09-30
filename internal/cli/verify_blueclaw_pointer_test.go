package cli

import (
	"strings"
	"testing"
)

func TestAPointerToACommitBlueclawMainLacksIsRefused(t *testing.T) {
	tree := newShippableTree(t)
	commitFile(t, tree.blueclaw, "local.txt", "local")
	gitInDirectory(t, tree.root, "add", ".dependency/blueclaw")
	gitInDirectory(t, tree.root, "commit", "-q", "-m", "bump to an unmerged blueclaw commit")
	errorValue := refuseBackwardsBlueclawPointer(tree.root, "origin/main")
	if errorValue == nil || !strings.Contains(errorValue.Error(), "not on blueclaw's main yet") {
		t.Fatalf("an unmerged pointer was allowed: %v", errorValue)
	}
}

func TestAPointerThatMovesBackwardsIsRefused(t *testing.T) {
	tree := newShippableTree(t)
	previousPointer := blueclawPointerOf(tree.root, "HEAD")
	commitFile(t, tree.blueclaw, "next.txt", "next")
	gitInDirectory(t, tree.blueclaw, "push", "-q", "origin", "main")
	gitInDirectory(t, tree.root, "add", ".dependency/blueclaw")
	gitInDirectory(t, tree.root, "commit", "-q", "-m", "bump")
	gitInDirectory(t, tree.root, "push", "-q", "origin", "main")
	gitInDirectory(t, tree.blueclaw, "checkout", "-q", previousPointer)
	gitInDirectory(t, tree.root, "add", ".dependency/blueclaw")
	gitInDirectory(t, tree.root, "commit", "-q", "-m", "backwards")
	errorValue := refuseBackwardsBlueclawPointer(tree.root, "HEAD~1")
	if errorValue == nil || !strings.Contains(errorValue.Error(), "would move backwards") {
		t.Fatalf("a backwards pointer was allowed: %v", errorValue)
	}
}

func TestAPointerThatMovesForwardIsAllowed(t *testing.T) {
	tree := newShippableTree(t)
	commitFile(t, tree.blueclaw, "next.txt", "next")
	gitInDirectory(t, tree.blueclaw, "push", "-q", "origin", "main")
	gitInDirectory(t, tree.root, "add", ".dependency/blueclaw")
	gitInDirectory(t, tree.root, "commit", "-q", "-m", "bump")
	if errorValue := refuseBackwardsBlueclawPointer(tree.root, "HEAD~1"); errorValue != nil {
		t.Fatal(errorValue)
	}
}
