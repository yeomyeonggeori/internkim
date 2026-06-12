package admind

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

type serviceLogsResponse struct {
	Service   string   `json:"service"`
	TaskRunID string   `json:"taskRunID,omitempty"`
	Count     int      `json:"count"`
	Lines     []string `json:"lines"`
}

var allowedTaskRunIDPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

var allowedServiceNames = []string{"blueclaw", "admind", "capabilityd", "mattermost"}

func isAllowedServiceName(serviceName string) bool {
	for _, allowed := range allowedServiceNames {
		if serviceName == allowed {
			return true
		}
	}
	return false
}

func parseLogsLimit(rawLimit string) int {
	defaultLimit := 200
	maximumLimit := 1000

	if rawLimit == "" {
		return defaultLimit
	}
	parsed, parseError := strconv.Atoi(rawLimit)
	if parseError != nil {
		return defaultLimit
	}
	if parsed < 1 {
		return 1
	}
	if parsed > maximumLimit {
		return maximumLimit
	}
	return parsed
}

func (service *Service) writeServiceLogs(responseWriter http.ResponseWriter, request *http.Request) {
	serviceName := request.URL.Query().Get("service")
	taskRunID := request.URL.Query().Get("taskRunID")
	limit := parseLogsLimit(request.URL.Query().Get("limit"))

	if !isAllowedServiceName(serviceName) {
		http.Error(responseWriter, "service must be one of blueclaw, admind, capabilityd, mattermost", http.StatusBadRequest)
		return
	}
	if taskRunID != "" && !allowedTaskRunIDPattern.MatchString(taskRunID) {
		http.Error(responseWriter, "taskRunID contains illegal characters", http.StatusBadRequest)
		return
	}

	rawOutput, commandError := service.fetchServiceLogOutput(request, serviceName, taskRunID, limit)
	if commandError != nil {
		http.Error(responseWriter, commandError.Error(), http.StatusBadGateway)
		return
	}

	lines := filterNonEmptyLines(rawOutput)
	if serviceName != "blueclaw" && taskRunID != "" {
		lines = filterLinesByTaskRunID(lines, taskRunID)
	}

	service.writeJSON(responseWriter, serviceLogsResponse{
		Service:   serviceName,
		TaskRunID: taskRunID,
		Count:     len(lines),
		Lines:     lines,
	})
}

func (service *Service) fetchServiceLogOutput(request *http.Request, serviceName string, taskRunID string, limit int) (string, error) {
	switch serviceName {
	case "blueclaw":
		return service.fetchBlueclawLogs(request, taskRunID, limit)
	case "admind":
		return service.fetchJournalctlLogs(request, "internkim-admind", limit)
	case "capabilityd":
		return service.fetchJournalctlLogs(request, "internkim-capabilityd", limit)
	case "mattermost":
		return service.fetchJournalctlLogs(request, "mattermost", limit)
	}
	return "", fmt.Errorf("unhandled service: %s", serviceName)
}

func (service *Service) fetchBlueclawLogs(request *http.Request, taskRunID string, limit int) (string, error) {
	var shellCommand string
	if taskRunID == "" {
		shellCommand = fmt.Sprintf("cat /var/log/blueclaw-supervisor/*.jsonl 2>/dev/null | tail -n %d", limit)
	} else {
		shellCommand = fmt.Sprintf("grep -hF '%s' /var/log/blueclaw-supervisor/*.jsonl 2>/dev/null | tail -n %d", taskRunID, limit)
	}
	output, commandError := service.runCommand(request.Context(), "sh", "-c", shellCommand)
	if commandError != nil {
		return "", commandError
	}
	return string(output), nil
}

func (service *Service) fetchJournalctlLogs(request *http.Request, unitName string, limit int) (string, error) {
	output, commandError := service.runCommand(
		request.Context(),
		"journalctl",
		"-u", unitName,
		"-n", strconv.Itoa(limit),
		"--no-pager",
		"--output", "short-iso",
	)
	if commandError != nil {
		return "", commandError
	}
	return string(output), nil
}

func filterNonEmptyLines(rawOutput string) []string {
	allLines := strings.Split(rawOutput, "\n")
	filtered := make([]string, 0, len(allLines))
	for _, line := range allLines {
		if line != "" {
			filtered = append(filtered, line)
		}
	}
	return filtered
}

func filterLinesByTaskRunID(lines []string, taskRunID string) []string {
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.Contains(line, taskRunID) {
			filtered = append(filtered, line)
		}
	}
	return filtered
}
