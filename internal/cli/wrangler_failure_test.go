package cli

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestWranglerSaysWhatItSaidWhenItSaidAnything(t *testing.T) {
	failure := wranglerFailure("put", "blobs/one", "/bin/wrangler", []byte("bucket not found"), errors.New("exit status 1"))

	if !strings.Contains(failure.Error(), "bucket not found") {
		t.Fatalf("the tool's own words are the answer, got %q", failure.Error())
	}
}

// A clean checkout has no web/node_modules and usually no wrangler on PATH, and
// the failure that follows carried no output at all.
func TestWranglerThatCannotBeRunSaysSo(t *testing.T) {
	failure := wranglerFailure("put", "blobs/one", "wrangler", nil, exec.ErrNotFound)

	for _, expected := range []string{"could not be run", "INTERNKIM_RELEASE_WRANGLER_BIN", "web/node_modules"} {
		if !strings.Contains(failure.Error(), expected) {
			t.Fatalf("the refusal does not say %q: %q", expected, failure.Error())
		}
	}
}

func TestWranglerThatSaidNothingIsNotReportedAsSilence(t *testing.T) {
	failure := wranglerFailure("delete", "blobs/one", "/bin/wrangler", []byte("   "), errors.New("signal: killed"))

	if !strings.Contains(failure.Error(), "signal: killed") {
		t.Fatalf("a tool that exits without a word still names how it exited, got %q", failure.Error())
	}
}
