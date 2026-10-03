package cli

import (
	"errors"
	"testing"
)

func TestVerifyReleaseTreeReportsWhatTheReleaseCheckRefuses(t *testing.T) {
	original := checkReleaseTree
	defer func() { checkReleaseTree = original }()
	refusal := errors.New("HEAD is not on origin/main")
	checkReleaseTree = func(string) error { return refusal }
	if errorValue := runVerifyArguments([]string{"release-tree"}); !errors.Is(errorValue, refusal) {
		t.Fatalf("verify release-tree answered %v, want the release check's refusal", errorValue)
	}
}

func TestVerifyReleaseTreePassesWhenTheReleaseCheckDoes(t *testing.T) {
	original := checkReleaseTree
	defer func() { checkReleaseTree = original }()
	checkReleaseTree = func(string) error { return nil }
	if errorValue := runVerifyArguments([]string{"release-tree"}); errorValue != nil {
		t.Fatalf("verify release-tree answered %v on a tree the release check accepts", errorValue)
	}
}
