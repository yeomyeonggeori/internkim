package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

func runLLM() {
	flagSet := flag.NewFlagSet("llm", flag.ContinueOnError)
	mode := flagSet.String("mode", "", "Execution mode: local, remote, or both (overrides --remote)")
	useRemote := flagSet.Bool("remote", false, "Use OpenRouter instead of the on-board LiteRT model")
	model := flagSet.String("model", "", "Override model name")
	host := flagSet.String("host", "", "Board host")
	user := flagSet.String("user", boardUser, "SSH user")
	password := flagSet.String("password", "", "SSH password")
	if errorValue := flagSet.Parse(os.Args[2:]); errorValue != nil {
		fatal(errorValue.Error())
	}
	prompt := strings.TrimSpace(strings.Join(flagSet.Args(), " "))
	if prompt == "" {
		fatal("usage: internkim llm \"<prompt>\" [--remote | --mode local|remote|both] [--model NAME] [--host IP --user USER --password PASS]")
	}
	resolvedMode := strings.TrimSpace(*mode)
	if resolvedMode == "" {
		if *useRemote {
			resolvedMode = "remote"
		} else {
			resolvedMode = "local"
		}
	}

	verifyArguments := []string{}
	if strings.TrimSpace(*host) != "" {
		verifyArguments = append(verifyArguments, "--host", *host)
	}
	if strings.TrimSpace(*user) != "" {
		verifyArguments = append(verifyArguments, "--user", *user)
	}
	if strings.TrimSpace(*password) != "" {
		verifyArguments = append(verifyArguments, "--password", *password)
	}
	target, errorValue := resolveVerifyTarget(verifyArguments)
	if errorValue != nil {
		fatal(errorValue.Error())
	}

	modes := []string{resolvedMode}
	if strings.EqualFold(resolvedMode, "both") {
		modes = []string{"local", "remote"}
	}
	for _, requestedMode := range modes {
		if errorValue := runLLMRequest(target, requestedMode, *model, prompt); errorValue != nil {
			fmt.Fprintf(os.Stderr, "[%s] failed: %v\n", requestedMode, errorValue)
		}
	}
}

func runLLMRequest(target verifyTarget, executionMode string, modelName string, prompt string) error {
	body := map[string]any{
		"executionMode": executionMode,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}
	if strings.TrimSpace(modelName) != "" {
		body["model"] = modelName
	}
	bodyDocument, errorValue := json.Marshal(body)
	if errorValue != nil {
		return errorValue
	}
	command := "curl --silent --show-error --max-time 60 --unix-socket /run/internkim/capability.sock -H 'Content-Type: application/json' -d " + quoteShellValue(string(bodyDocument)) + " http://internkim/v1/llm/text"
	output, errorValue := target.sshClient.runResult(command)
	if errorValue != nil {
		return fmt.Errorf("%s: %s", errorValue, strings.TrimSpace(output))
	}

	var response struct {
		Content         string `json:"content"`
		SelectedBackend string `json:"selectedBackend"`
		Error           string `json:"error"`
	}
	if errorValue := json.Unmarshal([]byte(output), &response); errorValue != nil {
		return fmt.Errorf("decode response: %w (raw: %s)", errorValue, strings.TrimSpace(output))
	}
	if response.Error != "" {
		return fmt.Errorf("%s", response.Error)
	}
	header := fmt.Sprintf("[%s", executionMode)
	if response.SelectedBackend != "" && !strings.EqualFold(response.SelectedBackend, executionMode) {
		header += " → " + response.SelectedBackend
	}
	header += "]"
	fmt.Println(header)
	fmt.Println(strings.TrimRight(response.Content, "\n"))
	fmt.Println()
	return nil
}
