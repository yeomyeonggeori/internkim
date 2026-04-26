package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/anthropic-lab/internkim/internal/capabilityd"
)

func main() {
	defaultConfiguration := capabilityd.DefaultConfiguration()
	configuration := capabilityd.Configuration{}
	flag.StringVar(&configuration.SocketPath, "socket", defaultConfiguration.SocketPath, "capability socket path")
	flag.StringVar(&configuration.OpenRouterKeyPath, "openrouter-key", defaultConfiguration.OpenRouterKeyPath, "OpenRouter key path")
	flag.StringVar(&configuration.MattermostBaseURL, "mattermost-url", defaultConfiguration.MattermostBaseURL, "Mattermost base URL")
	flag.StringVar(&configuration.MattermostTokenPath, "mattermost-token", defaultConfiguration.MattermostTokenPath, "Mattermost bot token path")
	flag.StringVar(&configuration.SlackTokenPath, "slack-token", defaultConfiguration.SlackTokenPath, "Slack bot token path")
	flag.StringVar(&configuration.SlackAppTokenPath, "slack-app-token", defaultConfiguration.SlackAppTokenPath, "Slack app token path")
	flag.StringVar(&configuration.SignalJSONRPCURL, "signal-jsonrpc-url", defaultConfiguration.SignalJSONRPCURL, "Signal JSON-RPC URL")
	flag.StringVar(&configuration.SignalAccount, "signal-account", defaultConfiguration.SignalAccount, "Signal account")
	flag.StringVar(&configuration.BlueclawBaseURL, "blueclaw-url", defaultConfiguration.BlueclawBaseURL, "Blueclaw base URL")
	flag.StringVar(&configuration.LiteRTModelPath, "litert-model", defaultConfiguration.LiteRTModelPath, "LiteRT-LM model path")
	flag.StringVar(&configuration.LiteRTWrapperPath, "litert-wrapper", defaultConfiguration.LiteRTWrapperPath, "LiteRT-LM wrapper path")
	flag.Parse()

	if errorValue := capabilityd.Run(configuration); errorValue != nil {
		fmt.Fprintln(os.Stderr, errorValue.Error())
		os.Exit(1)
	}
}
