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
	flag.StringVar(&configuration.SlackTokenPath, "slack-token", defaultConfiguration.SlackTokenPath, "Slack bot token path")
	flag.StringVar(&configuration.SlackAppTokenPath, "slack-app-token", defaultConfiguration.SlackAppTokenPath, "Slack app token path")
	flag.StringVar(&configuration.SignalJSONRPCURL, "signal-jsonrpc-url", defaultConfiguration.SignalJSONRPCURL, "Signal JSON-RPC URL")
	flag.StringVar(&configuration.SignalAccount, "signal-account", defaultConfiguration.SignalAccount, "Signal account")
	flag.StringVar(&configuration.BlueclawBaseURL, "blueclaw-url", defaultConfiguration.BlueclawBaseURL, "Blueclaw base URL")
	flag.StringVar(&configuration.LiteRTModelPath, "litert-model", defaultConfiguration.LiteRTModelPath, "LiteRT-LM model path")
	flag.StringVar(&configuration.LocalLLMRunnerPath, "local-llm-runner", defaultConfiguration.LocalLLMRunnerPath, "local LLM runner path")
	flag.StringVar(&configuration.OpenRouterEmbeddingBaseURL, "openrouter-embedding-url", defaultConfiguration.OpenRouterEmbeddingBaseURL, "OpenRouter embedding URL")
	flag.StringVar(&configuration.OpenRouterEmbeddingModel, "openrouter-embedding-model", defaultConfiguration.OpenRouterEmbeddingModel, "OpenRouter embedding model")
	flag.StringVar(&configuration.OllamaBaseURL, "ollama-url", defaultConfiguration.OllamaBaseURL, "Ollama base URL")
	flag.StringVar(&configuration.OllamaModel, "ollama-model", defaultConfiguration.OllamaModel, "Ollama local model")
	flag.StringVar(&configuration.LlamaCppBaseURL, "llamacpp-url", defaultConfiguration.LlamaCppBaseURL, "llama.cpp server base URL")
	flag.StringVar(&configuration.LlamaCppModel, "llamacpp-model", defaultConfiguration.LlamaCppModel, "llama.cpp model")
	flag.StringVar(&configuration.CompanionBaseURL, "companion-url", defaultConfiguration.CompanionBaseURL, "companion capability base URL")
	flag.StringVar(&configuration.AgentBrowserPath, "agent-browser", defaultConfiguration.AgentBrowserPath, "agent-browser executable path")
	flag.StringVar(&configuration.DeviceBrowserPath, "device-browser", defaultConfiguration.DeviceBrowserPath, "device Chromium executable path")
	flag.BoolVar(&configuration.PreferCompanionLLM, "prefer-companion-llm", defaultConfiguration.PreferCompanionLLM, "prefer companion local LLM when available")
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
