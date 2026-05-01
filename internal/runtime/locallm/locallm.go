package locallm

type Backend string

const (
	BackendLiteRT   Backend = "litert"
	BackendLlamaCpp Backend = "llama-cpp"
)

const Default = BackendLlamaCpp

const (
	LiteRTBinaryPath    = "/usr/local/bin/litert_lm_main"
	LiteRTLibraryDir    = "/usr/local/lib/litert_lm"
	LiteRTModelPath     = "/root/.internkim/models/gemma-4-E4B-it.litertlm"
	LiteRTModelFilename = "gemma-4-E4B-it.litertlm"
	LiteRTModelURL      = "https://huggingface.co/litert-community/gemma-4-E4B-it-litert-lm/resolve/main/gemma-4-E4B-it.litertlm"
	LiteRTCacheRelative = ".dependency/litert-lm-main"
	LiteRTCacheKey      = "v0.10.2-aarch64"
	LiteRTCacheModelDir = ".dependency/litert-models"
	LiteRTBuildTool     = "build-litert-lm-main"
	LiteRTDisplayName   = "litert_lm_main"

	LlamaCppBinaryPath    = "/usr/local/bin/llama-cli"
	LlamaCppLibraryDir    = "/usr/local/lib/llama-cpp"
	LlamaCppModelPath     = "/root/.internkim/models/gemma-4-E4B-it-Q4_0.gguf"
	LlamaCppModelFilename = "gemma-4-E4B-it-Q4_0.gguf"
	LlamaCppModelURL      = "https://huggingface.co/unsloth/gemma-4-E4B-it-GGUF/resolve/main/gemma-4-E4B-it-Q4_0.gguf"
	LlamaCppCacheRelative = ".dependency/llama-cpp"
	LlamaCppCacheKey      = "05e141a-aarch64"
	LlamaCppCacheModelDir = ".dependency/llama-cpp-models"
	LlamaCppBuildTool     = "build-llama-cpp-jetson"
	LlamaCppDisplayName   = "llama-cli"
)

func BinaryPath() string {
	if Default == BackendLlamaCpp {
		return LlamaCppBinaryPath
	}
	return LiteRTBinaryPath
}

func LibraryDir() string {
	if Default == BackendLlamaCpp {
		return LlamaCppLibraryDir
	}
	return LiteRTLibraryDir
}

func ModelPath() string {
	if Default == BackendLlamaCpp {
		return LlamaCppModelPath
	}
	return LiteRTModelPath
}

func ModelFilename() string {
	if Default == BackendLlamaCpp {
		return LlamaCppModelFilename
	}
	return LiteRTModelFilename
}

func ModelURL() string {
	if Default == BackendLlamaCpp {
		return LlamaCppModelURL
	}
	return LiteRTModelURL
}
