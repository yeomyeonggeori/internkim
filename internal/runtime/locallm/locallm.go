package locallm

import "time"

const SubprocessTimeout = 10 * time.Minute

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

	LlamaCppBinaryPath             = "/usr/local/bin/llama-server"
	LlamaCppLibraryDir             = "/usr/local/lib/llama-cpp"
	LlamaCppVersion                = "b8995"
	LlamaCppModelPath              = "/root/.internkim/models/gemma-4-E4B-it-Q4_0.gguf"
	LlamaCppModelFilename          = "gemma-4-E4B-it-Q4_0.gguf"
	LlamaCppModelURL               = "https://huggingface.co/unsloth/gemma-4-E4B-it-GGUF/resolve/main/gemma-4-E4B-it-Q4_0.gguf"
	LlamaCppEmbeddingModelPath     = "/root/.internkim/models/embeddinggemma-300M-Q8_0.gguf"
	LlamaCppEmbeddingModelFilename = "embeddinggemma-300M-Q8_0.gguf"
	LlamaCppEmbeddingModelURL      = "https://huggingface.co/ggml-org/embeddinggemma-300M-GGUF/resolve/main/embeddinggemma-300M-Q8_0.gguf"
	LlamaCppCacheRelative          = ".dependency/llama-cpp"
	LlamaCppCacheKey               = LlamaCppVersion + "-aarch64"
	LlamaCppCacheModelDir          = ".dependency/llama-cpp-models"
	LlamaCppBuildTool              = "build-llama-cpp-jetson"
	LlamaCppDisplayName            = "llama-server"
	LlamaCppServiceName            = "internkim-llamacpp.service"
	LlamaCppServicePath            = "/etc/systemd/system/internkim-llamacpp.service"
	LlamaCppEmbeddingServiceName   = "internkim-llamacpp-embedding.service"
	LlamaCppEmbeddingServicePath   = "/etc/systemd/system/internkim-llamacpp-embedding.service"
	LlamaCppBaseURL                = "http://127.0.0.1:18081"
	LlamaCppEmbeddingBaseURL       = "http://127.0.0.1:18082"
	LlamaCppHost                   = "127.0.0.1"
	LlamaCppPort                   = "18081"
	LlamaCppEmbeddingPort          = "18082"
	LlamaCppEmbeddingBatchSize     = "2048"
	LlamaCppEmbeddingUBatchSize    = "2048"
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
