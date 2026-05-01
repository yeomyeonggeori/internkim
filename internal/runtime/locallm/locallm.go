package locallm

type Backend string

const (
	BackendLiteRT   Backend = "litert"
	BackendLlamaCpp Backend = "llama-cpp"
)

const Default = BackendLiteRT

type Spec struct {
	Name                       string
	RemoteBinaryPath           string
	RemoteLibraryDirectory     string
	RemoteModelPath            string
	ModelFilename              string
	ModelSourceURL             string
	ModelRepository            string
	LocalCacheRelativeRoot     string
	LocalCacheKey              string
	LocalModelCacheRelativeDir string
}

func litertSpec() Spec {
	return Spec{
		Name:                       string(BackendLiteRT),
		RemoteBinaryPath:           "/usr/local/bin/litert_lm_main",
		RemoteLibraryDirectory:     "/usr/local/lib/litert_lm",
		RemoteModelPath:            "/root/.internkim/models/gemma-4-E4B-it.litertlm",
		ModelFilename:              "gemma-4-E4B-it.litertlm",
		ModelSourceURL:             "https://huggingface.co/litert-community/gemma-4-E4B-it-litert-lm/resolve/main/gemma-4-E4B-it.litertlm",
		ModelRepository:            "litert-community/gemma-4-E4B-it-litert-lm",
		LocalCacheRelativeRoot:     ".dependency/litert-lm-main",
		LocalCacheKey:              "v0.10.2-aarch64",
		LocalModelCacheRelativeDir: ".dependency/litert-models",
	}
}

func llamaCppSpec() Spec {
	return Spec{
		Name:                       string(BackendLlamaCpp),
		RemoteBinaryPath:           "/usr/local/bin/llama-cli",
		RemoteLibraryDirectory:     "/usr/local/lib/llama-cpp",
		RemoteModelPath:            "/root/.internkim/models/gemma-4-E4B-it-Q4_0.gguf",
		ModelFilename:              "gemma-4-E4B-it-Q4_0.gguf",
		ModelSourceURL:             "https://huggingface.co/unsloth/gemma-4-E4B-it-GGUF/resolve/main/gemma-4-E4B-it-Q4_0.gguf",
		ModelRepository:            "unsloth/gemma-4-E4B-it-GGUF",
		LocalCacheRelativeRoot:     ".dependency/llama-cpp",
		LocalCacheKey:              "05e141a-aarch64",
		LocalModelCacheRelativeDir: ".dependency/llama-cpp-models",
	}
}

func SpecFor(backend Backend) Spec {
	switch backend {
	case BackendLlamaCpp:
		return llamaCppSpec()
	}
	return litertSpec()
}

func DefaultSpec() Spec {
	return SpecFor(Default)
}
