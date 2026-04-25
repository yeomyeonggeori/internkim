package setup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/anthropic-lab/internkim/internal/runtime/blueclaw"
)

var StepLiteRT = Step{
	Name: "litert",
	Deps: []string{"binaries"},
	Title: func(context *Context) string {
		return context.T("LiteRT 로컬 모델 준비...", "Preparing LiteRT local model...")
	},
	IsSatisfied: func(context *Context) bool {
		return liteRTIsSatisfied(context)
	},
	Run: func(context *Context) error {
		if context.SSH == nil {
			return errors.New("litert SSH callback missing")
		}

		modelPath, errorValue := resolveLiteRTModelPath(context)
		if errorValue != nil {
			return errorValue
		}
		if modelPath != "" {
			if errorValue := copyLiteRTModelSSH(context, modelPath); errorValue != nil {
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
		if !liteRTIsSatisfied(context) {
			return errors.New("litert setup did not produce an isolated model and runnable litert-lm command")
		}
		fmt.Println("  " + context.T("LiteRT 준비 완료", "LiteRT ready"))
		return nil
	},
	RunSD: func(context *Context) error {
		modelPath, errorValue := resolveLiteRTModelPath(context)
		if errorValue != nil {
			return errorValue
		}
		if modelPath == "" {
			fmt.Println("  " + context.T("LiteRT 모델은 첫 부팅 중 다운로드됩니다", "LiteRT model will download on first boot"))
			return nil
		}
		document, errorValue := os.ReadFile(modelPath)
		if errorValue != nil {
			return errorValue
		}
		if errorValue := context.SD.WriteFile(filepath.Join("models", blueclaw.LiteRTModelFilename), document, 0o600); errorValue != nil {
			return errorValue
		}
		fmt.Println("  " + context.T("LiteRT 모델 스테이지 완료", "LiteRT model staged"))
		return nil
	},
}

func liteRTIsSatisfied(context *Context) bool {
	switch context.Backend {
	case BackendSSH:
		return trimmedRun(context, `test -x /usr/local/bin/internkim-litert-wrapper && command -v litert-lm >/dev/null 2>&1 && test -s /root/.internkim/models/gemma-4-E4B-it.litertlm && ! su -s /bin/sh blueclaw -c 'test -r /root/.internkim/models/gemma-4-E4B-it.litertlm' 2>/dev/null && echo y || echo n`) == "y"
	case BackendSD:
		return stagedFileExists(context, "models/gemma-4-E4B-it.litertlm")
	}
	return false
}

func resolveLiteRTModelPath(context *Context) (string, error) {
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

func copyLiteRTModelSSH(context *Context, modelPath string) error {
	if _, errorValue := os.Stat(modelPath); errorValue != nil {
		return errorValue
	}
	remoteTemporaryPath := "/tmp/" + blueclaw.LiteRTModelFilename
	if errorValue := context.SSH.SCP(modelPath, remoteTemporaryPath); errorValue != nil {
		return errorValue
	}
	context.SSH.Run(fmt.Sprintf(`install -d -o root -g root -m 700 /root/.internkim/models
mv %s %s
chown root:root %s
chmod 600 %s`,
		shellQuote(remoteTemporaryPath),
		shellQuote(blueclaw.LiteRTModelPath),
		shellQuote(blueclaw.LiteRTModelPath),
		shellQuote(blueclaw.LiteRTModelPath),
	))
	return nil
}
