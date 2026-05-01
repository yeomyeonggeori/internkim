package setup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/anthropic-lab/internkim/internal/runtime/blueclaw"
	"github.com/anthropic-lab/internkim/internal/runtime/locallm"
)

var StepLiteRT = Step{
	Name: "litert",
	Deps: []string{"binaries"},
	Title: func(context *Context) string {
		switch locallm.Default {
		case locallm.BackendLlamaCpp:
			return context.T("llama.cpp 로컬 모델 준비...", "Preparing llama.cpp local model...")
		default:
			return context.T("LiteRT 로컬 모델 준비...", "Preparing LiteRT local model...")
		}
	},
	IsSatisfied: func(context *Context) bool {
		return localLLMIsSatisfied(context)
	},
	Run: func(context *Context) error {
		if context.SSH == nil {
			return errors.New("local-llm SSH callback missing")
		}
		switch locallm.Default {
		case locallm.BackendLlamaCpp:
			return runLlamaCppStep(context)
		default:
			return runLiteRTStep(context)
		}
	},
	RunSD: func(context *Context) error {
		return stageLocalLLMModelSD(context)
	},
}

func localLLMIsSatisfied(context *Context) bool {
	spec := locallm.DefaultSpec()
	switch context.Backend {
	case BackendSSH:
		switch locallm.Default {
		case locallm.BackendLlamaCpp:
			return trimmedRun(context, fmt.Sprintf(
				`test -x /usr/local/bin/internkim-litert-wrapper && test -x %s && test -s %s && ! su -s /bin/sh blueclaw -c 'test -r %s' 2>/dev/null && echo y || echo n`,
				shellQuote(spec.RemoteBinaryPath),
				shellQuote(spec.RemoteModelPath),
				shellQuote(spec.RemoteModelPath),
			)) == "y"
		default:
			return trimmedRun(context, fmt.Sprintf(
				`test -x /usr/local/bin/internkim-litert-wrapper && command -v litert-lm >/dev/null 2>&1 && test -s %s && ! su -s /bin/sh blueclaw -c 'test -r %s' 2>/dev/null && echo y || echo n`,
				shellQuote(spec.RemoteModelPath),
				shellQuote(spec.RemoteModelPath),
			)) == "y"
		}
	case BackendSD:
		return stagedFileExists(context, filepath.Join("models", spec.ModelFilename))
	}
	return false
}

func runLiteRTStep(context *Context) error {
	modelPath, errorValue := resolveLocalLLMModelPath(context)
	if errorValue != nil {
		return errorValue
	}
	if modelPath != "" {
		if errorValue := copyLocalLLMModelSSH(context, modelPath); errorValue != nil {
			return errorValue
		}
	}

	context.SSH.Run(fmt.Sprintf(`set -e
install -d -o root -g root -m 700 /root/.internkim/models
if ! command -v uv >/dev/null 2>&1; then
  curl -LsSf https://astral.sh/uv/install.sh -o /tmp/internkim-uv-install.sh
  sh /tmp/internkim-uv-install.sh
  ln -sf /root/.local/bin/uv /usr/local/bin/uv
fi
uv tool install --upgrade litert-lm >/dev/null
ln -sf /root/.local/bin/litert-lm /usr/local/bin/litert-lm
if [ ! -s %s ]; then
  curl -L --fail --retry 3 --output %s.tmp %s
  mv %s.tmp %s
fi
chown root:root /root/.internkim/models %s
chmod 700 /root/.internkim/models
chmod 600 %s`,
		shellQuote(blueclaw.LiteRTModelPath),
		shellQuote(blueclaw.LiteRTModelPath),
		shellQuote(blueclaw.LiteRTModelSourceURL),
		shellQuote(blueclaw.LiteRTModelPath),
		shellQuote(blueclaw.LiteRTModelPath),
		shellQuote(blueclaw.LiteRTModelPath),
		shellQuote(blueclaw.LiteRTModelPath),
	))
	if !localLLMIsSatisfied(context) {
		return errors.New("litert setup did not produce an isolated model and runnable litert-lm command")
	}
	fmt.Println("  " + context.T("LiteRT 준비 완료", "LiteRT ready"))
	return nil
}

func runLlamaCppStep(context *Context) error {
	spec := locallm.SpecFor(locallm.BackendLlamaCpp)
	modelPath, errorValue := resolveLocalLLMModelPath(context)
	if errorValue != nil {
		return errorValue
	}
	if modelPath != "" {
		if errorValue := copyLocalLLMModelSSH(context, modelPath); errorValue != nil {
			return errorValue
		}
	}
	context.SSH.Run(fmt.Sprintf(`set -e
install -d -o root -g root -m 700 /root/.internkim/models
if [ ! -s %s ]; then
  curl -L --fail --retry 3 --output %s.tmp %s
  mv %s.tmp %s
fi
chown root:root /root/.internkim/models %s
chmod 700 /root/.internkim/models
chmod 600 %s`,
		shellQuote(spec.RemoteModelPath),
		shellQuote(spec.RemoteModelPath),
		shellQuote(spec.ModelSourceURL),
		shellQuote(spec.RemoteModelPath),
		shellQuote(spec.RemoteModelPath),
		shellQuote(spec.RemoteModelPath),
		shellQuote(spec.RemoteModelPath),
	))
	if !localLLMIsSatisfied(context) {
		return errors.New("llama.cpp setup did not produce a runnable llama-cli with local model")
	}
	fmt.Println("  " + context.T("llama.cpp 준비 완료", "llama.cpp ready"))
	return nil
}

func stageLocalLLMModelSD(context *Context) error {
	spec := locallm.DefaultSpec()
	modelPath, errorValue := resolveLocalLLMModelPath(context)
	if errorValue != nil {
		return errorValue
	}
	if modelPath == "" {
		fmt.Println("  " + context.T("로컬 모델은 첫 부팅 중 다운로드됩니다", "Local LLM model will download on first boot"))
		return nil
	}
	document, errorValue := os.ReadFile(modelPath)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := context.SD.WriteFile(filepath.Join("models", spec.ModelFilename), document, 0o600); errorValue != nil {
		return errorValue
	}
	fmt.Println("  " + context.T("로컬 모델 스테이지 완료", "Local LLM model staged"))
	return nil
}

func resolveLocalLLMModelPath(context *Context) (string, error) {
	if context.Callbacks.GetLiteRTModelPath == nil {
		return "", nil
	}
	modelPath, errorValue := context.Callbacks.GetLiteRTModelPath(context.Force)
	if errorValue != nil {
		return "", errorValue
	}
	if modelPath == "" {
		return "", nil
	}
	return filepath.Clean(modelPath), nil
}

func copyLocalLLMModelSSH(context *Context, modelPath string) error {
	spec := locallm.DefaultSpec()
	if _, errorValue := os.Stat(modelPath); errorValue != nil {
		return errorValue
	}
	remoteTemporaryPath := "/tmp/" + spec.ModelFilename
	if errorValue := context.SSH.SCP(modelPath, remoteTemporaryPath); errorValue != nil {
		return errorValue
	}
	context.SSH.Run(fmt.Sprintf(`install -d -o root -g root -m 700 /root/.internkim/models
mv %s %s
chown root:root %s
chmod 600 %s`,
		shellQuote(remoteTemporaryPath),
		shellQuote(spec.RemoteModelPath),
		shellQuote(spec.RemoteModelPath),
		shellQuote(spec.RemoteModelPath),
	))
	return nil
}
