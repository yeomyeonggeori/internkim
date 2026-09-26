package locallm

import (
	"errors"
	"strings"
	"testing"
)

func TestSetupDriftNamesAServiceStillServingAnOldModel(t *testing.T) {
	units := map[string]string{
		LlamaCppServicePath:          "ExecStart=/usr/local/bin/llama-server -m " + LlamaCppModelPath + " --host 127.0.0.1",
		LlamaCppEmbeddingServicePath: "ExecStart=/usr/local/bin/llama-server -m /root/.internkim/models/embeddinggemma-300M-qat-Q4_0.gguf --host 127.0.0.1",
	}

	drift := SetupDrift(func(path string) ([]byte, error) { return []byte(units[path]), nil })

	if len(drift) != 1 || !strings.Contains(drift[0], LlamaCppEmbeddingServiceName) || !strings.Contains(drift[0], LlamaCppEmbeddingModelPath) {
		t.Fatalf("expected only the embedding service to drift, got %v", drift)
	}
}

func TestSetupDriftIgnoresServicesThisDeviceDoesNotRun(t *testing.T) {
	drift := SetupDrift(func(string) ([]byte, error) { return nil, errors.New("no such file") })

	if len(drift) != 0 {
		t.Fatalf("expected no drift without the units, got %v", drift)
	}
}
