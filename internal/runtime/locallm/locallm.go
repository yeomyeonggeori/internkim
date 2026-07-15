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
	LlamaCppVersion                = "b9660"
	LlamaCppModelPath              = "/root/.internkim/models/gemma-4-E2B-it-qat-UD-Q4_K_XL.gguf"
	LlamaCppModelFilename          = "gemma-4-E2B-it-qat-UD-Q4_K_XL.gguf"
	LlamaCppModelURL               = "https://huggingface.co/unsloth/gemma-4-E2B-it-qat-GGUF/resolve/main/gemma-4-E2B-it-qat-UD-Q4_K_XL.gguf"
	LlamaCppDraftModelPath         = "/root/.internkim/models/mtp-gemma-4-E2B-it.gguf"
	LlamaCppDraftModelFilename     = "mtp-gemma-4-E2B-it.gguf"
	LlamaCppDraftModelURL          = "https://huggingface.co/unsloth/gemma-4-E2B-it-qat-GGUF/resolve/main/mtp-gemma-4-E2B-it.gguf"
	LlamaCppEmbeddingModelPath     = "/root/.internkim/models/bge-m3-Q8_0.gguf"
	LlamaCppEmbeddingModelFilename = "bge-m3-Q8_0.gguf"
	LlamaCppEmbeddingModelURL      = "https://huggingface.co/gpustack/bge-m3-GGUF/resolve/2d48f1737679ad900d5c26c5aad5410e9c70fdca/bge-m3-Q8_0.gguf"
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
