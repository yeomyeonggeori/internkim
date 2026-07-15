package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"gitlab.com/eastriver/internkim/internal/llmgateway"
)

func main() {
	configuration := llmgateway.Configuration{}
	listenAddress := flag.String("listen", "127.0.0.1:18181", "LLM gateway listen address")
	flag.StringVar(&configuration.ProviderChatCompletionsURL, "provider-chat-url", "https://openrouter.ai/api/v1/chat/completions", "provider chat completions URL")
	flag.StringVar(&configuration.ProviderEmbeddingsURL, "provider-embeddings-url", "", "provider embeddings URL")
	flag.StringVar(&configuration.ProviderAPIKeyPath, "provider-key", "/root/.internkim/secrets/openrouter-provider-api-key", "provider API key path")
	flag.StringVar(&configuration.DeviceTokensPath, "device-tokens", "/root/.internkim/config/llm-gateway-device-tokens.json", "device token configuration path")
	flag.StringVar(&configuration.LedgerPath, "ledger", "/root/.internkim/state/llm-gateway-usage.jsonl", "usage ledger path")
	flag.Int64Var(&configuration.MinimumRequestMicrounits, "minimum-request-microunits", 1, "minimum quota charge for preflight checks")
	flag.Int64Var(&configuration.PromptMicrounitsPerMillion, "prompt-microunits-per-million", 0, "prompt microunits per million tokens")
	flag.Int64Var(&configuration.OutputMicrounitsPerMillion, "output-microunits-per-million", 0, "output microunits per million tokens")
	flag.Parse()

	server := &http.Server{
		Addr: *listenAddress,
		Handler: llmgateway.Service{
			Configuration: configuration,
			RateLimiter:   llmgateway.NewMemoryRateLimiter(),
			HTTPClient:    &http.Client{},
		}.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	if errorValue := server.ListenAndServe(); errorValue != nil && errorValue != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, errorValue.Error())
		os.Exit(1)
	}
}
