package cli

import (
	"fmt"
	"os"
	"strings"

	setup "gitlab.com/eastriver/internkim/internal/provisioning/steps"
)

func runSetup() {
	if containsArg("--help") || containsArg("-h") {
		printSetupUsage()
		return
	}
	lang := "ko"
	if containsArg("--en") {
		lang = "en"
	}
	sim := containsArg("--sim")
	m := newMsg(lang)

	if sim {
		runSetupSimulation(setupControlArguments(os.Args[2:]))
		return
	}

	if containsArg("--list-steps") {
		setup.DefaultRegistry().PrintSteps()
		return
	}

	boardType := argString("--board", setup.BoardJetsonOrinNano)

	// Pipeline mode: selective re-run (auto/ssh/sd backend).
	// Jetson is the default live setup target. Legacy SD flashing is still
	// available by explicitly passing --board rpi/orangepi5 without live flags.
	if hasFlag("--only") || hasFlag("--skip") || hasFlag("--from") ||
		containsArg("--force") || containsArg("--force-all") || containsArg("--plan") ||
		containsArg("--ssh") || containsArg("--sd") || containsArg("--live") ||
		containsArg("--wait-lock") ||
		argString("--host", "") != "" ||
		boardType == setup.BoardJetsonOrinNano {
		runSetupLive(m)
		return
	}

	runSetupSD(m)
}

func printSetupUsage() {
	fmt.Println("Usage: internkim setup [options]")
	fmt.Println()
	fmt.Println("Common options:")
	fmt.Println("  --only <steps>       Run only selected setup steps, for example web, admind, or capabilityd")
	fmt.Println("  --force              Re-run selected steps even when state says they are complete")
	fmt.Println("  --force-all          Re-run every selected setup step")
	fmt.Println("  --host <ip>          Override the saved board IP")
	fmt.Println("  --user <name>        Override the SSH user")
	fmt.Println("  --password <value>   Override the SSH password")
	fmt.Println("  --profile <name>     Use an isolated company/customer profile")
	fmt.Println("  --node <number>      Target a numbered fleet node")
	fmt.Println("  --fleet <fleet-id>   Join an existing fleet")
	fmt.Println("  --fleet-secret <s>   Secret for joining an existing fleet (or INTERNKIM_FLEET_SECRET)")
	fmt.Println("  --plan               Print the selected setup plan")
	fmt.Println("  --wait-lock          Wait for another setup on the same target instead of failing fast")
	fmt.Println("  --list-steps         Print available setup steps")
	fmt.Println("  --sim                Run the container lab simulation flow")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  internkim setup --only web --force")
	fmt.Println("  internkim setup --only admind --force")
	fmt.Println("  internkim setup --only web,admind --force")
	fmt.Println("  internkim setup --profile acme --node 1 --only blueclaw-payload-direct --force")
}

func containsSetupPlan(arguments []string) bool {
	for _, argument := range arguments {
		if argument == "--plan" {
			return true
		}
	}

	return false
}

func setupControlArguments(arguments []string) []string {
	var filteredArguments []string
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		switch argument {
		case "--sim", "--board", "--host", "--user":
			if argument != "--sim" && index+1 < len(arguments) {
				index++
			}
		case "--password", "--admin-email", "--openrouter-api-key", "--litert-model-path", "--relay-domain":
			if index+1 < len(arguments) {
				if argument != "--password" {
					filteredArguments = append(filteredArguments, argument)
				}
				index++
				if argument != "--password" {
					filteredArguments = append(filteredArguments, arguments[index])
				}
			}
		case "--only", "--from", "--skip":
			filteredArguments = append(filteredArguments, argument)
			if index+1 < len(arguments) {
				index++
				filteredArguments = append(filteredArguments, arguments[index])
			}
		case "--force", "--force-all", "--plan", "--list-steps", "--en", "--non-interactive", "--verify", "--verify-browser", "--wait-lock":
			filteredArguments = append(filteredArguments, argument)
		default:
			if strings.HasPrefix(argument, "--only=") ||
				strings.HasPrefix(argument, "--from=") ||
				strings.HasPrefix(argument, "--skip=") ||
				strings.HasPrefix(argument, "--admin-email=") ||
				strings.HasPrefix(argument, "--openrouter-api-key=") ||
				strings.HasPrefix(argument, "--litert-model-path=") ||
				strings.HasPrefix(argument, "--relay-domain=") {
				filteredArguments = append(filteredArguments, argument)
			}
		}
	}
	return filteredArguments
}

func resolveSetupSSHCredentials(boardType string, requestedUser string, requestedPassword string) (string, string) {
	if boardType != setup.BoardJetsonOrinNano {
		if requestedUser == "" {
			return boardUser, requestedPassword
		}
		return requestedUser, requestedPassword
	}
	if requestedUser == "" {
		requestedUser = jetsonDefaultUser
	}
	if requestedPassword == "" {
		requestedPassword = strings.TrimSpace(os.Getenv("INTERNKIM_CONSOLE_PASSWORD"))
	}
	return requestedUser, requestedPassword
}

func isBlueclawPayloadDirectOnlySetup(arguments []string) bool {
	onlyNames := setup.ParseNames(commandArgumentValue(arguments, "--only", ""))
	return len(onlyNames) == 1 && onlyNames[0] == "blueclaw-payload-direct"
}

func setupCanRunWithoutSSH(arguments []string) bool {
	if strings.TrimSpace(commandArgumentValue(arguments, "--from", "")) != "" {
		return false
	}
	onlySteps := setup.ParseNames(commandArgumentValue(arguments, "--only", ""))
	if len(onlySteps) != 1 {
		return false
	}
	return onlySteps[0] == "blueclaw-payload-direct"
}
