package capabilityd

import (
	"context"
	"log"
	"os/exec"

	"github.com/yeomyeonggeori/internkim/internal/runtime/locallm"
)

func (service Service) applyLocalInferenceMode(ctx context.Context) {
	switch service.localInferenceMode() {
	case "remote":
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
	if !service.hasServiceManager() {
		return
	}
	if _, errorValue := service.runCommand(ctx, "systemctl", arguments, nil); errorValue != nil {
		log.Printf("local inference service command failed: systemctl %v: %v", arguments, errorValue)
	}
}

func (service Service) hasServiceManager() bool {
	lookupExecutable := service.LookupExecutable
	if lookupExecutable == nil {
		lookupExecutable = exec.LookPath
	}
	_, errorValue := lookupExecutable("systemctl")
	return errorValue == nil
}
