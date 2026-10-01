package locallm

import "time"

const SubprocessTimeout = 10 * time.Minute

type Backend string

const (
	BackendLlamaCpp Backend = "llama-cpp"
)

const Default = BackendLlamaCpp

const (
	LiteRTModelPath     = "/root/.internkim/models/gemma-4-E4B-it.litertlm"
	LiteRTModelFilename = "gemma-4-E4B-it.litertlm"

	LlamaCppModelPath        = "/root/.internkim/models/gemma-4-E2B-it-qat-UD-Q4_K_XL.gguf"
	LlamaCppModelFilename    = "gemma-4-E2B-it-qat-UD-Q4_K_XL.gguf"
	LlamaCppServiceName      = "internkim-llamacpp.service"
	LlamaCppBaseURL          = "http://127.0.0.1:18081"
	LlamaCppEmbeddingBaseURL = "http://127.0.0.1:18082"
)

func ModelPath() string {
	if Default == BackendLlamaCpp {
		return LlamaCppModelPath
	}
	return LiteRTModelPath
}
