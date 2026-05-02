package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"gitlab.com/eastriver/internkim/internal/runtime/locallm"
)

func runLLM() {
	flagSet := flag.NewFlagSet("llm", flag.ContinueOnError)
	mode := flagSet.String("mode", "", "Execution mode: device, remote, or both (overrides --remote)")
	useRemote := flagSet.Bool("remote", false, "Use OpenRouter instead of the on-board LiteRT model")
	model := flagSet.String("model", "", "Override model name")
	provider := flagSet.String("provider", "", "Provider name: openrouter, litert, llamacpp, ollama, companion")
	accelerator := flagSet.String("accelerator", "", "Device accelerator: gpu or cpu")
	host := flagSet.String("host", "", "Board host")
	user := flagSet.String("user", boardUser, "SSH user")
	password := flagSet.String("password", "", "SSH password")
	flagArguments, positionalArguments := splitFlagsAndPositionals(os.Args[2:], map[string]bool{
		"remote": true,
	}, map[string]bool{
		"mode":        true,
		"model":       true,
		"provider":    true,
		"accelerator": true,
		"host":        true,
		"user":        true,
		"password":    true,
	})
	if errorValue := flagSet.Parse(flagArguments); errorValue != nil {
		fatal(errorValue.Error())
	}
	prompt := strings.TrimSpace(strings.Join(positionalArguments, " "))
	if prompt == "" {
		fatal("usage: internkim llm \"<prompt>\" [--remote | --mode device|remote|both] [--model NAME] [--host IP --user USER --password PASS]")
	}
	resolvedMode := strings.TrimSpace(*mode)
	if resolvedMode == "" {
		if *useRemote {
			resolvedMode = "remote"
		} else {
			resolvedMode = "device"
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
		modes = []string{"device", "remote"}
	}
	for _, requestedMode := range modes {
		if errorValue := runLLMRequest(target, requestedMode, *model, *provider, *accelerator, prompt); errorValue != nil {
			fmt.Fprintf(os.Stderr, "[%s] failed: %v\n", requestedMode, errorValue)
		}
	}
}

func splitFlagsAndPositionals(arguments []string, booleanFlags map[string]bool, valueFlags map[string]bool) ([]string, []string) {
	flagArguments := []string{}
	positionalArguments := []string{}
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		if !strings.HasPrefix(argument, "-") {
			positionalArguments = append(positionalArguments, argument)
			continue
		}
		flagBody := strings.TrimLeft(argument, "-")
		flagName := flagBody
		hasInlineValue := false
		if equalIndex := strings.Index(flagBody, "="); equalIndex >= 0 {
			flagName = flagBody[:equalIndex]
			hasInlineValue = true
		}
		switch {
		case booleanFlags[flagName]:
			flagArguments = append(flagArguments, argument)
		case valueFlags[flagName]:
			flagArguments = append(flagArguments, argument)
			if !hasInlineValue && index+1 < len(arguments) {
				index++
				flagArguments = append(flagArguments, arguments[index])
			}
		default:
			positionalArguments = append(positionalArguments, argument)
		}
	}
	return flagArguments, positionalArguments
}

func runLLMRequest(target verifyTarget, executionMode string, modelName string, providerName string, accelerator string, prompt string) error {
	body := map[string]any{
		"executionMode": executionMode,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}
	if strings.TrimSpace(modelName) != "" {
		body["model"] = modelName
	}
	if strings.TrimSpace(providerName) != "" {
		body["provider"] = providerName
	}
	if strings.TrimSpace(accelerator) != "" {
		body["accelerator"] = accelerator
	}
	bodyDocument, errorValue := json.Marshal(body)
	if errorValue != nil {
		return errorValue
	}
	if os.Getenv("INTERNKIM_LLM_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "request: %s\n", bodyDocument)
	}
	command := fmt.Sprintf("curl --silent --show-error --max-time %d --unix-socket /run/internkim/capability.sock -H 'Content-Type: application/json' -d %s http://internkim/v1/llm/text",
		int(locallm.SubprocessTimeout.Seconds()),
		quoteShellValue(string(bodyDocument)),
	)
	output, errorValue := target.sshClient.runResult(command)
	if os.Getenv("INTERNKIM_LLM_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "raw response: %s\n", strings.TrimSpace(output))
	}
	if errorValue != nil {
		return fmt.Errorf("%s: %s", errorValue, strings.TrimSpace(output))
	}

	trimmedOutput := strings.TrimSpace(output)
	if !strings.HasPrefix(trimmedOutput, "{") {
		return fmt.Errorf("%s", trimmedOutput)
	}
	var response struct {
		Content         string `json:"content"`
		SelectedBackend string `json:"selectedBackend"`
		Error           string `json:"error"`
	}
	if errorValue := json.Unmarshal([]byte(trimmedOutput), &response); errorValue != nil {
		return fmt.Errorf("decode response: %w (raw: %s)", errorValue, trimmedOutput)
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
