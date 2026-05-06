package capabilityd

import (
	"context"
	"log"

	"gitlab.com/eastriver/internkim/internal/runtime/locallm"
)

func (service Service) applyLocalInferenceMode(ctx context.Context) {
	switch service.localInferenceMode() {
	case "companion_preferred", "companion_only", "remote":
		service.runLocalInferenceServiceCommand(ctx, "stop", locallm.LlamaCppServiceName)
		service.runLocalInferenceServiceCommand(ctx, "disable", locallm.LlamaCppServiceName)
	case "device":
		service.runLocalInferenceServiceCommand(ctx, "enable", "--now", locallm.LlamaCppServiceName)
	}
}

func (service Service) runLocalInferenceServiceCommand(ctx context.Context, arguments ...string) {
	if len(arguments) == 0 {
		return
	}
	if _, errorValue := service.runCommand(ctx, "systemctl", arguments, nil); errorValue != nil {
		log.Printf("local inference service command failed: systemctl %v: %v", arguments, errorValue)
	}
}
