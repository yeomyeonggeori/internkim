package setup

import (
	"errors"
	"fmt"
	"strings"
)

const buzzKeySeedDevicePath = "/root/.internkim/secrets/buzz-key-seed"
const buzzKeySeedStagedPath = "secrets/buzz-key-seed"

// StepBuzzSeed persists the Buzz identity derivation seed into the service
// secrets directory. The seed is the one value that ties every imported
// identity and channel together; losing or changing it orphans all imported
// history. So this step NEVER overwrites a seed that already exists on the
// device — an existing seed always wins, and a reprovision preserves it.
var StepBuzzSeed = Step{
	Name: "buzz-seed",
	Title: func(context *Context) string {
		return context.T("Buzz 신원 seed 설정...", "Configuring Buzz identity seed...")
	},
	IsSatisfied: func(context *Context) bool {
		switch context.Backend {
		case BackendSSH:
			return buzzKeySeedIsPresent(trimmedRun(context, "cat "+buzzKeySeedDevicePath+" 2>/dev/null || true"))
		case BackendSD:
			document, errorValue := readStagedFile(context, buzzKeySeedStagedPath)
			return errorValue == nil && buzzKeySeedIsPresent(string(document))
		}
		return false
	},
	Run: func(context *Context) error {
		seed, errorValue := resolveBuzzKeySeed(context)
		if errorValue != nil {
			return errorValue
		}
		context.SSH.Run(fmt.Sprintf(
			`mkdir -p /root/.internkim/secrets
chown root:root /root/.internkim/secrets
chmod 700 /root/.internkim/secrets
printf '%%s' %s > %s
chown root:root %s
chmod 600 %s`,
			shellQuote(seed), buzzKeySeedDevicePath, buzzKeySeedDevicePath, buzzKeySeedDevicePath,
		))
		fmt.Println("  " + context.T("Buzz seed 설치 완료", "Buzz seed installed"))
		return nil
	},
	RunSD: func(context *Context) error {
		seed, errorValue := resolveBuzzKeySeed(context)
		if errorValue != nil {
			return errorValue
		}
		if errorValue := context.SD.WriteFile(buzzKeySeedStagedPath, []byte(seed), 0o600); errorValue != nil {
			return errorValue
		}
		fmt.Println("  " + context.T("Buzz seed 스테이지 완료", "Buzz seed staged"))
		return nil
	},
}

func resolveBuzzKeySeed(context *Context) (string, error) {
	if context.Callbacks.GetBuzzKeySeed == nil {
		return "", errors.New("buzz key seed callback missing")
	}
	seed, errorValue := context.Callbacks.GetBuzzKeySeed(context.Force)
	if errorValue != nil {
		return "", errorValue
	}
	seed = strings.TrimSpace(seed)
	if !buzzKeySeedIsPresent(seed) {
		return "", errors.New("buzz key seed is empty; generate .local/secrets/buzz-key-seed (openssl rand -hex 32) or set INTERNKIM_BUZZ_KEY_SEED")
	}
	return seed, nil
}

func buzzKeySeedIsPresent(value string) bool {
	return strings.TrimSpace(value) != ""
}
