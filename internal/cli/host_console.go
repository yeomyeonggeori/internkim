package cli

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed host_console.html
var hostConsoleAssets embed.FS

const defaultHostConsoleListenAddress = "127.0.0.1:9090"

var hostConsoleTeamIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}$`)

type hostConsoleOptions struct {
	ListenAddress      string
	VirtualMachineName string
}

type hostConsoleTeamRequest struct {
	TeamID         string                     `json:"teamID"`
	DisplayName    string                     `json:"displayName"`
	Members        []hostConsoleMemberRequest `json:"members"`
	ExtraArguments []string                   `json:"extraArguments"`
}

type hostConsoleMemberRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type hostConsoleJob struct {
	JobID   string          `json:"jobID"`
	Status  string          `json:"status"`
	Log     string          `json:"log"`
	Summary json.RawMessage `json:"summary,omitempty"`
}

type hostConsoleJobStore struct {
	mutex sync.Mutex
	jobs  map[string]hostConsoleJob
}

type hostConsoleServer struct {
	executor           hostCommandExecutor
	virtualMachineName string
	jobs               *hostConsoleJobStore
}

type hostConsoleJobLogWriter struct {
	jobs  *hostConsoleJobStore
	jobID string
}

func executeHostConsole(arguments []string, executor hostCommandExecutor, output io.Writer) error {
	options, errorValue := parseHostConsoleOptions(arguments)
	if errorValue != nil {
		return errorValue
	}
	console := newHostConsoleServer(executor, options.VirtualMachineName)
	fmt.Fprintf(output, "host console listening at http://%s/ (vm: %s)\n", options.ListenAddress, options.VirtualMachineName)
	server := &http.Server{
		Addr:    options.ListenAddress,
		Handler: console.handler(),
	}
	return server.ListenAndServe()
}

func parseHostConsoleOptions(arguments []string) (hostConsoleOptions, error) {
	flags := flag.NewFlagSet("host console", flag.ContinueOnError)
	listenAddress := flags.String("listen", defaultHostConsoleListenAddress, "HTTP listen address")
	virtualMachineName := flags.String("vm", defaultHostVirtualMachineName, "container VM name")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return hostConsoleOptions{}, errorValue
	}
	options := hostConsoleOptions{
		ListenAddress:      strings.TrimSpace(*listenAddress),
		VirtualMachineName: strings.TrimSpace(*virtualMachineName),
	}
	if options.ListenAddress == "" {
		return hostConsoleOptions{}, errors.New("--listen is required")
	}
	if options.VirtualMachineName == "" {
		return hostConsoleOptions{}, errors.New("--vm is required")
	}
	return options, nil
}

func newHostConsoleServer(executor hostCommandExecutor, virtualMachineName string) *hostConsoleServer {
	return &hostConsoleServer{
		executor:           executor,
		virtualMachineName: virtualMachineName,
		jobs:               &hostConsoleJobStore{jobs: map[string]hostConsoleJob{}},
	}
}

func (console *hostConsoleServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", console.serveIndex)
	mux.HandleFunc("/api/tenants", console.serveTenants)
	mux.HandleFunc("/api/teams", console.serveTeams)
	mux.HandleFunc("/api/jobs/", console.serveJob)
	return mux
}

func (console *hostConsoleServer) serveIndex(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		http.NotFound(responseWriter, request)
		return
	}
	if request.Method != http.MethodGet {
		writeHostConsoleError(responseWriter, http.StatusMethodNotAllowed, "GET만 지원합니다")
		return
	}
	content, errorValue := hostConsoleAssets.ReadFile("host_console.html")
	if errorValue != nil {
		writeHostConsoleError(responseWriter, http.StatusInternalServerError, errorValue.Error())
		return
	}
	responseWriter.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = responseWriter.Write(content)
}

func (console *hostConsoleServer) serveTenants(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeHostConsoleError(responseWriter, http.StatusMethodNotAllowed, "GET만 지원합니다")
		return
	}
	tenants, errorValue := listHostTenantSummaries(console.executor, console.virtualMachineName)
	if errorValue != nil {
		writeHostConsoleError(responseWriter, http.StatusInternalServerError, errorValue.Error())
		return
	}
	writeHostConsoleJSON(responseWriter, http.StatusOK, tenants)
}

func (console *hostConsoleServer) serveTeams(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeHostConsoleError(responseWriter, http.StatusMethodNotAllowed, "POST만 지원합니다")
		return
	}
	teamRequest, errorValue := readHostConsoleTeamRequest(request.Body)
	if errorValue != nil {
		writeHostConsoleError(responseWriter, http.StatusBadRequest, errorValue.Error())
		return
	}
	jobID, errorValue := console.jobs.createJob()
	if errorValue != nil {
		writeHostConsoleError(responseWriter, http.StatusConflict, errorValue.Error())
		return
	}
	go console.runTeamJob(jobID, teamRequest)
	writeHostConsoleJSON(responseWriter, http.StatusAccepted, map[string]string{"jobID": jobID})
}

func (console *hostConsoleServer) serveJob(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeHostConsoleError(responseWriter, http.StatusMethodNotAllowed, "GET만 지원합니다")
		return
	}
	jobID := strings.TrimPrefix(request.URL.Path, "/api/jobs/")
	if jobID == "" || strings.Contains(jobID, "/") {
		writeHostConsoleError(responseWriter, http.StatusNotFound, "작업을 찾을 수 없습니다")
		return
	}
	job, ok := console.jobs.job(jobID)
	if !ok {
		writeHostConsoleError(responseWriter, http.StatusNotFound, "작업을 찾을 수 없습니다")
		return
	}
	writeHostConsoleJSON(responseWriter, http.StatusOK, job)
}

func (console *hostConsoleServer) runTeamJob(jobID string, teamRequest hostConsoleTeamRequest) {
	writer := hostConsoleJobLogWriter{jobs: console.jobs, jobID: jobID}
	options := hostAddTeamOptions{
		TeamID:             teamRequest.TeamID,
		VirtualMachineName: console.virtualMachineName,
		RemoteArguments:    hostConsoleRemoteArguments(teamRequest),
	}
	exitCode, errorValue := runHostAddTeam(options, console.executor, writer, writer)
	logText := console.jobs.log(jobID)
	if errorValue != nil {
		console.jobs.finishJob(jobID, "failed", nil, errorValue.Error())
		return
	}
	if exitCode != 0 {
		console.jobs.finishJob(jobID, "failed", nil, fmt.Sprintf("provision command exited with code %d", exitCode))
		return
	}
	console.jobs.finishJob(jobID, "completed", parseHostConsoleSummary(logText), "")
}

func readHostConsoleTeamRequest(reader io.Reader) (hostConsoleTeamRequest, error) {
	var teamRequest hostConsoleTeamRequest
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if errorValue := decoder.Decode(&teamRequest); errorValue != nil {
		return hostConsoleTeamRequest{}, errors.New("요청 JSON을 읽을 수 없습니다")
	}
	normalizedRequest := normalizeHostConsoleTeamRequest(teamRequest)
	if errorValue := validateHostConsoleTeamRequest(normalizedRequest); errorValue != nil {
		return hostConsoleTeamRequest{}, errorValue
	}
	return normalizedRequest, nil
}

func normalizeHostConsoleTeamRequest(teamRequest hostConsoleTeamRequest) hostConsoleTeamRequest {
	members := make([]hostConsoleMemberRequest, 0, len(teamRequest.Members))
	for _, member := range teamRequest.Members {
		members = append(members, hostConsoleMemberRequest{
			Email:    strings.TrimSpace(member.Email),
			Name:     strings.TrimSpace(member.Name),
			Password: strings.TrimSpace(member.Password),
		})
	}
	return hostConsoleTeamRequest{
		TeamID:         strings.TrimSpace(teamRequest.TeamID),
		DisplayName:    strings.TrimSpace(teamRequest.DisplayName),
		Members:        members,
		ExtraArguments: append([]string{}, teamRequest.ExtraArguments...),
	}
}

func validateHostConsoleTeamRequest(teamRequest hostConsoleTeamRequest) error {
	if !hostConsoleTeamIDPattern.MatchString(teamRequest.TeamID) {
		return errors.New("팀 ID는 2~63자의 소문자, 숫자, 하이픈으로 입력하고 숫자나 문자로 시작해야 합니다")
	}
	if teamRequest.DisplayName == "" {
		return errors.New("표시 이름을 입력하세요")
	}
	if len(teamRequest.Members) == 0 {
		return errors.New("멤버를 한 명 이상 입력하세요")
	}
	for _, member := range teamRequest.Members {
		if errorValue := validateHostConsoleMember(member); errorValue != nil {
			return errorValue
		}
	}
	for _, argument := range teamRequest.ExtraArguments {
		if strings.TrimSpace(argument) == "" {
			return errors.New("추가 인자는 빈 값일 수 없습니다")
		}
	}
	return nil
}

func validateHostConsoleMember(member hostConsoleMemberRequest) error {
	address, errorValue := mail.ParseAddress(member.Email)
	if errorValue != nil || address.Address != member.Email {
		return errors.New("멤버 이메일 형식이 올바르지 않습니다")
	}
	if member.Name == "" {
		return errors.New("멤버 이름을 입력하세요")
	}
	return nil
}

func hostConsoleRemoteArguments(teamRequest hostConsoleTeamRequest) []string {
	arguments := []string{"--display-name", teamRequest.DisplayName}
	for _, member := range teamRequest.Members {
		arguments = append(arguments, "--member", hostConsoleMemberArgument(member))
	}
	arguments = append(arguments, teamRequest.ExtraArguments...)
	return arguments
}

func hostConsoleMemberArgument(member hostConsoleMemberRequest) string {
	return member.Email + ":" + member.Name + ":" + member.Password
}

func (jobs *hostConsoleJobStore) createJob() (string, error) {
	jobs.mutex.Lock()
	defer jobs.mutex.Unlock()
	if jobs.hasRunningJob() {
		return "", errors.New("이미 실행 중인 작업이 있습니다")
	}
	jobID := strconv.FormatInt(time.Now().UnixNano(), 36)
	jobs.jobs[jobID] = hostConsoleJob{JobID: jobID, Status: "running"}
	return jobID, nil
}

func (jobs *hostConsoleJobStore) hasRunningJob() bool {
	for _, job := range jobs.jobs {
		if job.Status == "running" {
			return true
		}
	}
	return false
}

func (jobs *hostConsoleJobStore) appendLog(jobID string, text string) {
	jobs.mutex.Lock()
	defer jobs.mutex.Unlock()
	job := jobs.jobs[jobID]
	job.Log += text
	jobs.jobs[jobID] = job
}

func (jobs *hostConsoleJobStore) finishJob(jobID string, status string, summary json.RawMessage, message string) {
	jobs.mutex.Lock()
	defer jobs.mutex.Unlock()
	job := jobs.jobs[jobID]
	if message != "" {
		job.Log += "\n" + message + "\n"
	}
	job.Status = status
	job.Summary = summary
	jobs.jobs[jobID] = job
}

func (jobs *hostConsoleJobStore) log(jobID string) string {
	jobs.mutex.Lock()
	defer jobs.mutex.Unlock()
	return jobs.jobs[jobID].Log
}

func (jobs *hostConsoleJobStore) job(jobID string) (hostConsoleJob, bool) {
	jobs.mutex.Lock()
	defer jobs.mutex.Unlock()
	job, ok := jobs.jobs[jobID]
	return job, ok
}

func (writer hostConsoleJobLogWriter) Write(data []byte) (int, error) {
	writer.jobs.appendLog(writer.jobID, string(data))
	return len(data), nil
}

func parseHostConsoleSummary(logText string) json.RawMessage {
	lines := strings.Split(logText, "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		line := strings.TrimSpace(lines[index])
		if !strings.HasPrefix(line, "{") {
			continue
		}
		if json.Valid([]byte(line)) {
			return json.RawMessage(line)
		}
	}
	var summary json.RawMessage
	for index, character := range logText {
		if character != '{' {
			continue
		}
		candidate := parseHostConsoleSummaryCandidate(logText[index:])
		if candidate != nil {
			summary = candidate
		}
	}
	return summary
}

func parseHostConsoleSummaryCandidate(text string) json.RawMessage {
	decoder := json.NewDecoder(bytes.NewReader([]byte(text)))
	var value json.RawMessage
	if errorValue := decoder.Decode(&value); errorValue != nil {
		return nil
	}
	var document map[string]any
	if errorValue := json.Unmarshal(value, &document); errorValue != nil {
		return nil
	}
	if _, ok := document["tenantID"]; !ok {
		return nil
	}
	return value
}

func writeHostConsoleJSON(responseWriter http.ResponseWriter, statusCode int, value any) {
	responseWriter.Header().Set("Content-Type", "application/json; charset=utf-8")
	responseWriter.WriteHeader(statusCode)
	_ = json.NewEncoder(responseWriter).Encode(value)
}

func writeHostConsoleError(responseWriter http.ResponseWriter, statusCode int, message string) {
	writeHostConsoleJSON(responseWriter, statusCode, map[string]string{"message": message})
}
