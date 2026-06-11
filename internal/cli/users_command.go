package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type adminAPIClient interface {
	request(method string, path string, requestBody any, responseBody any) ([]byte, error)
}

type sshAdminAPIClient struct {
	connection *sshClient
}

type commandUserRecord struct {
	UserID                 string `json:"userID,omitempty"`
	Handle                 string `json:"handle,omitempty"`
	Name                   string `json:"name,omitempty"`
	Email                  string `json:"email,omitempty"`
	Role                   string `json:"role,omitempty"`
	MattermostUserID       string `json:"mattermostUserID,omitempty"`
	MattermostUsername     string `json:"mattermostUsername,omitempty"`
	Status                 string `json:"status,omitempty"`
	TemporaryPassword      string `json:"temporaryPassword,omitempty"`
	TemporaryPasswordEmail string `json:"temporaryPasswordEmail,omitempty"`
}

type commandUsersResponse struct {
	Users                  []string            `json:"users,omitempty"`
	Records                []commandUserRecord `json:"records,omitempty"`
	Revision               string              `json:"revision,omitempty"`
	TemporaryPassword      string              `json:"temporaryPassword,omitempty"`
	TemporaryPasswordEmail string              `json:"temporaryPasswordEmail,omitempty"`
}

var usersCommandOutput io.Writer = os.Stdout

func runUsersArguments(arguments []string) error {
	if hasCommandArgument(arguments, "--help") || hasCommandArgument(arguments, "-h") {
		printUsersUsage()
		return nil
	}
	command, commandArguments := splitUsersCommand(arguments)
	if errorValue := validateUsersArguments(command, commandArguments); errorValue != nil {
		return errorValue
	}
	client, errorValue := resolveAdminAPIClient(arguments)
	if errorValue != nil {
		return errorValue
	}
	return runUsersArgumentsWithClient(arguments, client)
}

func runInviteArguments(arguments []string) error {
	if hasCommandArgument(arguments, "--help") || hasCommandArgument(arguments, "-h") {
		printInviteUsage()
		return nil
	}
	if _, errorValue := commandUserRecordFromArguments(arguments, "member"); errorValue != nil {
		return errorValue
	}
	client, errorValue := resolveAdminAPIClient(arguments)
	if errorValue != nil {
		return errorValue
	}
	return addUserWithClient(arguments, client, "member")
}

func runUsersArgumentsWithClient(arguments []string, client adminAPIClient) error {
	command, commandArguments := splitUsersCommand(arguments)
	switch command {
	case "list":
		return listUsersWithClient(commandArguments, client)
	case "add", "invite":
		return addUserWithClient(commandArguments, client, "member")
	case "remove", "delete":
		return removeUserWithClient(commandArguments, client)
	case "promote":
		return changeUserRoleWithClient(commandArguments, client, "admin")
	case "demote":
		return changeUserRoleWithClient(commandArguments, client, "member")
	default:
		return fmt.Errorf("unknown users command %q", command)
	}
}

func splitUsersCommand(arguments []string) (string, []string) {
	if len(arguments) > 0 && !strings.HasPrefix(arguments[0], "-") {
		return arguments[0], arguments[1:]
	}
	return "list", arguments
}

func validateUsersArguments(command string, arguments []string) error {
	switch command {
	case "list":
		return nil
	case "add", "invite":
		_, errorValue := commandUserRecordFromArguments(arguments, "member")
		return errorValue
	case "remove", "delete":
		if userEmailFromArguments(arguments) == "" {
			return errors.New("usage: internkim users remove <email>")
		}
		return nil
	case "promote", "demote":
		if userEmailFromArguments(arguments) == "" {
			return errors.New("usage: internkim users promote|demote <email>")
		}
		return nil
	default:
		return fmt.Errorf("unknown users command %q", command)
	}
}

func resolveAdminAPIClient(arguments []string) (adminAPIClient, error) {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return nil, errorValue
	}
	configuration := loadConfig()
	sshpassBin := filepath.Join(repositoryRootPath, "bin", "sshpass")
	target := resolveCommandTarget(arguments)
	target = resolveLabHostForCommandTarget(target, repositoryRootPath)
	connection, _, errorValue := resolveDeviceSSHConnection(configuration, sshpassBin, target)
	if errorValue != nil {
		return nil, errorValue
	}
	if connection == nil {
		return nil, errors.New("device is not reachable; pass --host <ip> or run setup once")
	}
	return sshAdminAPIClient{connection: connection}, nil
}

func listUsersWithClient(arguments []string, client adminAPIClient) error {
	response := commandUsersResponse{}
	rawResponse, errorValue := client.request("GET", "/users", nil, &response)
	if errorValue != nil {
		return errorValue
	}
	if hasCommandArgument(arguments, "--json") {
		fmt.Fprintln(usersCommandOutput, string(rawResponse))
		return nil
	}
	printUsersResponse(response)
	return nil
}

func addUserWithClient(arguments []string, client adminAPIClient, defaultRole string) error {
	record, errorValue := commandUserRecordFromArguments(arguments, defaultRole)
	if errorValue != nil {
		return errorValue
	}
	response := commandUsersResponse{}
	rawResponse, errorValue := client.request("POST", "/users", record, &response)
	if errorValue != nil {
		return errorValue
	}
	if hasCommandArgument(arguments, "--json") {
		fmt.Fprintln(usersCommandOutput, string(rawResponse))
		return nil
	}
	createdRecord := response.recordForEmail(record.Email)
	if createdRecord.Email == "" {
		createdRecord = record
	}
	fmt.Fprintf(usersCommandOutput, "Added user: %s (%s)\n", createdRecord.Email, createdRecord.Role)
	if response.TemporaryPassword != "" {
		fmt.Fprintf(usersCommandOutput, "Temporary password: %s / %s\n", response.TemporaryPasswordEmail, response.TemporaryPassword)
	}
	return nil
}

func removeUserWithClient(arguments []string, client adminAPIClient) error {
	email := userEmailFromArguments(arguments)
	if email == "" {
		return errors.New("usage: internkim users remove <email>")
	}
	_, errorValue := client.request("DELETE", "/users/"+url.PathEscape(email), nil, nil)
	if errorValue != nil {
		return errorValue
	}
	fmt.Fprintf(usersCommandOutput, "Removed user: %s\n", email)
	return nil
}

func changeUserRoleWithClient(arguments []string, client adminAPIClient, role string) error {
	email := userEmailFromArguments(arguments)
	if email == "" {
		return errors.New("usage: internkim users promote|demote <email>")
	}
	response := commandUsersResponse{}
	if _, errorValue := client.request("GET", "/users", nil, &response); errorValue != nil {
		return errorValue
	}
	record := response.recordForEmail(email)
	if record.Email == "" {
		return fmt.Errorf("user not found: %s", email)
	}
	record.Role = role
	if _, errorValue := client.request("POST", "/users", record, &response); errorValue != nil {
		return errorValue
	}
	fmt.Fprintf(usersCommandOutput, "Updated user: %s (%s)\n", email, role)
	return nil
}

func commandUserRecordFromArguments(arguments []string, defaultRole string) (commandUserRecord, error) {
	email := userEmailFromArguments(arguments)
	if email == "" {
		return commandUserRecord{}, errors.New("usage: internkim users add <email> [--name <name>] [--handle <handle>] [--role admin|member]")
	}
	role := commandArgumentValue(arguments, "--role", defaultRole)
	if role == "" {
		role = "member"
	}
	role = normalizeCommandUserRole(role)
	if role == "" {
		return commandUserRecord{}, errors.New("role must be admin or member")
	}
	return commandUserRecord{
		Email:  email,
		Name:   strings.TrimSpace(commandArgumentValue(arguments, "--name", "")),
		Handle: strings.TrimSpace(commandArgumentValue(arguments, "--handle", "")),
		Role:   role,
	}, nil
}

func userEmailFromArguments(arguments []string) string {
	if email := strings.ToLower(strings.TrimSpace(commandArgumentValue(arguments, "--email", ""))); email != "" {
		return email
	}
	for _, argument := range usersCommandPositionals(arguments) {
		return strings.ToLower(strings.TrimSpace(argument))
	}
	return ""
}

func usersCommandPositionals(arguments []string) []string {
	positionals := []string{}
	valueOptions := map[string]bool{
		"--board":    true,
		"--email":    true,
		"--handle":   true,
		"--host":     true,
		"--name":     true,
		"--node":     true,
		"--password": true,
		"--role":     true,
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
		positionals = append(positionals, argument)
	}
	return positionals
}

func normalizeCommandUserRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "", "member", "user", "staff":
		return "member"
	case "admin", "administrator":
		return "admin"
	default:
		return ""
	}
}

func (response commandUsersResponse) recordForEmail(email string) commandUserRecord {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	for _, record := range response.Records {
		if strings.EqualFold(strings.TrimSpace(record.Email), normalizedEmail) {
			return record
		}
	}
	return commandUserRecord{}
}

func printUsersResponse(response commandUsersResponse) {
	records := append([]commandUserRecord(nil), response.Records...)
	sort.Slice(records, func(leftIndex int, rightIndex int) bool {
		if records[leftIndex].Role != records[rightIndex].Role {
			return records[leftIndex].Role == "admin"
		}
		return strings.ToLower(records[leftIndex].Email) < strings.ToLower(records[rightIndex].Email)
	})
	if len(records) == 0 {
		fmt.Fprintln(usersCommandOutput, "No users.")
		return
	}
	fmt.Fprintf(usersCommandOutput, "%-32s %-12s %-18s %s\n", "EMAIL", "ROLE", "HANDLE", "NAME")
	for _, record := range records {
		fmt.Fprintf(usersCommandOutput, "%-32s %-12s %-18s %s\n", record.Email, record.Role, record.Handle, record.Name)
	}
}

func (client sshAdminAPIClient) request(method string, path string, requestBody any, responseBody any) ([]byte, error) {
	command, errorValue := adminAPICurlCommand(method, path, requestBody)
	if errorValue != nil {
		return nil, errorValue
	}
	output, errorValue := client.connection.runResult(command)
	if errorValue != nil {
		return nil, errorValue
	}
	body, statusCode, errorValue := splitCurlResponse(output)
	if errorValue != nil {
		return nil, errorValue
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, fmt.Errorf("admin API %s %s returned %d: %s", method, path, statusCode, strings.TrimSpace(string(body)))
	}
	if responseBody != nil && len(strings.TrimSpace(string(body))) > 0 {
		if errorValue := json.Unmarshal(body, responseBody); errorValue != nil {
			return nil, errorValue
		}
	}
	return body, nil
}

func adminAPICurlCommand(method string, path string, requestBody any) (string, error) {
	requestURL := "http://127.0.0.1:18080/admin/api" + path
	parts := []string{
		"curl",
		"--silent",
		"--show-error",
		"--write-out",
		quoteShellValue("\n%{http_code}"),
		"--request",
		quoteShellValue(method),
		"--header",
		quoteShellValue("Content-Type: application/json"),
	}
	if requestBody != nil {
		body, errorValue := json.Marshal(requestBody)
		if errorValue != nil {
			return "", errorValue
		}
		parts = append(parts, "--data", quoteShellValue(string(body)))
	}
	parts = append(parts, quoteShellValue(requestURL))
	return strings.Join(parts, " "), nil
}

func splitCurlResponse(output string) ([]byte, int, error) {
	trimmedOutput := strings.TrimRight(output, "\r\n")
	index := strings.LastIndex(trimmedOutput, "\n")
	if index < 0 {
		return nil, 0, errors.New("admin API response did not include a status code")
	}
	statusCode, errorValue := strconv.Atoi(strings.TrimSpace(trimmedOutput[index+1:]))
	if errorValue != nil {
		return nil, 0, errorValue
	}
	return []byte(trimmedOutput[:index]), statusCode, nil
}

func printUsersUsage() {
	fmt.Fprintln(usersCommandOutput, "Usage: internkim users [list|add|remove|promote|demote] [options]")
	fmt.Fprintln(usersCommandOutput, "Examples:")
	fmt.Fprintln(usersCommandOutput, "  internkim users list")
	fmt.Fprintln(usersCommandOutput, "  internkim users add person@example.com --name \"Person\" --role member")
	fmt.Fprintln(usersCommandOutput, "  internkim users promote person@example.com")
	fmt.Fprintln(usersCommandOutput, "  internkim users remove person@example.com")
}

func printInviteUsage() {
	fmt.Fprintln(usersCommandOutput, "Usage: internkim invite <email> [--name <name>] [--handle <handle>] [--role admin|member]")
}
