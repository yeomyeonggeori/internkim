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

	"gitlab.com/eastriver/internkim/internal/runtime/locallm"
)

const liteRTMainBinaryPath = "/usr/local/bin/litert_lm_main"
const liteRTConstrainedBinaryPath = "/usr/local/bin/internkim-litert-constrained"
const liteRTMainLibraryDirectory = "/usr/local/lib/litert_lm"

type message struct {
	Role    string        `json:"role"`
	Content string        `json:"content"`
	Parts   []messagePart `json:"parts,omitempty"`
}

type messagePart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type schemaRequest struct {
	Name               string          `json:"name"`
	Document           json.RawMessage `json:"document"`
	IsStrictlyEnforced bool            `json:"isStrictlyEnforced"`
}

type requestDocument struct {
	ModelPath           string            `json:"modelPath"`
	Accelerator         string            `json:"accelerator"`
	Mode                string            `json:"mode"`
	Messages            []message         `json:"messages"`
	ConstrainedDecoding constraintRequest `json:"constrainedDecoding"`
}

type responseDocument struct {
	Content        string `json:"content"`
	ConstraintMode string `json:"constraintMode,omitempty"`
}

type constraintRequest struct {
	Type       string        `json:"type"`
	JSONSchema schemaRequest `json:"jsonSchema"`
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
	if request.accelerator() == "" {
		return errors.New("accelerator is required")
	}

	response, errorValue := runLocalLLM(request)
	if errorValue != nil {
		return errorValue
	}
	if request.outputMode() == "structured" && !isJSONDocument(response.Content) {
		return errors.New("local LLM output was not valid structured JSON")
	}

	return json.NewEncoder(os.Stdout).Encode(response)
}

func runLocalLLM(request requestDocument) (responseDocument, error) {
	if request.outputMode() == "structured" {
		return runConstrainedLiteRT(request)
	}
	if liteRTMainAvailable() {
		return runMainLiteRT(request)
	}
	return responseDocument{}, errors.New("local text output requires litert_lm_main")
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

func runConstrainedLiteRT(request requestDocument) (responseDocument, error) {
	if _, errorValue := os.Stat(liteRTConstrainedBinaryPath); errorValue != nil {
		return responseDocument{}, fmt.Errorf("LiteRT structured output requires constrained decoding runner at %s: %w", liteRTConstrainedBinaryPath, errorValue)
	}
	document, errorValue := json.Marshal(request)
	if errorValue != nil {
		return responseDocument{}, errorValue
	}
	command := exec.Command(liteRTConstrainedBinaryPath)
	command.Env = append(os.Environ(), "LD_LIBRARY_PATH="+liteRTMainLibraryDirectory+pathSeparator()+os.Getenv("LD_LIBRARY_PATH"))
	command.Stdin = bytes.NewReader(document)

	timer := time.AfterFunc(locallm.SubprocessTimeout, func() {
		if command.Process != nil {
			_ = command.Process.Kill()
		}
	})
	defer timer.Stop()

	output, errorValue := command.Output()
	if errorValue != nil {
		return responseDocument{}, fmt.Errorf("LiteRT constrained decoding failed: %w", errorValue)
	}
	var response responseDocument
	if errorValue := json.Unmarshal(output, &response); errorValue != nil {
		return responseDocument{}, errorValue
	}
	if response.ConstraintMode == "" {
		response.ConstraintMode = "litert_llguidance_json_schema"
	}
	return response, nil
}

func runMainLiteRT(request requestDocument) (responseDocument, error) {
	promptFile, errorValue := os.CreateTemp("", "litert-prompt-*.txt")
	if errorValue != nil {
		return responseDocument{}, errorValue
	}
	defer os.Remove(promptFile.Name())
	if _, errorValue := promptFile.WriteString(renderPrompt(request)); errorValue != nil {
		_ = promptFile.Close()
		return responseDocument{}, errorValue
	}
	if errorValue := promptFile.Close(); errorValue != nil {
		return responseDocument{}, errorValue
	}

	command := exec.Command(liteRTMainBinaryPath,
		"--backend="+request.accelerator(),
		"--model_path="+strings.TrimSpace(request.ModelPath),
		"--input_prompt_file="+promptFile.Name(),
	)
	command.Env = append(os.Environ(), "LD_LIBRARY_PATH="+liteRTMainLibraryDirectory+pathSeparator()+os.Getenv("LD_LIBRARY_PATH"))
	command.Stdin = bytes.NewReader(nil)

	timer := time.AfterFunc(locallm.SubprocessTimeout, func() {
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
		return responseDocument{}, fmt.Errorf("litert_lm_main failed: %w: %s", runError, strings.TrimSpace(stderrBuffer.String()))
	}

	generated := extractLiteRTMainGenerated(stdoutBuffer.String())
	if generated == "" {
		return responseDocument{}, fmt.Errorf("litert_lm_main produced no generated text: %s", strings.TrimSpace(stdoutBuffer.String()))
	}
	content, errorValue := extractTextContent(generated)
	return responseDocument{Content: content}, errorValue
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

func renderPrompt(request requestDocument) string {
	var builder strings.Builder
	for _, message := range request.Messages {
		role := strings.TrimSpace(message.Role)
		if role == "" {
			role = "user"
		}
		builder.WriteString(role)
		builder.WriteString(": ")
		builder.WriteString(strings.TrimSpace(messageTextContent(message)))
		builder.WriteString("\n")
	}
	return builder.String()
}

func messageTextContent(message message) string {
	parts := []string{}
	if strings.TrimSpace(message.Content) != "" {
		parts = append(parts, message.Content)
	}
	for _, part := range message.Parts {
		switch strings.TrimSpace(part.Type) {
		case "text":
			if strings.TrimSpace(part.Text) != "" {
				parts = append(parts, part.Text)
			}
		case "image":
			parts = append(parts, "[image input omitted by local runner]")
		}
	}
	return strings.Join(parts, "\n")
}

func (request requestDocument) outputMode() string {
	if strings.EqualFold(strings.TrimSpace(request.Mode), "text") {
		return "text"
	}
	return "structured"
}

func (request requestDocument) accelerator() string {
	if accelerator := strings.TrimSpace(request.Accelerator); accelerator != "" {
		return accelerator
	}
	return ""
}

func (request requestDocument) schemaDocument() json.RawMessage {
	return request.ConstrainedDecoding.JSONSchema.Document
}

func extractTextContent(output string) (string, error) {
	text := strings.TrimSpace(output)
	if text == "" {
		return "", errors.New("litert-lm output was empty")
	}
	return text, nil
}

func isJSONDocument(value string) bool {
	var document any
	return json.Unmarshal([]byte(value), &document) == nil
}
