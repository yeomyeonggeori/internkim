package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"
)

type mattermostScenarioRemote interface {
	run(context.Context, string) (string, error)
}

type mattermostScenarioSSHRemote struct {
	client *sshClient
}

type mattermostScenarioLocalFleetRemote struct {
	executablePath    string
	configurationPath string
}

func (remote mattermostScenarioSSHRemote) run(contextValue context.Context, script string) (string, error) {
	target := fmt.Sprintf("%s@%s", remote.client.user, remote.client.host)
	commandName := "ssh"
	commandArguments := remote.client.sshArgs(target, remote.client.privilegedCommand(script))
	if remote.client.pass != "" {
		commandName = remote.client.sshpassBin
		commandArguments = append([]string{"-p", remote.client.pass, "ssh"}, commandArguments...)
	}
	output, errorValue := exec.CommandContext(contextValue, commandName, commandArguments...).CombinedOutput()
	if errorValue != nil {
		return "", fmt.Errorf("run Local Fleet command: %w: %s", errorValue, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func (remote mattermostScenarioLocalFleetRemote) run(contextValue context.Context, script string) (string, error) {
	command := exec.CommandContext(contextValue, remote.executablePath, remote.arguments(script)...)
	output, errorValue := command.CombinedOutput()
	if errorValue != nil {
		return "", fmt.Errorf("run Local Fleet command: %w: %s", errorValue, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func (remote mattermostScenarioLocalFleetRemote) arguments(script string) []string {
	privilegedScript := "sudo -p '' bash -lc " + quoteShellValue(script)
	return []string{"lab", "vm-ssh", "--config", remote.configurationPath, privilegedScript}
}

type mattermostScenarioAdmin struct {
	remote mattermostScenarioRemote
}

type mattermostScenarioAdminAPI interface {
	readSecret(context.Context, string) (string, error)
	invitePerson(context.Context, string, string, string) error
	listTasks(context.Context, string) ([]mattermostScenarioTaskSummary, error)
	taskDetail(context.Context, string) (mattermostScenarioTaskDetail, error)
	workspaceFiles(context.Context, mattermostScenarioStep) ([]mattermostScenarioWorkspaceResult, error)
	cleanup(context.Context, mattermostScenarioResult, string) error
}

type mattermostScenarioTaskSummary struct {
	TaskRunID            string `json:"taskRunID"`
	OriginConversationID string `json:"originConversationID"`
	Status               string `json:"status"`
	UpdatedAt            string `json:"updatedAt"`
}

type mattermostScenarioSite struct {
	SiteID         string                         `json:"siteID"`
	ConversationID string                         `json:"conversationID"`
	Owner          string                         `json:"owner"`
	OwnerIdentity  mattermostScenarioSiteIdentity `json:"ownerIdentity"`
}

type mattermostScenarioSiteIdentity struct {
	PersonID       string `json:"personID,omitempty"`
	Platform       string `json:"platform,omitempty"`
	PlatformUserID string `json:"platformUserID,omitempty"`
	DisplayName    string `json:"displayName,omitempty"`
}

type mattermostScenarioWorkspaceEntry struct {
	Name        string `json:"name"`
	IsDirectory bool   `json:"isDirectory"`
}

type mattermostScenarioCreatedResourceIDs struct {
	TaskIDs          []string
	CalendarEventIDs []string
}

func (admin mattermostScenarioAdmin) readSecret(contextValue context.Context, filePath string) (string, error) {
	value, errorValue := admin.remote.run(contextValue, "cat "+quoteShellValue(filePath))
	if errorValue != nil {
		return "", errorValue
	}
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("Local Fleet secret %s is empty", filePath)
	}
	return strings.TrimSpace(value), nil
}

func (admin mattermostScenarioAdmin) invitePerson(contextValue context.Context, personID string, email string, displayName string) error {
	body := map[string]string{"personID": personID, "email": email, "displayName": displayName}
	return admin.requestJSON(contextValue, "POST", "/admin/api/people/invite", body, nil)
}

func (admin mattermostScenarioAdmin) listTasks(contextValue context.Context, conversationID string) ([]mattermostScenarioTaskSummary, error) {
	var tasks []mattermostScenarioTaskSummary
	if errorValue := admin.requestJSON(contextValue, "GET", "/admin/api/task", nil, &tasks); errorValue != nil {
		return nil, errorValue
	}
	filteredTasks := make([]mattermostScenarioTaskSummary, 0, len(tasks))
	for _, taskSummary := range tasks {
		if taskSummary.OriginConversationID == conversationID {
			filteredTasks = append(filteredTasks, taskSummary)
		}
	}
	sort.Slice(filteredTasks, func(firstIndex int, secondIndex int) bool {
		return filteredTasks[firstIndex].UpdatedAt < filteredTasks[secondIndex].UpdatedAt
	})
	return filteredTasks, nil
}

func (admin mattermostScenarioAdmin) taskDetail(contextValue context.Context, taskRunID string) (mattermostScenarioTaskDetail, error) {
	var detail mattermostScenarioTaskDetail
	endpoint := "/admin/api/task/detail?taskRunID=" + url.QueryEscape(taskRunID)
	if errorValue := admin.requestJSON(contextValue, "GET", endpoint, nil, &detail); errorValue != nil {
		return mattermostScenarioTaskDetail{}, errorValue
	}
	return detail, nil
}

func (admin mattermostScenarioAdmin) workspaceFiles(contextValue context.Context, step mattermostScenarioStep) ([]mattermostScenarioWorkspaceResult, error) {
	files := []mattermostScenarioWorkspaceResult{}
	for _, expectation := range step.ExpectedWorkspaceFiles {
		matches, errorValue := admin.workspaceMatches(contextValue, expectation.PathGlob)
		if errorValue != nil {
			return nil, errorValue
		}
		for _, filePath := range matches {
			content, errorValue := admin.workspaceDownload(contextValue, filePath)
			if errorValue != nil {
				return nil, errorValue
			}
			files = append(files, mattermostScenarioWorkspaceResult{Path: filePath, Content: content})
		}
	}
	for _, pathGlob := range step.ForbiddenWorkspaceFiles {
		matches, errorValue := admin.workspaceMatches(contextValue, pathGlob)
		if errorValue != nil {
			return nil, errorValue
		}
		for _, filePath := range matches {
			files = append(files, mattermostScenarioWorkspaceResult{Path: filePath})
		}
	}
	return files, nil
}

func (admin mattermostScenarioAdmin) workspaceMatches(contextValue context.Context, pathGlob string) ([]string, error) {
	segments := strings.Split(strings.Trim(strings.TrimSpace(pathGlob), "/"), "/")
	if len(segments) == 0 || segments[0] == "" {
		return nil, fmt.Errorf("workspace path glob is empty")
	}
	return admin.matchWorkspaceSegments(contextValue, "/workspace", segments)
}

func (admin mattermostScenarioAdmin) matchWorkspaceSegments(contextValue context.Context, directoryPath string, segments []string) ([]string, error) {
	entries, errorValue := admin.workspaceEntries(contextValue, directoryPath)
	if errorValue != nil {
		return nil, errorValue
	}
	matches := []string{}
	for _, entry := range entries {
		isMatch, matchError := path.Match(segments[0], entry.Name)
		if matchError != nil {
			return nil, fmt.Errorf("invalid workspace path glob %q: %w", segments[0], matchError)
		}
		if !isMatch {
			continue
		}
		matchedPath := path.Join(directoryPath, entry.Name)
		if len(segments) == 1 {
			matches = append(matches, matchedPath)
			continue
		}
		if !entry.IsDirectory {
			continue
		}
		nestedMatches, nestedError := admin.matchWorkspaceSegments(contextValue, matchedPath, segments[1:])
		if nestedError != nil {
			return nil, nestedError
		}
		matches = append(matches, nestedMatches...)
	}
	sort.Strings(matches)
	return matches, nil
}

func (admin mattermostScenarioAdmin) workspaceEntries(contextValue context.Context, directoryPath string) ([]mattermostScenarioWorkspaceEntry, error) {
	var response struct {
		Entries []mattermostScenarioWorkspaceEntry `json:"entries"`
	}
	endpoint := "/admin/api/workspace/list?path=" + url.QueryEscape(directoryPath)
	if errorValue := admin.requestJSON(contextValue, "GET", endpoint, nil, &response); errorValue != nil {
		return nil, errorValue
	}
	return response.Entries, nil
}

func (admin mattermostScenarioAdmin) workspaceDownload(contextValue context.Context, filePath string) (string, error) {
	return admin.request(contextValue, "GET", "/admin/api/workspace/download?path="+url.QueryEscape(filePath), nil)
}

func (admin mattermostScenarioAdmin) cleanup(contextValue context.Context, result mattermostScenarioResult, email string) error {
	cleanupErrors := []error{}
	resourceCleanupError := admin.deleteCreatedResources(contextValue, result, email)
	if resourceCleanupError != nil {
		cleanupErrors = append(cleanupErrors, resourceCleanupError)
	}
	conversationID := firstNonEmptyString(result.ConversationID, result.ChannelID)
	if errorValue := admin.deleteConversationTasks(contextValue, conversationID); errorValue != nil {
		cleanupErrors = append(cleanupErrors, errorValue)
	}
	if mattermostScenarioHasSiteEvidence(result) {
		if errorValue := admin.deleteConversationSites(contextValue, conversationID); errorValue != nil {
			cleanupErrors = append(cleanupErrors, errorValue)
		}
	}
	if normalizedEmail := strings.TrimSpace(email); normalizedEmail != "" && resourceCleanupError == nil {
		if _, errorValue := admin.request(contextValue, "DELETE", "/admin/api/people?email="+url.QueryEscape(normalizedEmail), nil); errorValue != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("delete Mattermost scenario person: %w", errorValue))
		}
	}
	return errors.Join(cleanupErrors...)
}

func (admin mattermostScenarioAdmin) deleteCreatedResources(contextValue context.Context, result mattermostScenarioResult, email string) error {
	resourceIDs := collectMattermostScenarioCreatedResourceIDs(result)
	if len(resourceIDs.TaskIDs) == 0 && len(resourceIDs.CalendarEventIDs) == 0 {
		return nil
	}
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		return errors.New("delete Mattermost scenario domain resources: requester email is empty")
	}
	cleanupErrors := []error{}
	for _, taskID := range resourceIDs.TaskIDs {
		if errorValue := admin.deleteCreatedResource(contextValue, "/flow/api/tasks/"+url.PathEscape(taskID), "X-InternKim-Requester-Email", normalizedEmail); errorValue != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("delete Mattermost scenario task %s: %w", taskID, errorValue))
		}
	}
	for _, eventID := range resourceIDs.CalendarEventIDs {
		if errorValue := admin.deleteCreatedResource(contextValue, "/calendar/api/events/"+url.PathEscape(eventID), "CF-Access-Authenticated-User-Email", normalizedEmail); errorValue != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("delete Mattermost scenario calendar event %s: %w", eventID, errorValue))
		}
	}
	return errors.Join(cleanupErrors...)
}

func collectMattermostScenarioCreatedResourceIDs(result mattermostScenarioResult) mattermostScenarioCreatedResourceIDs {
	resourceIDs := mattermostScenarioCreatedResourceIDs{}
	seenTaskIDs := map[string]bool{}
	seenCalendarEventIDs := map[string]bool{}
	for _, step := range result.Steps {
		for _, event := range step.TaskEvents {
			toolName, resourceID := mattermostScenarioCreatedResourceFromEvent(event)
			switch {
			case toolName == "task.add" && !seenTaskIDs[resourceID]:
				seenTaskIDs[resourceID] = true
				resourceIDs.TaskIDs = append(resourceIDs.TaskIDs, resourceID)
			case toolName == "calendar.add" && !seenCalendarEventIDs[resourceID]:
				seenCalendarEventIDs[resourceID] = true
				resourceIDs.CalendarEventIDs = append(resourceIDs.CalendarEventIDs, resourceID)
			}
		}
	}
	return resourceIDs
}

func mattermostScenarioCreatedResourceFromEvent(event mattermostScenarioTaskEvent) (string, string) {
	expectedToolName, isCreatedResourceEvent := mattermostScenarioCreatedResourceToolName(event.Name)
	if !isCreatedResourceEvent {
		return "", ""
	}
	var result struct {
		Tool   string `json:"tool"`
		Output struct {
			Content string          `json:"content"`
			Data    json.RawMessage `json:"data"`
		} `json:"output"`
	}
	if json.Unmarshal([]byte(event.Body), &result) != nil {
		return "", ""
	}
	if expectedToolName != "" && result.Tool != expectedToolName {
		return "", ""
	}
	identifierKeys := []string{"id"}
	if result.Tool == "task.add" {
		identifierKeys = []string{"taskID", "id"}
	} else if result.Tool == "calendar.add" {
		identifierKeys = []string{"eventID", "id"}
	} else {
		return "", ""
	}
	resourceID := mattermostScenarioCreatedResourceID(result.Output.Data, identifierKeys)
	if resourceID == "" {
		resourceID = mattermostScenarioCreatedResourceID(json.RawMessage(result.Output.Content), identifierKeys)
	}
	if resourceID == "" {
		return "", ""
	}
	return result.Tool, resourceID
}

func mattermostScenarioCreatedResourceToolName(eventName string) (string, bool) {
	switch eventName {
	case "tool.capability.invoke.result":
		return "", true
	case "tool.task.add.result":
		return "task.add", true
	case "tool.calendar.add.result":
		return "calendar.add", true
	default:
		return "", false
	}
}

func mattermostScenarioCreatedResourceID(document json.RawMessage, identifierKeys []string) string {
	var values map[string]json.RawMessage
	if json.Unmarshal(document, &values) != nil {
		return ""
	}
	for _, identifierKey := range identifierKeys {
		var identifier string
		if json.Unmarshal(values[identifierKey], &identifier) == nil && strings.TrimSpace(identifier) != "" {
			return strings.TrimSpace(identifier)
		}
	}
	return ""
}

func (admin mattermostScenarioAdmin) deleteCreatedResource(contextValue context.Context, endpoint string, headerName string, email string) error {
	arguments := []string{
		"curl", "--silent", "--show-error", "-X", "DELETE",
		"-H", headerName + ": " + email,
		"--output", "/dev/null", "--write-out", "%{http_code}",
	}
	output, errorValue := admin.remote.run(contextValue, shellJoin(arguments)+" "+quoteShellValue("http://127.0.0.1:18080"+endpoint))
	if errorValue != nil {
		return errorValue
	}
	statusCode, conversionError := strconv.Atoi(strings.TrimSpace(output))
	if conversionError != nil {
		return fmt.Errorf("domain cleanup returned invalid HTTP status %q", output)
	}
	if statusCode == 404 || statusCode >= 200 && statusCode < 300 {
		return nil
	}
	return fmt.Errorf("domain cleanup returned HTTP %d", statusCode)
}

func mattermostScenarioHasSiteEvidence(result mattermostScenarioResult) bool {
	for _, step := range result.Steps {
		if step.PublicURL != "" {
			return true
		}
		for _, event := range step.TaskEvents {
			if strings.Contains(event.Name, "site.") || strings.Contains(event.Body, "site.") {
				return true
			}
		}
	}
	return false
}

func (admin mattermostScenarioAdmin) deleteConversationTasks(contextValue context.Context, conversationID string) error {
	if strings.TrimSpace(conversationID) == "" {
		return nil
	}
	tasks, errorValue := admin.listTasks(contextValue, conversationID)
	if errorValue != nil {
		return fmt.Errorf("list Mattermost scenario tasks: %w", errorValue)
	}
	taskRunIDs := mattermostScenarioTaskRunIDs(tasks)
	if len(taskRunIDs) == 0 {
		return nil
	}
	cleanupErrors := []error{}
	cancelBody := map[string]any{"taskRunIDs": taskRunIDs, "reason": "expensive Mattermost scenario cleanup"}
	if errorValue := admin.requestJSON(contextValue, "POST", "/admin/api/task/cancel", cancelBody, nil); errorValue != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("cancel Mattermost scenario tasks: %w", errorValue))
	}
	for _, taskRunID := range taskRunIDs {
		deleteBody := map[string]any{"taskRunID": taskRunID, "viewerIsAdmin": true}
		if errorValue := admin.requestJSON(contextValue, "POST", "/admin/api/task/delete", deleteBody, nil); errorValue != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("delete Mattermost scenario task %s: %w", taskRunID, errorValue))
		}
	}
	remainingTasks, verificationError := admin.listTasks(contextValue, conversationID)
	if verificationError != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("verify Mattermost scenario task cleanup: %w", verificationError))
	} else if remainingTaskRunIDs := mattermostScenarioTaskRunIDs(remainingTasks); len(remainingTaskRunIDs) > 0 {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("Mattermost scenario tasks remain after cleanup: %s", strings.Join(remainingTaskRunIDs, ", ")))
	}
	return errors.Join(cleanupErrors...)
}

func mattermostScenarioTaskRunIDs(tasks []mattermostScenarioTaskSummary) []string {
	taskRunIDs := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if taskRunID := strings.TrimSpace(task.TaskRunID); taskRunID != "" {
			taskRunIDs = append(taskRunIDs, taskRunID)
		}
	}
	return taskRunIDs
}

func (admin mattermostScenarioAdmin) deleteConversationSites(contextValue context.Context, conversationID string) error {
	if strings.TrimSpace(conversationID) == "" {
		return nil
	}
	sites, errorValue := admin.conversationSites(contextValue, conversationID)
	if errorValue != nil {
		return fmt.Errorf("list Mattermost scenario sites: %w", errorValue)
	}
	cleanupErrors := []error{}
	for _, site := range sites {
		if strings.TrimSpace(site.SiteID) == "" {
			cleanupErrors = append(cleanupErrors, errors.New("Mattermost scenario site has no siteID"))
			continue
		}
		endpoint := "/admin/api/sites/" + url.PathEscape(site.SiteID)
		body := map[string]any{
			"confirm":       "DELETE",
			"userConfirmed": true,
			"requestedBy":   site.Owner,
			"requester":     site.OwnerIdentity,
		}
		if errorValue := admin.requestAdmindJSON(contextValue, "DELETE", endpoint, body, nil); errorValue != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("delete Mattermost scenario site %s: %w", site.SiteID, errorValue))
		}
	}
	remainingSites, verificationError := admin.conversationSites(contextValue, conversationID)
	if verificationError != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("verify Mattermost scenario site cleanup: %w", verificationError))
	} else if remainingSiteIDs := mattermostScenarioSiteIDs(remainingSites); len(remainingSiteIDs) > 0 {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("Mattermost scenario sites remain after cleanup: %s", strings.Join(remainingSiteIDs, ", ")))
	}
	return errors.Join(cleanupErrors...)
}

func (admin mattermostScenarioAdmin) conversationSites(contextValue context.Context, conversationID string) ([]mattermostScenarioSite, error) {
	var response struct {
		Sites []mattermostScenarioSite `json:"sites"`
	}
	if errorValue := admin.requestAdmindJSON(contextValue, "GET", "/admin/api/sites", nil, &response); errorValue != nil {
		return nil, errorValue
	}
	sites := make([]mattermostScenarioSite, 0, len(response.Sites))
	for _, site := range response.Sites {
		if site.ConversationID == conversationID {
			sites = append(sites, site)
		}
	}
	return sites, nil
}

func mattermostScenarioSiteIDs(sites []mattermostScenarioSite) []string {
	siteIDs := make([]string, 0, len(sites))
	for _, site := range sites {
		if siteID := strings.TrimSpace(site.SiteID); siteID != "" {
			siteIDs = append(siteIDs, siteID)
		}
	}
	return siteIDs
}

func (admin mattermostScenarioAdmin) requestJSON(contextValue context.Context, method string, endpoint string, body any, output any) error {
	return admin.requestJSONAt(contextValue, "http://127.0.0.1:8080", method, endpoint, body, output)
}

func (admin mattermostScenarioAdmin) requestAdmindJSON(contextValue context.Context, method string, endpoint string, body any, output any) error {
	return admin.requestJSONAt(contextValue, "http://127.0.0.1:18080", method, endpoint, body, output)
}

func (admin mattermostScenarioAdmin) requestJSONAt(contextValue context.Context, baseURL string, method string, endpoint string, body any, output any) error {
	document, errorValue := admin.requestAt(contextValue, baseURL, method, endpoint, body)
	if errorValue != nil {
		return errorValue
	}
	if output == nil {
		return nil
	}
	if errorValue := json.Unmarshal([]byte(document), output); errorValue != nil {
		return fmt.Errorf("parse Blueclaw admin response for %s: %w", endpoint, errorValue)
	}
	return nil
}

func (admin mattermostScenarioAdmin) request(contextValue context.Context, method string, endpoint string, body any) (string, error) {
	return admin.requestAt(contextValue, "http://127.0.0.1:8080", method, endpoint, body)
}

func (admin mattermostScenarioAdmin) requestAt(contextValue context.Context, baseURL string, method string, endpoint string, body any) (string, error) {
	requestURL := baseURL + endpoint
	arguments := []string{"curl", "--silent", "--show-error", "--fail", "-X", method}
	if body != nil {
		document, errorValue := json.Marshal(body)
		if errorValue != nil {
			return "", errorValue
		}
		encodedBody := base64.StdEncoding.EncodeToString(document)
		arguments = append(arguments, "-H", "Content-Type: application/json", "--data-binary", "@-")
		script := "printf %s " + quoteShellValue(encodedBody) + " | base64 -d | " + shellJoin(arguments) + " " + quoteShellValue(requestURL)
		return admin.remote.run(contextValue, script)
	}
	return admin.remote.run(contextValue, shellJoin(arguments)+" "+quoteShellValue(requestURL))
}

func shellJoin(arguments []string) string {
	quotedArguments := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		quotedArguments = append(quotedArguments, quoteShellValue(argument))
	}
	return strings.Join(quotedArguments, " ")
}
