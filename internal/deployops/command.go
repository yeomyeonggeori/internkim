package deployops

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"gitlab.com/eastriver/internkim/internal/localfleet"
)

type CommandPlan struct {
	DirectoryPath string
	Name          string
	Arguments     []string
	Environment   []string
}

func (server *Server) runJob(contextValue context.Context, job *JobRunner, target Target, action string) {
	switch action {
	case JobActionCheck:
		status := server.CheckStatus(contextValue, target)
		job.Info(formatStatus(status))
	case JobActionDeployAdmind:
		job.Error(server.runCommandPlan(contextValue, job, deployCommand(server.options.RepositoryRootPath, server.options.ExecutablePath, "admind")))
	case JobActionDeployRuntime:
		job.Error(server.runCommandPlan(contextValue, job, deployCommand(server.options.RepositoryRootPath, server.options.ExecutablePath, "capabilityd,blueclawPayload,skills")))
	case JobActionDeployWeb:
		job.Error(server.runCommandPlan(contextValue, job, deployCommand(server.options.RepositoryRootPath, server.options.ExecutablePath, "web")))
	case JobActionApplyRelease:
		job.Error(server.runCommandPlan(contextValue, job, updateApplyCommand(server.options.RepositoryRootPath, server.options.ExecutablePath)))
	case JobActionPilotStandard:
		server.runPilotStandardDeploy(contextValue, job, target)
	case JobActionRestartSSH, JobActionRestartSSHRoute:
		job.Error(server.runCommandPlan(contextValue, job, recoveryCommand(server.options.RepositoryRootPath, server.options.ExecutablePath, action)))
	default:
		job.Error(fmt.Errorf("unsupported job action: %s", action))
	}
}

func (server *Server) runPilotStandardDeploy(contextValue context.Context, job *JobRunner, target Target) {
	if errorValue := server.runCommandPlan(contextValue, job, CommandPlan{DirectoryPath: server.options.RepositoryRootPath, Name: "make", Arguments: []string{"build"}, Environment: os.Environ()}); errorValue != nil {
		job.Error(errorValue)
		return
	}
	if errorValue := server.runCommandPlan(contextValue, job, deployCommand(server.options.RepositoryRootPath, server.options.ExecutablePath, "admind")); errorValue != nil {
		job.Error(errorValue)
		return
	}
	job.Info(formatStatus(server.CheckStatus(contextValue, target)))
	if errorValue := server.runCommandPlan(contextValue, job, deployCommand(server.options.RepositoryRootPath, server.options.ExecutablePath, "capabilityd,blueclawPayload,skills")); errorValue != nil {
		job.Error(errorValue)
		return
	}
	job.Info(formatStatus(server.CheckStatus(contextValue, target)))
}

func (server *Server) runLocalFleetJob(contextValue context.Context, job *JobRunner, request localfleet.JobRequest) {
	service, errorValue := server.localFleetService()
	if errorValue != nil {
		job.Error(errorValue)
		return
	}
	job.Error(service.Run(contextValue, job, request))
}

func (server *Server) runCommandPlan(contextValue context.Context, job *JobRunner, plan CommandPlan) error {
	job.Info("$ " + strings.Join(append([]string{plan.Name}, plan.Arguments...), " "))
	command := exec.CommandContext(contextValue, plan.Name, plan.Arguments...)
	command.Dir = plan.DirectoryPath
	command.Env = plan.Environment
	outputPipe, errorValue := command.StdoutPipe()
	if errorValue != nil {
		return errorValue
	}
	errorPipe, errorValue := command.StderrPipe()
	if errorValue != nil {
		return errorValue
	}
	if errorValue := command.Start(); errorValue != nil {
		return errorValue
	}
	done := make(chan struct{}, 2)
	go scanLines(outputPipe, job, done)
	go scanLines(errorPipe, job, done)
	<-done
	<-done
	if errorValue := command.Wait(); errorValue != nil {
		return errorValue
	}
	return nil
}

func scanLines(pipe interface{ Read([]byte) (int, error) }, job *JobRunner, done chan struct{}) {
	defer func() { done <- struct{}{} }()
	scanner := bufio.NewScanner(pipe)
	buffer := make([]byte, 0, 1024*64)
	scanner.Buffer(buffer, 1024*1024)
	for scanner.Scan() {
		job.appendLine(scanner.Text())
	}
}

func runBufferedCommand(contextValue context.Context, plan CommandPlan) (string, error) {
	command := exec.CommandContext(contextValue, plan.Name, plan.Arguments...)
	command.Dir = plan.DirectoryPath
	command.Env = plan.Environment
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	errorValue := command.Run()
	return output.String(), errorValue
}

func deployCommand(repositoryRootPath string, executablePath string, components string) CommandPlan {
	return internkimCommand(repositoryRootPath, executablePath, "deploy", "--components", components)
}

func updateApplyCommand(repositoryRootPath string, executablePath string) CommandPlan {
	return internkimCommand(repositoryRootPath, executablePath, "update", "apply")
}

func recoveryCommand(repositoryRootPath string, executablePath string, action string) CommandPlan {
	return internkimCommand(repositoryRootPath, executablePath, "recover", "ssh", "--action", action)
}

func internkimCommand(repositoryRootPath string, executablePath string, arguments ...string) CommandPlan {
	return CommandPlan{
		DirectoryPath: repositoryRootPath,
		Name:          executablePath,
		Arguments:     arguments,
		Environment:   os.Environ(),
	}
}
