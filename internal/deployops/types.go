package deployops

import "time"

type ServerOptions struct {
	RepositoryRootPath string
	ExecutablePath     string
	ListenAddress      string
	EnableSvelteUI     bool
}

type Target struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	AdminURL    string `json:"adminURL"`
	SSHHostname string `json:"sshHostname,omitempty"`
	FleetID     string `json:"fleetID,omitempty"`
	FleetSecret string `json:"-"`
}

type TargetRegistry struct {
	Targets []Target `json:"targets"`
}

type TargetStatus struct {
	TargetID  string            `json:"targetID"`
	CheckedAt time.Time         `json:"checkedAt"`
	Admin     EndpointStatus    `json:"admin"`
	Release   EndpointStatus    `json:"release"`
	Recovery  RecoveryStatus    `json:"recovery"`
	LLM       LLMModelStatus    `json:"llm"`
	Versions  VersionStatus     `json:"versions"`
	Services  map[string]string `json:"services,omitempty"`
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

type VersionStatus struct {
	Admind  string         `json:"admind,omitempty"`
	Runtime string         `json:"runtime,omitempty"`
	Web     string         `json:"web,omitempty"`
	Current ReleaseVersion `json:"current,omitempty"`
	Latest  ReleaseVersion `json:"latest,omitempty"`
}

type ReleaseVersion struct {
	ReleaseID       string                    `json:"releaseID,omitempty"`
	Admind          string                    `json:"admind,omitempty"`
	Capabilityd     string                    `json:"capabilityd,omitempty"`
	BlueclawPayload string                    `json:"blueclawPayload,omitempty"`
	Skills          string                    `json:"skills,omitempty"`
	Runtime         string                    `json:"runtime,omitempty"`
	Web             string                    `json:"web,omitempty"`
	Components      map[string]ComponentBrief `json:"components,omitempty"`
}

type ComponentBrief struct {
	Revision string `json:"revision,omitempty"`
	SHA256   string `json:"sha256,omitempty"`
}

type LLMModelStatus struct {
	State       string `json:"state"`
	Model       string `json:"model,omitempty"`
	Message     string `json:"message,omitempty"`
	RuntimePath string `json:"runtimePath,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
	Restarted   bool   `json:"restarted,omitempty"`
}

type UpdateLLMModelRequest struct {
	Model string `json:"model"`
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
)
