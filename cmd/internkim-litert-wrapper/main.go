package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type schemaRequest struct {
	Name               string          `json:"name"`
	Document           json.RawMessage `json:"document"`
	IsStrictlyEnforced bool            `json:"isStrictlyEnforced"`
}

type requestDocument struct {
	ModelPath              string        `json:"modelPath"`
	Backend                string        `json:"backend"`
	Mode                   string        `json:"mode"`
	Messages               []message     `json:"messages"`
	StructuredOutputSchema schemaRequest `json:"structuredOutputSchema"`
}

type responseDocument struct {
	Content string `json:"content"`
}

func main() {
	if errorValue := run(); errorValue != nil {
		fmt.Fprintln(os.Stderr, errorValue.Error())
		os.Exit(1)
	}
}

func run() error {
	payload, errorValue := io.ReadAll(os.Stdin)
	if errorValue != nil {
		return errorValue
	}

	var request requestDocument
	if errorValue := json.Unmarshal(payload, &request); errorValue != nil {
		return errorValue
	}
	if strings.TrimSpace(request.ModelPath) == "" {
		return errors.New("modelPath is required")
	}
	if strings.TrimSpace(request.Backend) == "" {
		return errors.New("backend is required")
	}

	content, errorValue := runLiteRT(request)
	if errorValue != nil {
		return errorValue
	}
	if request.outputMode() == "structured" && !validateMinimumStructuredOutput(content, request.StructuredOutputSchema.Document) {
		return errors.New("litert-lm output was not valid structured JSON")
	}

	return json.NewEncoder(os.Stdout).Encode(responseDocument{Content: content})
}

func runLiteRT(request requestDocument) (string, error) {
	output, errorValue := runLiteRTCommand([]string{
		"run",
		"--backend=" + strings.TrimSpace(request.Backend),
		strings.TrimSpace(request.ModelPath),
		"--prompt=" + renderPrompt(request),
	})
	if errorValue != nil && isUnsupportedBackendFlag(output) {
		if strings.TrimSpace(request.Backend) != "cpu" {
			return "", fmt.Errorf("litert-lm backend flag is unsupported for %s", strings.TrimSpace(request.Backend))
		}
		output, errorValue = runLiteRTCommand([]string{
			"run",
			strings.TrimSpace(request.ModelPath),
			"--prompt=" + renderPrompt(request),
		})
	}
	if errorValue != nil {
		return "", fmt.Errorf("litert-lm run failed: %w: %s", errorValue, strings.TrimSpace(string(output)))
	}
	if isLiteRTFailureOutput(output) {
		return "", fmt.Errorf("litert-lm run failed: %s", strings.TrimSpace(string(output)))
	}
	if request.outputMode() == "text" {
		return extractTextContent(string(output))
	}
	return extractJSONContent(string(output))
}

func runLiteRTCommand(arguments []string) ([]byte, error) {
	command := exec.Command("litert-lm", arguments...)
	command.Stdin = bytes.NewReader(nil)

	timer := time.AfterFunc(10*time.Minute, func() {
		if command.Process != nil {
			_ = command.Process.Kill()
		}
	})
	defer timer.Stop()

	output, errorValue := command.CombinedOutput()
	return output, errorValue
}

func isUnsupportedBackendFlag(output []byte) bool {
	normalizedOutput := strings.ToLower(string(output))
	return strings.Contains(normalizedOutput, "no such option") ||
		strings.Contains(normalizedOutput, "unknown option") ||
		strings.Contains(normalizedOutput, "unrecognized arguments") ||
		strings.Contains(normalizedOutput, "flag provided but not defined")
}

func isLiteRTFailureOutput(output []byte) bool {
	normalizedOutput := strings.ToLower(string(output))
	return strings.Contains(normalizedOutput, "traceback") ||
		strings.Contains(normalizedOutput, "runtimeerror:") ||
		strings.Contains(normalizedOutput, "internal: error:")
}

func renderPrompt(request requestDocument) string {
	var builder strings.Builder
	if request.outputMode() == "text" {
		builder.WriteString("You are Intern Kim. Reply with plain text only. Do not wrap the answer in JSON or markdown.\n\n")
	} else {
		builder.WriteString("You are Intern Kim. Return exactly one JSON object. Do not wrap the JSON in markdown, prose, or a code fence.\n\n")
	}
	for _, message := range request.Messages {
		role := strings.TrimSpace(message.Role)
		if role == "" {
			role = "user"
		}
		builder.WriteString(role)
		builder.WriteString(": ")
		builder.WriteString(strings.TrimSpace(message.Content))
		builder.WriteString("\n")
	}
	schemaDocument := strings.TrimSpace(string(request.StructuredOutputSchema.Document))
	if schemaDocument != "" {
		builder.WriteString("\nJSON schema:\n")
		builder.WriteString(schemaDocument)
		builder.WriteString("\n")
	}
	return builder.String()
}

func (request requestDocument) outputMode() string {
	if strings.EqualFold(strings.TrimSpace(request.Mode), "text") {
		return "text"
	}
	return "structured"
}

func extractTextContent(output string) (string, error) {
	text := strings.TrimSpace(output)
	if text == "" {
		return "", errors.New("litert-lm output was empty")
	}
	return text, nil
}

func extractJSONContent(output string) (string, error) {
	trimmedOutput := strings.TrimSpace(output)
	if isJSONDocument(trimmedOutput) {
		return trimmedOutput, nil
	}

	startIndex := strings.Index(trimmedOutput, "{")
	endIndex := strings.LastIndex(trimmedOutput, "}")
	if startIndex < 0 || endIndex <= startIndex {
		return "", errors.New("litert-lm output did not include JSON")
	}

	candidate := strings.TrimSpace(trimmedOutput[startIndex : endIndex+1])
	if !isJSONDocument(candidate) {
		return "", errors.New("litert-lm output JSON could not be parsed")
	}
	return candidate, nil
}

func validateMinimumStructuredOutput(content string, schemaDocument json.RawMessage) bool {
	var parsedContent any
	if json.Unmarshal([]byte(content), &parsedContent) != nil {
		return false
	}
	if len(bytes.TrimSpace(schemaDocument)) == 0 {
		return true
	}

	var schema struct {
		Required []string `json:"required"`
	}
	if json.Unmarshal(schemaDocument, &schema) != nil {
		return true
	}
	contentMap, isMap := parsedContent.(map[string]any)
	if !isMap {
		return len(schema.Required) == 0
	}
	for _, requiredKey := range schema.Required {
		if _, isFound := contentMap[requiredKey]; !isFound {
			return false
		}
	}
	return true
}

func isJSONDocument(value string) bool {
	var document any
	return json.Unmarshal([]byte(value), &document) == nil
}
