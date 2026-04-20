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
			return sshFileExists(context, "/root/.internkim/secrets/openrouter-api-key")
		case BackendSD:
			return stagedFileExists(context, "secrets/openrouter-api-key")
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
printf 'OPENROUTER_API_KEY=%%s' %s > /root/.internkim/secrets/openrouter-api-key
chown zeroclaw /root/.internkim/secrets/openrouter-api-key
chmod 640 /root/.internkim/secrets/openrouter-api-key`,
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
			[]byte("OPENROUTER_API_KEY="+apiKey), 0o644); err != nil {
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
	return apiKey, nil
}
