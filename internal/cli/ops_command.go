package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"gitlab.com/eastriver/internkim/internal/deployops"
)

func runOps() {
	if errorValue := runOpsArguments(os.Args[2:]); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runOpsArguments(arguments []string) error {
	subcommand := "serve"
	if len(arguments) > 0 && arguments[0] != "" && arguments[0][0] != '-' {
		subcommand = arguments[0]
		arguments = arguments[1:]
	}
	switch subcommand {
	case "serve":
		return runOpsServe(arguments)
	default:
		return fmt.Errorf("unknown ops subcommand: %s", subcommand)
	}
}

func runOpsServe(arguments []string) error {
	flagSet := flag.NewFlagSet("ops serve", flag.ContinueOnError)
	listenAddress := flagSet.String("listen", "127.0.0.1:8789", "Loopback listen address")
	withoutUI := flagSet.Bool("api-only", false, "Serve only the local ops API")
	if errorValue := flagSet.Parse(arguments); errorValue != nil {
		return errorValue
	}
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	executablePath, errorValue := currentExecutablePath()
	if errorValue != nil {
		return errorValue
	}
	contextValue, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return deployops.Serve(contextValue, deployops.ServerOptions{
		RepositoryRootPath: repositoryRootPath,
		ExecutablePath:     executablePath,
		ListenAddress:      *listenAddress,
		EnableSvelteUI:     !*withoutUI,
	})
}
