package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const defaultTaskListLimit = 20
const taskEventBodyPreviewLimit = 200
const taskSummaryPreviewLimit = 80

type commandTaskRun struct {
	TaskRunID               string    `json:"taskRunID"`
	RequesterPersonID       string    `json:"requesterPersonID"`
	CurrentAgentProfileName string    `json:"currentAgentProfileName"`
	Status                  string    `json:"status"`
	Prompt                  string    `json:"prompt"`
	Result                  string    `json:"result"`
	FailureReason           string    `json:"failureReason"`
	CreatedAt               time.Time `json:"createdAt"`
	UpdatedAt               time.Time `json:"updatedAt"`
}

type commandTaskStep struct {
	TaskStepID  string `json:"taskStepID"`
	Instruction string `json:"instruction"`
	Status      string `json:"status"`
	Output      string `json:"output"`
}

type commandTaskEvent struct {
	Name      string    `json:"name"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}

type commandTaskDetail struct {
	TaskRun    commandTaskRun     `json:"taskRun"`
	TaskSteps  []commandTaskStep  `json:"taskSteps"`
	TaskEvents []commandTaskEvent `json:"taskEvents"`
}

var runCommandOutput io.Writer = os.Stdout

func runTaskRun() {
	if errorValue := runTaskArguments(os.Args[2:]); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runTaskArguments(arguments []string) error {
	if hasCommandArgument(arguments, "--help") || hasCommandArgument(arguments, "-h") {
		printTaskUsage()
		return nil
	}
	command, commandArguments := splitTaskCommand(arguments)
	if errorValue := validateTaskArguments(command, commandArguments); errorValue != nil {
		return errorValue
	}
	client, errorValue := resolveAdminAPIClient(arguments)
	if errorValue != nil {
		return errorValue
	}
	return runTaskArgumentsWithClient(arguments, client)
}

func runTaskArgumentsWithClient(arguments []string, client adminAPIClient) error {
	command, commandArguments := splitTaskCommand(arguments)
	switch command {
	case "list":
		return listTaskRunsWithClient(commandArguments, client)
	case "logs":
		return showTaskLogsWithClient(commandArguments, client)
	default:
		return fmt.Errorf("unknown task command %q", command)
	}
}

func splitTaskCommand(arguments []string) (string, []string) {
	if len(arguments) > 0 && !strings.HasPrefix(arguments[0], "-") {
		return arguments[0], arguments[1:]
	}
	return "list", arguments
}

func validateTaskArguments(command string, arguments []string) error {
	switch command {
	case "list":
		return nil
	case "logs":
		return taskLogsUsageError(arguments)
	default:
		return fmt.Errorf("unknown task command %q", command)
	}
}

func taskLogsUsageError(arguments []string) error {
	if firstTaskPositional(arguments) == "" && !hasCommandArgument(arguments, "--last-failed") {
		return errors.New("usage: internkim run logs <taskRunID> | --last-failed")
	}
	return nil
}

func listTaskRunsWithClient(arguments []string, client adminAPIClient) error {
	status := taskStatusFilterFromArguments(arguments)
	limit := taskLimitFromArguments(arguments)
	taskRuns, errorValue := fetchTaskRuns(client, taskListQuery(status, limit))
	if errorValue != nil {
		return errorValue
	}
	filteredTaskRuns := filterTaskRunsByStatus(taskRuns, status)
	limitedTaskRuns := limitTaskRuns(filteredTaskRuns, limit)
	if hasCommandArgument(arguments, "--json") {
		return printTaskJSON(limitedTaskRuns)
	}
	printTaskRunTable(limitedTaskRuns)
	return nil
}

func showTaskLogsWithClient(arguments []string, client adminAPIClient) error {
	taskRunID, errorValue := resolveTaskRunID(arguments, client)
	if errorValue != nil {
		return errorValue
	}
	detail := commandTaskDetail{}
	rawResponse, errorValue := client.request("GET", "/diagnostics/task-detail?taskRunID="+url.QueryEscape(taskRunID), nil, &detail)
	if errorValue != nil {
		return errorValue
	}
	if hasCommandArgument(arguments, "--json") {
		fmt.Fprintln(runCommandOutput, string(rawResponse))
		return nil
	}
	printTaskDetail(detail, hasCommandArgument(arguments, "--full"))
	return nil
}

func taskListQuery(status string, limit int) string {
	values := url.Values{}
	if status != "" {
		values.Set("status", status)
	}
	values.Set("limit", strconv.Itoa(limit))
	return values.Encode()
}

func fetchTaskRuns(client adminAPIClient, query string) ([]commandTaskRun, error) {
	path := "/diagnostics/tasks"
	if query != "" {
		path += "?" + query
	}
	taskRuns := []commandTaskRun{}
	if _, errorValue := client.request("GET", path, nil, &taskRuns); errorValue != nil {
		return nil, errorValue
	}
	sort.Slice(taskRuns, func(leftIndex int, rightIndex int) bool {
		return taskRuns[leftIndex].UpdatedAt.After(taskRuns[rightIndex].UpdatedAt)
	})
	return taskRuns, nil
}

func resolveTaskRunID(arguments []string, client adminAPIClient) (string, error) {
	if errorValue := taskLogsUsageError(arguments); errorValue != nil {
		return "", errorValue
	}
	taskRuns, errorValue := fetchTaskRuns(client, "")
	if errorValue != nil {
		return "", errorValue
	}
	requestedID := firstTaskPositional(arguments)
	if requestedID == "" {
		return latestFailedTaskRunID(taskRuns)
	}
	return matchTaskRunID(taskRuns, requestedID)
}

func latestFailedTaskRunID(taskRuns []commandTaskRun) (string, error) {
	for _, taskRun := range taskRuns {
		if taskRun.Status == "failed" {
			return taskRun.TaskRunID, nil
		}
	}
	return "", errors.New("no failed task runs found")
}

func matchTaskRunID(taskRuns []commandTaskRun, requestedID string) (string, error) {
	prefixMatches := []string{}
	for _, taskRun := range taskRuns {
		if taskRun.TaskRunID == requestedID {
			return requestedID, nil
		}
		if strings.HasPrefix(taskRun.TaskRunID, requestedID) {
			prefixMatches = append(prefixMatches, taskRun.TaskRunID)
		}
	}
	if len(prefixMatches) == 1 {
		return prefixMatches[0], nil
	}
	if len(prefixMatches) > 1 {
		return "", fmt.Errorf("task run ID prefix %q is ambiguous (%d matches)", requestedID, len(prefixMatches))
	}
	return "", fmt.Errorf("task run not found: %s", requestedID)
}

func taskStatusFilterFromArguments(arguments []string) string {
	if hasCommandArgument(arguments, "--failed") {
		return "failed"
	}
	return strings.TrimSpace(commandArgumentValue(arguments, "--status", ""))
}

func filterTaskRunsByStatus(taskRuns []commandTaskRun, status string) []commandTaskRun {
	if status == "" {
		return taskRuns
	}
	filteredTaskRuns := []commandTaskRun{}
	for _, taskRun := range taskRuns {
		if taskRun.Status == status {
			filteredTaskRuns = append(filteredTaskRuns, taskRun)
		}
	}
	return filteredTaskRuns
}

func taskLimitFromArguments(arguments []string) int {
	limit, errorValue := strconv.Atoi(commandArgumentValue(arguments, "--limit", ""))
	if errorValue != nil || limit <= 0 {
		return defaultTaskListLimit
	}
	return limit
}

func limitTaskRuns(taskRuns []commandTaskRun, limit int) []commandTaskRun {
	if len(taskRuns) <= limit {
		return taskRuns
	}
	return taskRuns[:limit]
}

func firstTaskPositional(arguments []string) string {
	valueOptions := map[string]bool{
		"--board":    true,
		"--host":     true,
		"--limit":    true,
		"--node":     true,
		"--password": true,
		"--status":   true,
		"--user":     true,
	}
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		if strings.HasPrefix(argument, "--") {
			if valueOptions[argument] && index+1 < len(arguments) {
				index++
			}
			continue
		}
		return argument
	}
	return ""
}

func printTaskRunTable(taskRuns []commandTaskRun) {
	if len(taskRuns) == 0 {
		fmt.Fprintln(runCommandOutput, "No task runs.")
		return
	}
	fmt.Fprintf(runCommandOutput, "%-38s %-18s %-20s %s\n", "TASK RUN ID", "STATUS", "UPDATED", "SUMMARY")
	for _, taskRun := range taskRuns {
		fmt.Fprintf(runCommandOutput, "%-38s %-18s %-20s %s\n", taskRun.TaskRunID, taskRun.Status, formatTaskTime(taskRun.UpdatedAt), taskRunSummary(taskRun))
	}
}

func taskRunSummary(taskRun commandTaskRun) string {
	if strings.TrimSpace(taskRun.FailureReason) != "" {
		return truncateTaskText(taskRun.FailureReason, taskSummaryPreviewLimit)
	}
	return truncateTaskText(taskRun.Prompt, taskSummaryPreviewLimit)
}

func printTaskDetail(detail commandTaskDetail, showFullBodies bool) {
	printTaskRunHeader(detail.TaskRun)
	printTaskSteps(detail.TaskSteps)
	printTaskEvents(detail.TaskEvents, showFullBodies)
}

func printTaskRunHeader(taskRun commandTaskRun) {
	printTaskField("Task Run", taskRun.TaskRunID)
	printTaskField("Status", taskRun.Status)
	printTaskField("Profile", taskRun.CurrentAgentProfileName)
	printTaskField("Requester", taskRun.RequesterPersonID)
	printTaskField("Created", formatTaskTime(taskRun.CreatedAt))
	printTaskField("Updated", formatTaskTime(taskRun.UpdatedAt))
	printTaskField("Prompt", taskRun.Prompt)
	printTaskField("Result", taskRun.Result)
	printTaskField("Failure", taskRun.FailureReason)
}

func printTaskField(label string, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	fmt.Fprintf(runCommandOutput, "%-10s %s\n", label, value)
}

func printTaskSteps(taskSteps []commandTaskStep) {
	if len(taskSteps) == 0 {
		return
	}
	fmt.Fprintln(runCommandOutput)
	fmt.Fprintln(runCommandOutput, "Steps:")
	for _, taskStep := range taskSteps {
		fmt.Fprintf(runCommandOutput, "  [%s] %s\n", taskStep.Status, truncateTaskText(taskStep.Instruction, 100))
	}
}

func printTaskEvents(taskEvents []commandTaskEvent, showFullBodies bool) {
	if len(taskEvents) == 0 {
		return
	}
	sortedTaskEvents := append([]commandTaskEvent{}, taskEvents...)
	sort.Slice(sortedTaskEvents, func(leftIndex int, rightIndex int) bool {
		return sortedTaskEvents[leftIndex].CreatedAt.Before(sortedTaskEvents[rightIndex].CreatedAt)
	})
	fmt.Fprintln(runCommandOutput)
	fmt.Fprintln(runCommandOutput, "Events:")
	for _, taskEvent := range sortedTaskEvents {
		fmt.Fprintf(runCommandOutput, "%s  %-28s %s\n", formatTaskTime(taskEvent.CreatedAt), taskEvent.Name, taskEventBody(taskEvent, showFullBodies))
	}
}

func taskEventBody(taskEvent commandTaskEvent, showFullBody bool) string {
	if showFullBody || strings.Contains(taskEvent.Name, "failure") {
		return strings.TrimSpace(taskEvent.Body)
	}
	return truncateTaskText(taskEvent.Body, taskEventBodyPreviewLimit)
}

func truncateTaskText(text string, limit int) string {
	collapsedText := strings.Join(strings.Fields(text), " ")
	collapsedRunes := []rune(collapsedText)
	if len(collapsedRunes) <= limit {
		return collapsedText
	}
	return string(collapsedRunes[:limit]) + "…"
}

func formatTaskTime(timestamp time.Time) string {
	if timestamp.IsZero() {
		return ""
	}
	return timestamp.Local().Format("2006-01-02 15:04:05")
}

func printTaskJSON(value any) error {
	document, errorValue := json.MarshalIndent(value, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	fmt.Fprintln(runCommandOutput, string(document))
	return nil
}

func printTaskUsage() {
	fmt.Fprintln(runCommandOutput, "Usage: internkim run [list|logs] [options]")
	fmt.Fprintln(runCommandOutput, "Examples:")
	fmt.Fprintln(runCommandOutput, "  internkim run list")
	fmt.Fprintln(runCommandOutput, "  internkim run list --failed --limit 10")
	fmt.Fprintln(runCommandOutput, "  internkim run logs <taskRunID>")
	fmt.Fprintln(runCommandOutput, "  internkim run logs --last-failed")
	fmt.Fprintln(runCommandOutput, "Options:")
	fmt.Fprintln(runCommandOutput, "  --failed       Show only failed task runs")
	fmt.Fprintln(runCommandOutput, "  --status <s>   Filter list by status")
	fmt.Fprintln(runCommandOutput, "  --limit <n>    Maximum list rows (default 20)")
	fmt.Fprintln(runCommandOutput, "  --last-failed  Show logs for the most recent failed task run")
	fmt.Fprintln(runCommandOutput, "  --full         Print full event bodies")
	fmt.Fprintln(runCommandOutput, "  --json         Print raw JSON")
}
