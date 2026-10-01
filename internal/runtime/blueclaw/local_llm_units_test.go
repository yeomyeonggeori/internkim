package blueclaw

import (
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/locallm"
)

func TestTheRenderedModelUnitsAreWhatDriftDetectionExpects(t *testing.T) {
	units := map[string]string{
		locallm.LlamaCppServicePath:          LlamaCppServiceUnit(),
		locallm.LlamaCppEmbeddingServicePath: LlamaCppEmbeddingServiceUnit(),
	}

	if drift := locallm.SetupDrift(func(path string) ([]byte, error) { return []byte(units[path]), nil }); len(drift) != 0 {
		t.Fatalf("freshly rendered units must not read as drift: %v", drift)
	}
}
