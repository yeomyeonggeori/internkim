package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

const (
	testSuiteCheap     = "cheap"
	testSuiteExpensive = "expensive"
	testSuiteFull      = "full"
)

type testCommandConfiguration struct {
	Suite                      string
	Prompt                     string
	DownloadDirectoryPath      string
	RunID                      string
	TimeoutSeconds             int
	GenerationSeed             *int64
	GenerationTemperature      *float64
	ShouldUseRealModels        bool
	MaximumModelTier           string
	ScenarioNames              []string
	LanguageModelProvider      string
	LanguageModelEndpoint      string
	LanguageModelSocket        string
	LanguageModelExecutionMode string
}

func runTest() {
	if errorValue := runTestArguments(os.Args[2:]); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runTestArguments(arguments []string) error {
	configuration, errorValue := parseTestArguments(arguments, time.Now())
	if errors.Is(errorValue, flag.ErrHelp) {
		return nil
	}
	if errorValue != nil {
		return errorValue
	}
	contextValue, stop := interruptContext()
	defer stop()
	return runTestConfiguration(contextValue, configuration)
}

func parseTestArguments(arguments []string, now time.Time) (testCommandConfiguration, error) {
	flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
	useRealModels := flagSet.Bool("real", false, "Use production model configuration instead of the Local Fleet test model")
	runID := flagSet.String("run-id", "", "Optional disposable Local Fleet run identifier")
	timeoutSeconds := flagSet.Int("timeout", 0, "Maximum seconds to observe scenario work; 0 disables the deadline")
	generationSeed := flagSet.Int64("seed", 0, "Generation seed to apply before the scenario prompt")
	generationTemperature := flagSet.Float64("temperature", 0, "Generation temperature to apply before the scenario prompt")
	maximumModelTier := flagSet.String("maximum-model-tier", "", "Maximum model tier for costed tests: xlow, low, medium, high, xhigh, or max")
	languageModelProviderDefault := strings.TrimSpace(os.Getenv("BLUECLAW_E2E_LLM_PROVIDER"))
	if languageModelProviderDefault == "" {
		languageModelProviderDefault = "capability"
	}
	languageModelProvider := flagSet.String("llm-provider", languageModelProviderDefault, "Live LLM provider: openrouter or capability")
	languageModelEndpoint := flagSet.String("llm-endpoint", "", "Live LLM endpoint; defaults to BLUECLAW_E2E_LLM_ENDPOINT")
	languageModelSocket := flagSet.String("llm-unix-socket", "", "Live LLM Unix socket; defaults to BLUECLAW_E2E_LLM_UNIX_SOCKET")
	languageModelExecutionMode := flagSet.String("llm-execution-mode", "", "Live LLM execution mode; defaults to BLUECLAW_E2E_LLM_EXECUTION_MODE")
	scenarioNames := repeatedStringFlag{}
	flagSet.Var(&scenarioNames, "scenario", "Run one expensive scenario by name; repeat for multiple scenarios")
	flagArguments, positionalArguments := splitFlagsAndPositionals(arguments, map[string]bool{
		"real": true,
		"help": true,
	}, map[string]bool{
		"run-id":             true,
		"timeout":            true,
		"seed":               true,
		"temperature":        true,
		"maximum-model-tier": true,
		"scenario":           true,
		"llm-provider":       true,
		"llm-endpoint":       true,
		"llm-unix-socket":    true,
		"llm-execution-mode": true,
	})
	if errorValue := flagSet.Parse(flagArguments); errorValue != nil {
		return testCommandConfiguration{}, errorValue
	}
	providedFlags := visitedFlagNames(flagSet)
	suite, prompt, errorValue := parseTestSuiteAndPrompt(positionalArguments)
	if errorValue != nil {
		return testCommandConfiguration{}, errorValue
	}
	if *timeoutSeconds < 0 {
		return testCommandConfiguration{}, errors.New("--timeout must be 0 or greater")
	}
	if providedFlags["temperature"] && (math.IsNaN(*generationTemperature) || math.IsInf(*generationTemperature, 0) || *generationTemperature < 0) {
		return testCommandConfiguration{}, errors.New("--temperature must be 0 or greater")
	}
	normalizedMaximumModelTier, errorValue := blueclaw.NormalizeMaximumModelTier(*maximumModelTier)
	if errorValue != nil {
		return testCommandConfiguration{}, errorValue
	}
	if *useRealModels && normalizedMaximumModelTier != "" {
		return testCommandConfiguration{}, errors.New("--real cannot be combined with --maximum-model-tier")
	}
	if !*useRealModels && normalizedMaximumModelTier == "" {
		normalizedMaximumModelTier = "low"
	}
	normalizedLanguageModelProvider, errorValue := normalizeTestLanguageModelProvider(*languageModelProvider)
	if errorValue != nil {
		return testCommandConfiguration{}, errorValue
	}
	defaultDownloadDirectoryPath := filepath.Join("/tmp", "internkim-test-"+now.UTC().Format("20060102T150405"))
	return testCommandConfiguration{
		Suite:                      suite,
		Prompt:                     prompt,
		DownloadDirectoryPath:      defaultDownloadDirectoryPath,
		RunID:                      strings.TrimSpace(*runID),
		TimeoutSeconds:             *timeoutSeconds,
		GenerationSeed:             optionalInt64(providedFlags["seed"], *generationSeed),
		GenerationTemperature:      optionalFloat64(providedFlags["temperature"], *generationTemperature),
		ShouldUseRealModels:        *useRealModels,
		MaximumModelTier:           normalizedMaximumModelTier,
		ScenarioNames:              scenarioNames.Values(),
		LanguageModelProvider:      normalizedLanguageModelProvider,
		LanguageModelEndpoint:      strings.TrimSpace(*languageModelEndpoint),
		LanguageModelSocket:        strings.TrimSpace(*languageModelSocket),
		LanguageModelExecutionMode: strings.TrimSpace(*languageModelExecutionMode),
	}, nil
}

func visitedFlagNames(flagSet *flag.FlagSet) map[string]bool {
	flagNames := map[string]bool{}
	flagSet.Visit(func(flagValue *flag.Flag) {
		flagNames[flagValue.Name] = true
	})
	return flagNames
}

func optionalInt64(isProvided bool, value int64) *int64 {
	if !isProvided {
		return nil
	}
	return &value
}

func optionalFloat64(isProvided bool, value float64) *float64 {
	if !isProvided {
		return nil
	}
	return &value
}

func formatOptionalInt64(value *int64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatInt(*value, 10)
}

func formatOptionalFloat64(value *float64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatFloat(*value, 'f', -1, 64)
}

func normalizeTestLanguageModelProvider(provider string) (string, error) {
	normalizedProvider := strings.ToLower(strings.TrimSpace(provider))
	switch normalizedProvider {
	case "openrouter", "capability":
		return normalizedProvider, nil
	default:
		return "", fmt.Errorf("llm provider must be openrouter or capability: %s", provider)
	}
}

func parseTestSuiteAndPrompt(positionalArguments []string) (string, string, error) {
	if len(positionalArguments) == 1 {
		candidateSuite := strings.ToLower(strings.TrimSpace(positionalArguments[0]))
		if candidateSuite == testSuiteCheap || candidateSuite == testSuiteExpensive || candidateSuite == testSuiteFull {
			return candidateSuite, "", nil
		}
	}
	prompt := strings.TrimSpace(strings.Join(positionalArguments, " "))
	if prompt == "" {
		return "", "", errors.New("usage: internkim test <cheap|expensive|full> [--scenario name] [--maximum-model-tier xlow] [--real] or internkim test \"<prompt>\"")
	}
	return "", prompt, nil
}

func runTestConfiguration(contextValue context.Context, configuration testCommandConfiguration) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	if configuration.Suite == "" {
		return errors.New("name a suite: `internkim test cheap`, or `internkim test expensive --scenario <name>`")
	}
	return runTestSuite(contextValue, repositoryRootPath, configuration)
}

func runTestSuite(contextValue context.Context, repositoryRootPath string, configuration testCommandConfiguration) error {
	if configuration.Suite == testSuiteExpensive || configuration.Suite == testSuiteFull {
		return refuseTheExpensiveSuite()
	}
	if configuration.Suite == testSuiteCheap {
		return runCheapTestSuite(contextValue, repositoryRootPath)
	}
	return fmt.Errorf("unsupported test suite: %s", configuration.Suite)
}

func runCheapTestSuite(contextValue context.Context, repositoryRootPath string) error {
	fmt.Println("Test suite: cheap")
	command := exec.CommandContext(contextValue, "make", "check")
	command.Dir = repositoryRootPath
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if errorValue := command.Run(); errorValue != nil {
		return fmt.Errorf("cheap test suite failed: %w", errorValue)
	}
	return nil
}

func refuseTheExpensiveSuite() error {
	return errors.New("the expensive suite drove tests/expensive through the messenger it was written for, and that driver is gone. " +
		"The Linux acceptance gate is now `internkim dev fleet run --scenario buzz-attachment` and `--scenario buzz-direct-message`; " +
		"tests/expensive stays as the specification a Buzz driver has to satisfy")
}
