package computer

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// The legacy initialize flow every MCP client speaks; cua-driver keeps it beside
// the 2026-07-28 revision. libs/cua-driver/docs/mcp-protocol-and-skills.md
const mcpProtocolVersion = "2025-06-18"

const largestMCPLine = 64 * 1024 * 1024

type MCPDriverOpener struct {
	ExecutablePath string
}

func (opener MCPDriverOpener) Open(ctx context.Context, sessionLabel string) (Driver, error) {
	command := exec.CommandContext(ctx, opener.ExecutablePath, "mcp")
	command.Stderr = io.Discard
	toServer, errorValue := command.StdinPipe()
	if errorValue != nil {
		return nil, errorValue
	}
	fromServer, errorValue := command.StdoutPipe()
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := command.Start(); errorValue != nil {
		return nil, fmt.Errorf("start cua-driver: %w", errorValue)
	}
	driver := &mcpDriver{
		command:      command,
		toServer:     toServer,
		sessionLabel: sessionLabel,
		answers:      map[int64]chan json.RawMessage{},
	}
	go driver.readAnswers(fromServer)
	if errorValue := driver.initialize(ctx); errorValue != nil {
		_ = driver.Close()
		return nil, errorValue
	}
	return driver, nil
}

type mcpDriver struct {
	command      *exec.Cmd
	toServer     io.WriteCloser
	sessionLabel string
	writeMutex   sync.Mutex
	answersMutex sync.Mutex
	answers      map[int64]chan json.RawMessage
	nextID       int64
}

type jsonRPCRequest struct {
	Version string `json:"jsonrpc"`
	ID      *int64 `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type jsonRPCAnswer struct {
	ID     *int64          `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *jsonRPCError   `json:"error"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolCallResult struct {
	Content           []toolContent   `json:"content"`
	StructuredContent json.RawMessage `json:"structuredContent"`
	IsError           bool            `json:"isError"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (driver *mcpDriver) initialize(ctx context.Context) error {
	_, errorValue := driver.request(ctx, "initialize", map[string]any{
		"protocolVersion": mcpProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "internkim-companion", "version": "1"},
	})
	if errorValue != nil {
		return fmt.Errorf("cua-driver did not initialize: %w", errorValue)
	}
	return driver.notify("notifications/initialized")
}

func (driver *mcpDriver) Call(ctx context.Context, tool string, arguments map[string]any) (json.RawMessage, error) {
	labelled := map[string]any{"session": driver.sessionLabel}
	for name, value := range arguments {
		labelled[name] = value
	}
	answer, errorValue := driver.request(ctx, "tools/call", map[string]any{"name": tool, "arguments": labelled})
	if errorValue != nil {
		return nil, fmt.Errorf("%s: %w", tool, errorValue)
	}
	return structuredResultOf(tool, answer)
}

func structuredResultOf(tool string, answer json.RawMessage) (json.RawMessage, error) {
	var result toolCallResult
	if errorValue := json.Unmarshal(answer, &result); errorValue != nil {
		return nil, fmt.Errorf("%s answered with something other than a tool result: %w", tool, errorValue)
	}
	if result.IsError {
		return nil, fmt.Errorf("%s failed: %s", tool, textOf(result.Content))
	}
	if len(result.StructuredContent) == 0 {
		return nil, fmt.Errorf("%s returned no structured result", tool)
	}
	if refusal := refusalOf(result.StructuredContent); refusal != "" {
		return nil, fmt.Errorf("%s refused: %s", tool, refusal)
	}
	return result.StructuredContent, nil
}

func textOf(content []toolContent) string {
	texts := []string{}
	for _, part := range content {
		if part.Type == "text" && strings.TrimSpace(part.Text) != "" {
			texts = append(texts, strings.TrimSpace(part.Text))
		}
	}
	return strings.Join(texts, " ")
}

func refusalOf(document json.RawMessage) string {
	var envelope struct {
		Status  string          `json:"status"`
		Refusal json.RawMessage `json:"refusal"`
	}
	if errorValue := json.Unmarshal(document, &envelope); errorValue != nil {
		return ""
	}
	if envelope.Status == "refused" || len(envelope.Refusal) > 0 && string(envelope.Refusal) != "null" {
		return string(envelope.Refusal)
	}
	return ""
}

func (driver *mcpDriver) request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	driver.answersMutex.Lock()
	driver.nextID++
	id := driver.nextID
	answer := make(chan json.RawMessage, 1)
	driver.answers[id] = answer
	driver.answersMutex.Unlock()

	if errorValue := driver.write(jsonRPCRequest{Version: "2.0", ID: &id, Method: method, Params: params}); errorValue != nil {
		driver.forget(id)
		return nil, errorValue
	}
	select {
	case document, isOpen := <-answer:
		if !isOpen {
			return nil, errors.New("cua-driver closed the connection")
		}
		return parseAnswer(document)
	case <-ctx.Done():
		driver.forget(id)
		return nil, ctx.Err()
	}
}

func parseAnswer(document json.RawMessage) (json.RawMessage, error) {
	var answer jsonRPCAnswer
	if errorValue := json.Unmarshal(document, &answer); errorValue != nil {
		return nil, errorValue
	}
	if answer.Error != nil {
		return nil, fmt.Errorf("%s (code %d)", answer.Error.Message, answer.Error.Code)
	}
	return answer.Result, nil
}

func (driver *mcpDriver) notify(method string) error {
	return driver.write(jsonRPCRequest{Version: "2.0", Method: method})
}

func (driver *mcpDriver) write(request jsonRPCRequest) error {
	document, errorValue := json.Marshal(request)
	if errorValue != nil {
		return errorValue
	}
	driver.writeMutex.Lock()
	defer driver.writeMutex.Unlock()
	_, errorValue = driver.toServer.Write(append(document, '\n'))
	return errorValue
}

func (driver *mcpDriver) forget(id int64) {
	driver.answersMutex.Lock()
	delete(driver.answers, id)
	driver.answersMutex.Unlock()
}

func (driver *mcpDriver) readAnswers(fromServer io.Reader) {
	scanner := bufio.NewScanner(fromServer)
	scanner.Buffer(make([]byte, 0, 1024*1024), largestMCPLine)
	for scanner.Scan() {
		line := scanner.Bytes()
		var answer jsonRPCAnswer
		if json.Unmarshal(line, &answer) != nil || answer.ID == nil {
			continue
		}
		driver.deliver(*answer.ID, append(json.RawMessage{}, line...))
	}
	driver.closeAnswers()
}

func (driver *mcpDriver) deliver(id int64, document json.RawMessage) {
	driver.answersMutex.Lock()
	answer, isWaiting := driver.answers[id]
	delete(driver.answers, id)
	driver.answersMutex.Unlock()
	if isWaiting {
		answer <- document
	}
}

func (driver *mcpDriver) closeAnswers() {
	driver.answersMutex.Lock()
	defer driver.answersMutex.Unlock()
	for id, answer := range driver.answers {
		close(answer)
		delete(driver.answers, id)
	}
}

func (driver *mcpDriver) Close() error {
	_ = driver.toServer.Close()
	if driver.command.Process != nil {
		_ = driver.command.Process.Kill()
	}
	_ = driver.command.Wait()
	return nil
}
