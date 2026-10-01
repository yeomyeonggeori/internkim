package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/capabilityd"
	"github.com/yeomyeonggeori/internkim/internal/modelladder"
)

func main() {
	defaultConfiguration := capabilityd.DefaultConfiguration()
	configuration := capabilityd.Configuration{}
	flag.StringVar(&configuration.SocketPath, "socket", defaultConfiguration.SocketPath, "capability socket path")
	flag.IntVar(&configuration.VSockPort, "vsock-port", defaultConfiguration.VSockPort, "optional host vsock port for the agent guest")
	flag.StringVar(&configuration.OpenRouterKeyPath, "openrouter-key", defaultConfiguration.OpenRouterKeyPath, "OpenRouter key path")
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
	flag.StringVar(&configuration.BlueclawWorkspacePath, "blueclaw-workspace", defaultConfiguration.BlueclawWorkspacePath, "Blueclaw host workspace path")
	flag.StringVar(&configuration.AdmindBaseURL, "admind-url", defaultConfiguration.AdmindBaseURL, "admind base URL, which holds the seed a message is signed with")
	flag.StringVar(&configuration.AdmindSocketPath, "admind-socket", defaultConfiguration.AdmindSocketPath, "admind requester socket, the only door that honours an asserted requester")
	flag.StringVar(&configuration.ChatdEndpoint, "chatd-endpoint", defaultConfiguration.ChatdEndpoint, "chatd base URL for platform message delivery")
	flag.StringVar(&configuration.ChatdPlatform, "chatd-platform", defaultConfiguration.ChatdPlatform, "platform name chatd serves for message delivery")
	flag.StringVar(&configuration.FileReadPythonPath, "file-read-python", defaultConfiguration.FileReadPythonPath, "Python executable for file_read conversions")
	flag.StringVar(&configuration.AgentBrowserPath, "agent-browser", defaultConfiguration.AgentBrowserPath, "agent-browser executable path")
	flag.StringVar(&configuration.DeviceBrowserExecutablePath, "device-browser", defaultConfiguration.DeviceBrowserExecutablePath, "moli executable each requester's device browser runs")
	flag.StringVar(&configuration.DeviceBrowserStateDirectory, "device-browser-state-dir", defaultConfiguration.DeviceBrowserStateDirectory, "directory holding each requester's device browser profile")
	flag.IntVar(&configuration.DeviceBrowserFirstPort, "device-browser-first-port", defaultConfiguration.DeviceBrowserFirstPort, "first loopback port a device browser answers the Chrome DevTools Protocol on")
	flag.IntVar(&configuration.DeviceBrowserCapacity, "device-browser-capacity", defaultConfiguration.DeviceBrowserCapacity, "most device browsers running at once")
	flag.StringVar(&configuration.DeviceBrowserUserName, "device-browser-user", defaultConfiguration.DeviceBrowserUserName, "user a device browser runs as when capabilityd runs as root")
	flag.StringVar(&configuration.LocalInferenceMode, "local-inference-mode", defaultConfiguration.LocalInferenceMode, "local inference mode: device or remote")
	flag.BoolVar(&configuration.LocalOnly, "local-only", defaultConfiguration.LocalOnly, "disable remote LLM fallback")
	flag.String("companion-url", "", "ignored; device units written before 2026-10-01 still pass it, and an undefined flag stops the daemon")
	flag.Bool("prefer-companion-llm", false, "ignored; device units written in May 2026 still pass it, and an undefined flag stops the daemon")
	shouldPrintCapabilities := flag.Bool("print-capabilities", false, "print the capabilities block the agent's runtime document needs, and exit")
	modelLadderKeyPath := flag.String("print-model-ladder", "", "print the languageModel block the agent's runtime document needs, with this path named as each endpoint's key file, and exit")
	modelLadderEndpoint := flag.String("model-endpoint", "", "the OpenAI-compatible endpoint the printed ladder reaches, when this company runs its models somewhere of its own")
	flag.Parse()
	if *shouldPrintCapabilities {
		printCapabilities(configuration.SocketPath)
		return
	}
	if strings.TrimSpace(*modelLadderKeyPath) != "" {
		printModelLadder(*modelLadderEndpoint, *modelLadderKeyPath)
		return
	}
	if strings.TrimSpace(configuration.ChatdPlatform) == "" {
		fmt.Fprintln(os.Stderr, "--chatd-platform names the messenger this company runs, and every message tool reaches it through chatd; capabilityd delivers nothing without it")
		os.Exit(1)
	}

	if errorValue := capabilityd.Run(configuration); errorValue != nil {
		fmt.Fprintln(os.Stderr, errorValue.Error())
		os.Exit(1)
	}
}

func printModelLadder(endpointURL string, apiKeyPath string) {
	printJSONDocument(modelladder.LanguageModelDocument(endpointURL, apiKeyPath))
}

func printCapabilities(socketPath string) {
	printJSONDocument(map[string]any{
		"transport":      "unix",
		"unixSocketPath": socketPath,
		"endpoint":       "http://internkim-capability",
		"timeoutSecond":  30,
	})
}

func printJSONDocument(document any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if errorValue := encoder.Encode(document); errorValue != nil {
		fmt.Fprintln(os.Stderr, errorValue.Error())
		os.Exit(1)
	}
}
