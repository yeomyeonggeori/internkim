package setup

import (
	"errors"
	"fmt"
	"strings"
)

var StepOpenRouter = Step{
	Name: "openrouter",
	Deps: []string{"binaries"},
	Title: func(context *Context) string {
		return context.T("OpenRouter API 키 설정...", "Configuring OpenRouter API key...")
	},
	IsSatisfied: func(context *Context) bool {
		switch context.Backend {
		case BackendSSH:
			return sshOpenRouterKeyIsUsable(context)
		case BackendSD:
			return stagedOpenRouterKeyIsUsable(context)
		}
		return false
	},
	Run: func(context *Context) error {
		apiKey, err := resolveOpenRouterKey(context)
		if err != nil {
			return err
		}
		context.SSH.Run(fmt.Sprintf(
			`mkdir -p /root/.internkim/secrets
chown root:root /root/.internkim/secrets
chmod 700 /root/.internkim/secrets
printf '%%s' %s > /root/.internkim/secrets/openrouter-api-key
chown root:root /root/.internkim/secrets/openrouter-api-key
chmod 600 /root/.internkim/secrets/openrouter-api-key`,
			shellQuote(apiKey),
		))
		fmt.Println("  " + context.T("API 키 설치 완료", "API key installed"))
		return nil
	},
	RunSD: func(context *Context) error {
		apiKey, err := resolveOpenRouterKey(context)
		if err != nil {
			return err
		}
		if err := context.SD.WriteFile("secrets/openrouter-api-key",
			[]byte(apiKey), 0o644); err != nil {
			return err
		}
		fmt.Println("  " + context.T("API 키 스테이지 완료", "API key staged"))
		return nil
	},
}

func resolveOpenRouterKey(context *Context) (string, error) {
	if context.Callbacks.GetOpenRouterKey == nil {
		return "", errors.New("openrouter key callback missing")
	}
	apiKey, err := context.Callbacks.GetOpenRouterKey(context.Force)
	if err != nil {
		return "", err
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return "", errors.New("openrouter API key is empty")
	}
	if isPlaceholderOpenRouterKey(apiKey) {
		return "", errors.New("openrouter API key is a simulation placeholder; pass --openrouter-api-key or set OPENROUTER_API_KEY")
	}
	return apiKey, nil
}

func sshOpenRouterKeyIsUsable(context *Context) bool {
	value := trimmedRun(context, "cat /root/.internkim/secrets/openrouter-api-key 2>/dev/null || true")
	return openRouterKeyIsUsable(value)
}

func stagedOpenRouterKeyIsUsable(context *Context) bool {
	document, errorValue := readStagedFile(context, "secrets/openrouter-api-key")
	if errorValue != nil {
		return false
	}
	return openRouterKeyIsUsable(string(document))
}

func openRouterKeyIsUsable(value string) bool {
	apiKey := strings.TrimSpace(strings.TrimPrefix(value, "OPENROUTER_API_KEY="))
	return apiKey != "" && !isPlaceholderOpenRouterKey(apiKey)
}

func isPlaceholderOpenRouterKey(value string) bool {
	normalizedValue := strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(normalizedValue, "internkim-simulation-openrouter-api-key") ||
		strings.Contains(normalizedValue, "simulation-openrouter")
}
