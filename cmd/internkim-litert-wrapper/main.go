package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const liteRTMainBinaryPath = "/usr/local/bin/litert_lm_main"
const liteRTMainLibraryDirectory = "/usr/local/lib/litert_lm"
const llamaCppBinaryPath = "/usr/local/bin/llama-cli"
const llamaCppLibraryDirectory = "/usr/local/lib/llama-cpp"

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
	if llamaCppAvailable() {
		return runLlamaCli(request)
	}
	if liteRTMainAvailable() {
		return runLiteRTMain(request)
	}
	return runLiteRTPython(request)
}

func llamaCppAvailable() bool {
	binaryInfo, errorValue := os.Stat(llamaCppBinaryPath)
	if errorValue != nil || binaryInfo.IsDir() {
		return false
	}
	libraryInfo, errorValue := os.Stat(llamaCppLibraryDirectory)
	if errorValue != nil || !libraryInfo.IsDir() {
		return false
	}
	return true
}

func runLlamaCli(request requestDocument) (string, error) {
	promptFile, errorValue := os.CreateTemp("", "llamacpp-prompt-*.txt")
	if errorValue != nil {
		return "", errorValue
	}
	defer os.Remove(promptFile.Name())
	if _, errorValue := promptFile.WriteString(renderPrompt(request)); errorValue != nil {
		_ = promptFile.Close()
		return "", errorValue
	}
	if errorValue := promptFile.Close(); errorValue != nil {
		return "", errorValue
	}

	arguments := []string{
		"-m", strings.TrimSpace(request.ModelPath),
		"-f", promptFile.Name(),
		"-n", "256",
		"-st",
		"-ngl", llamaCppGpuLayers(request),
	}
	if grammarPath, hadSchema, errorValue := writeStructuredOutputGrammarFile(request); errorValue != nil {
		return "", errorValue
	} else if hadSchema {
		defer os.Remove(grammarPath)
		arguments = append(arguments, "--grammar-file", grammarPath)
	}

	command := exec.Command(llamaCppBinaryPath, arguments...)
	command.Env = append(os.Environ(), "LD_LIBRARY_PATH="+llamaCppLibraryDirectory+pathSeparator()+os.Getenv("LD_LIBRARY_PATH"))
	command.Stdin = bytes.NewReader(nil)

	timer := time.AfterFunc(10*time.Minute, func() {
		if command.Process != nil {
			_ = command.Process.Kill()
		}
	})
	defer timer.Stop()

	stdoutBuffer := &bytes.Buffer{}
	stderrBuffer := &bytes.Buffer{}
	command.Stdout = stdoutBuffer
	command.Stderr = stderrBuffer
	if runError := command.Run(); runError != nil {
		return "", fmt.Errorf("llama-cli failed: %w: %s", runError, strings.TrimSpace(stderrBuffer.String()))
	}

	generated := extractLlamaCliGenerated(stdoutBuffer.String())
	if generated == "" {
		return "", fmt.Errorf("llama-cli produced no generated text: %s", strings.TrimSpace(stdoutBuffer.String()))
	}
	if request.outputMode() == "text" {
		return extractTextContent(generated)
	}
	return extractJSONContent(generated)
}

func llamaCppGpuLayers(request requestDocument) string {
	backend := strings.ToLower(strings.TrimSpace(request.Backend))
	if backend == "cpu" {
		return "0"
	}
	return "99"
}

// llama.cpp --json-schema is broken upstream (issue #22396); convert via the
// bundled json_schema_to_grammar.py and pass --grammar-file instead.
func writeStructuredOutputGrammarFile(request requestDocument) (string, bool, error) {
	if request.outputMode() != "structured" {
		return "", false, nil
	}
	schemaDocument := bytes.TrimSpace(request.StructuredOutputSchema.Document)
	if len(schemaDocument) == 0 {
		return "", false, nil
	}
	converterPath := filepath.Join(llamaCppLibraryDirectory, "json_schema_to_grammar.py")
	if _, statError := os.Stat(converterPath); statError != nil {
		return "", false, nil
	}

	command := exec.Command("python3", converterPath, "-")
	command.Stdin = bytes.NewReader(schemaDocument)
	stdoutBuffer := &bytes.Buffer{}
	stderrBuffer := &bytes.Buffer{}
	command.Stdout = stdoutBuffer
	command.Stderr = stderrBuffer
	if runError := command.Run(); runError != nil {
		return "", false, fmt.Errorf("json_schema_to_grammar.py failed: %w: %s", runError, strings.TrimSpace(stderrBuffer.String()))
	}

	grammarFile, errorValue := os.CreateTemp("", "llamacpp-grammar-*.gbnf")
	if errorValue != nil {
		return "", false, errorValue
	}
	if _, errorValue := grammarFile.Write(stdoutBuffer.Bytes()); errorValue != nil {
		_ = grammarFile.Close()
		_ = os.Remove(grammarFile.Name())
		return "", false, errorValue
	}
	if errorValue := grammarFile.Close(); errorValue != nil {
		_ = os.Remove(grammarFile.Name())
		return "", false, errorValue
	}
	return grammarFile.Name(), true, nil
}

func extractLlamaCliGenerated(output string) string {
	if endIndex := strings.Index(output, "[ Prompt:"); endIndex >= 0 {
		output = output[:endIndex]
	}
	if spinner := strings.LastIndex(output, "\b"); spinner >= 0 {
		output = output[spinner+1:]
	}
	return strings.TrimSpace(stripLlamaCliNoise(output))
}

func stripLlamaCliNoise(output string) string {
	noise := "|-\\/\b \t\n\r"
	for {
		next := strings.TrimLeft(output, noise)
		if next == output {
			return output
		}
		output = next
	}
}

func liteRTMainAvailable() bool {
	binaryInfo, errorValue := os.Stat(liteRTMainBinaryPath)
	if errorValue != nil || binaryInfo.IsDir() {
		return false
	}
	libraryInfo, errorValue := os.Stat(liteRTMainLibraryDirectory)
	if errorValue != nil || !libraryInfo.IsDir() {
		return false
	}
	return true
}

func runLiteRTMain(request requestDocument) (string, error) {
	promptFile, errorValue := os.CreateTemp("", "litert-prompt-*.txt")
	if errorValue != nil {
		return "", errorValue
	}
	defer os.Remove(promptFile.Name())
	if _, errorValue := promptFile.WriteString(renderPrompt(request)); errorValue != nil {
		_ = promptFile.Close()
		return "", errorValue
	}
	if errorValue := promptFile.Close(); errorValue != nil {
		return "", errorValue
	}

	command := exec.Command(liteRTMainBinaryPath,
		"--backend="+strings.TrimSpace(request.Backend),
		"--model_path="+strings.TrimSpace(request.ModelPath),
		"--input_prompt_file="+promptFile.Name(),
	)
	command.Env = append(os.Environ(), "LD_LIBRARY_PATH="+liteRTMainLibraryDirectory+pathSeparator()+os.Getenv("LD_LIBRARY_PATH"))
	command.Stdin = bytes.NewReader(nil)

	timer := time.AfterFunc(10*time.Minute, func() {
		if command.Process != nil {
			_ = command.Process.Kill()
		}
	})
	defer timer.Stop()

	stdoutBuffer := &bytes.Buffer{}
	stderrBuffer := &bytes.Buffer{}
	command.Stdout = stdoutBuffer
	command.Stderr = stderrBuffer
	runError := command.Run()
	if runError != nil {
		return "", fmt.Errorf("litert_lm_main failed: %w: %s", runError, strings.TrimSpace(stderrBuffer.String()))
	}

	generated := extractLiteRTMainGenerated(stdoutBuffer.String())
	if generated == "" {
		return "", fmt.Errorf("litert_lm_main produced no generated text: %s", strings.TrimSpace(stdoutBuffer.String()))
	}
	if request.outputMode() == "text" {
		return extractTextContent(generated)
	}
	return extractJSONContent(generated)
}

func extractLiteRTMainGenerated(output string) string {
	startMarker := "input_prompt:"
	endMarker := "BenchmarkInfo:"
	startIndex := strings.Index(output, startMarker)
	if startIndex < 0 {
		return strings.TrimSpace(output)
	}
	bodyStart := startIndex + len(startMarker)
	if newlineIndex := strings.Index(output[bodyStart:], "\n"); newlineIndex >= 0 {
		bodyStart += newlineIndex + 1
	}
	endIndex := strings.Index(output[bodyStart:], endMarker)
	body := output[bodyStart:]
	if endIndex >= 0 {
		body = output[bodyStart : bodyStart+endIndex]
	}
	return strings.TrimSpace(body)
}

func pathSeparator() string {
	if filepath.ListSeparator == 0 {
		return ":"
	}
	return string(filepath.ListSeparator)
}

func runLiteRTPython(request requestDocument) (string, error) {
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
