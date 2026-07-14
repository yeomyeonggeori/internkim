package main

import (
	"flag"
	"fmt"
	"os"

	"gitlab.com/eastriver/internkim/internal/capabilityd"
)

func main() {
	defaultConfiguration := capabilityd.DefaultConfiguration()
	configuration := capabilityd.Configuration{}
	flag.StringVar(&configuration.SocketPath, "socket", defaultConfiguration.SocketPath, "capability socket path")
	flag.IntVar(&configuration.VSockPort, "vsock-port", defaultConfiguration.VSockPort, "optional host vsock port for Firecracker guests")
	flag.StringVar(&configuration.OpenRouterKeyPath, "openrouter-key", defaultConfiguration.OpenRouterKeyPath, "OpenRouter key path")
	flag.StringVar(&configuration.MattermostBaseURL, "mattermost-url", defaultConfiguration.MattermostBaseURL, "Mattermost base URL")
	flag.StringVar(&configuration.MattermostTokenPath, "mattermost-token", defaultConfiguration.MattermostTokenPath, "Mattermost bot token path")
	flag.StringVar(&configuration.MattermostInteractiveTokenPath, "mattermost-interactive-token", defaultConfiguration.MattermostInteractiveTokenPath, "Mattermost interactive action token path")
	flag.StringVar(&configuration.MattermostInteractiveBaseURL, "mattermost-interactive-base-url", defaultConfiguration.MattermostInteractiveBaseURL, "public base URL for Mattermost interactive actions")
	flag.StringVar(&configuration.SlackTokenPath, "slack-token", defaultConfiguration.SlackTokenPath, "Slack bot token path")
	flag.StringVar(&configuration.SlackAppTokenPath, "slack-app-token", defaultConfiguration.SlackAppTokenPath, "Slack app token path")
	flag.StringVar(&configuration.SignalJSONRPCURL, "signal-jsonrpc-url", defaultConfiguration.SignalJSONRPCURL, "Signal JSON-RPC URL")
	flag.StringVar(&configuration.SignalAccount, "signal-account", defaultConfiguration.SignalAccount, "Signal account")
	flag.StringVar(&configuration.BlueclawBaseURL, "blueclaw-url", defaultConfiguration.BlueclawBaseURL, "Blueclaw base URL")
	flag.StringVar(&configuration.LiteRTModelPath, "litert-model", defaultConfiguration.LiteRTModelPath, "LiteRT-LM model path")
	flag.StringVar(&configuration.LocalLLMRunnerPath, "local-llm-runner", defaultConfiguration.LocalLLMRunnerPath, "local LLM runner path")
	flag.StringVar(&configuration.OpenRouterBaseURL, "openrouter-url", defaultConfiguration.OpenRouterBaseURL, "OpenRouter-compatible chat completion URL")
	flag.StringVar(&configuration.OpenRouterModel, "openrouter-model", defaultConfiguration.OpenRouterModel, "OpenRouter chat model")
	flag.BoolVar(&configuration.ForceOpenRouterModel, "force-openrouter-model", defaultConfiguration.ForceOpenRouterModel, "force OpenRouter requests to use --openrouter-model")
	flag.StringVar(&configuration.OpenRouterGatewaySecretPath, "openrouter-gateway-secret", defaultConfiguration.OpenRouterGatewaySecretPath, "OpenRouter gateway shared secret path")
	flag.StringVar(&configuration.OpenRouterGatewaySecretHeader, "openrouter-gateway-secret-header", defaultConfiguration.OpenRouterGatewaySecretHeader, "OpenRouter gateway shared secret header")
	flag.StringVar(&configuration.OpenRouterEmbeddingBaseURL, "openrouter-embedding-url", defaultConfiguration.OpenRouterEmbeddingBaseURL, "OpenRouter embedding URL")
	flag.StringVar(&configuration.OpenRouterEmbeddingModel, "openrouter-embedding-model", defaultConfiguration.OpenRouterEmbeddingModel, "OpenRouter embedding model")
	flag.StringVar(&configuration.OpenRouterWebBaseURL, "openrouter-web-url", defaultConfiguration.OpenRouterWebBaseURL, "OpenRouter web tool chat completion URL")
	flag.StringVar(&configuration.OllamaBaseURL, "ollama-url", defaultConfiguration.OllamaBaseURL, "Ollama base URL")
	flag.StringVar(&configuration.OllamaModel, "ollama-model", defaultConfiguration.OllamaModel, "Ollama local model")
	flag.StringVar(&configuration.LlamaCppBaseURL, "llamacpp-url", defaultConfiguration.LlamaCppBaseURL, "llama.cpp server base URL")
	flag.StringVar(&configuration.LlamaCppModel, "llamacpp-model", defaultConfiguration.LlamaCppModel, "llama.cpp model")
	flag.StringVar(&configuration.LlamaCppEmbeddingBaseURL, "llamacpp-embedding-url", defaultConfiguration.LlamaCppEmbeddingBaseURL, "llama.cpp embedding server base URL")
	flag.StringVar(&configuration.LlamaCppEmbeddingModel, "llamacpp-embedding-model", defaultConfiguration.LlamaCppEmbeddingModel, "llama.cpp embedding model")
	flag.StringVar(&configuration.CompanionBaseURL, "companion-url", defaultConfiguration.CompanionBaseURL, "companion capability base URL")
	flag.StringVar(&configuration.FleetIDPath, "fleet-id-path", defaultConfiguration.FleetIDPath, "fleet id path for public companion pairing links")
	flag.StringVar(&configuration.BlueclawWorkspacePath, "blueclaw-workspace", defaultConfiguration.BlueclawWorkspacePath, "Blueclaw host workspace path")
	flag.StringVar(&configuration.FileReadPythonPath, "file-read-python", defaultConfiguration.FileReadPythonPath, "Python executable for file.read conversions")
	flag.StringVar(&configuration.AgentBrowserPath, "agent-browser", defaultConfiguration.AgentBrowserPath, "agent-browser executable path")
	flag.StringVar(&configuration.DeviceBrowserPath, "device-browser", defaultConfiguration.DeviceBrowserPath, "device Chromium executable path")
	flag.BoolVar(&configuration.PreferCompanionLLM, "prefer-companion-llm", defaultConfiguration.PreferCompanionLLM, "prefer companion local LLM when available")
	flag.StringVar(&configuration.LocalInferenceMode, "local-inference-mode", defaultConfiguration.LocalInferenceMode, "local inference mode: device, companion_preferred, companion_only, remote")
	flag.BoolVar(&configuration.LocalOnly, "local-only", defaultConfiguration.LocalOnly, "disable remote LLM fallback")
	flag.Parse()
	if environmentDeviceBrowserPath := os.Getenv("INTERNKIM_DEVICE_BROWSER_PATH"); environmentDeviceBrowserPath != "" {
		configuration.DeviceBrowserPath = environmentDeviceBrowserPath
	}

	if errorValue := capabilityd.Run(configuration); errorValue != nil {
		fmt.Fprintln(os.Stderr, errorValue.Error())
		os.Exit(1)
	}
}
