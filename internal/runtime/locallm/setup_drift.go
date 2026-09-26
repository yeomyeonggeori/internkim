package locallm

import (
	"fmt"
	"strings"
)

type servedModel struct {
	serviceName string
	servicePath string
	modelPath   string
}

var servedModels = []servedModel{
	{serviceName: LlamaCppServiceName, servicePath: LlamaCppServicePath, modelPath: LlamaCppModelPath},
	{serviceName: LlamaCppEmbeddingServiceName, servicePath: LlamaCppEmbeddingServicePath, modelPath: LlamaCppEmbeddingModelPath},
}

func UnitServesModel(unitText string, modelPath string) bool {
	return strings.Contains(unitText, " -m "+modelPath+" ")
}

func EmbeddingUnitServesCurrentModel(unitText string) bool {
	return UnitServesModel(unitText, LlamaCppEmbeddingModelPath)
}

func SetupDrift(readFile func(string) ([]byte, error)) []string {
	drift := []string{}
	for _, model := range servedModels {
		unit, errorValue := readFile(model.servicePath)
		if errorValue != nil {
			continue
		}
		if !UnitServesModel(string(unit), model.modelPath) {
			drift = append(drift, fmt.Sprintf("%s does not serve %s; a release does not reinstall models, so run ./internkim setup --only local-llm,services", model.serviceName, model.modelPath))
		}
	}
	return drift
}
