package deployops

import "time"

type ServerOptions struct {
	RepositoryRootPath string
	InternKimHomePath  string
	ExecutablePath     string
	ListenAddress      string
	EnableSvelteUI     bool
}

type Target struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	AdminURL     string `json:"adminURL"`
	Profile      string `json:"profile,omitempty"`
	NodeArgument string `json:"nodeArgument,omitempty"`
	NodeID       string `json:"nodeID,omitempty"`
	StatePath    string `json:"statePath,omitempty"`
	SecretSource string `json:"secretSource,omitempty"`
}

type TargetRegistry struct {
	Targets []Target `json:"targets"`
}

type TargetStatus struct {
	TargetID   string            `json:"targetID"`
	CheckedAt  time.Time         `json:"checkedAt"`
	Admin      EndpointStatus    `json:"admin"`
	Mattermost EndpointStatus    `json:"mattermost"`
	Release    EndpointStatus    `json:"release"`
	Recovery   RecoveryStatus    `json:"recovery"`
	Services   map[string]string `json:"services,omitempty"`
}

type EndpointStatus struct {
	State          string `json:"state"`
	Code           int    `json:"code,omitempty"`
	Message        string `json:"message,omitempty"`
	StartedAt      string `json:"startedAt,omitempty"`
	Release        string `json:"release,omitempty"`
	CurrentRelease string `json:"currentRelease,omitempty"`
	LatestRelease  string `json:"latestRelease,omitempty"`
	UpdateAllowed  bool   `json:"updateAllowed,omitempty"`
}

type RecoveryStatus struct {
	State    string            `json:"state"`
	Message  string            `json:"message,omitempty"`
	Services map[string]string `json:"services,omitempty"`
}

type JobRequest struct {
	Action string `json:"action"`
}

type Job struct {
	ID         string     `json:"id"`
	TargetID   string     `json:"targetID"`
	Action     string     `json:"action"`
	State      string     `json:"state"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	Error      string     `json:"error,omitempty"`
	Events     []JobEvent `json:"events,omitempty"`
}

type JobEvent struct {
	ID      int       `json:"id"`
	JobID   string    `json:"jobID"`
	At      time.Time `json:"at"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

const (
	JobActionCheck           = "check"
	JobActionDeployAdmind    = "deploy-admind"
	JobActionDeployRuntime   = "deploy-runtime"
	JobActionDeployWeb       = "deploy-web"
	JobActionApplyRelease    = "apply-release"
	JobActionPilotStandard   = "pilot-standard"
	JobActionRestartSSH      = "restart-ssh"
	JobActionRestartSSHRoute = "restart-cloudflared-node-ssh"
	JobActionMattermostSmoke = "mattermost-smoke"
)
