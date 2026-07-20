package localfleet

import "time"

const (
	DefaultVirtualMachineName = "internkim-local-fleet"
	DefaultRecipe             = "predeploy-gate"
	DefaultAdminHostPort      = 18080
	DefaultMattermostHostPort = 8065

	ActionUp               = "up"
	ActionDown             = "down"
	ActionReset            = "reset"
	ActionRunRecipe        = "runRecipe"
	ActionRunScenario      = "runScenario"
	ActionVerifyRegression = "verifyRegression"

	LLMDModeAuthoritative LLMDMode = "authoritative"
)

type LLMDMode string

type Options struct {
	RepositoryRootPath    string
	ExecutablePath        string
	StateRootPath         string
	VirtualMachineName    string
	RunID                 string
	AdminHostPort         int
	MattermostHostPort    int
	GenerationSeed        string
	GenerationTemperature string
	MaximumModelTier      string
	LLMDMode              LLMDMode
	IsEphemeral           bool
	ShouldUseRealModels   bool
}

type JobRequest struct {
	Action            string `json:"action"`
	Recipe            string `json:"recipe,omitempty"`
	Scenario          string `json:"scenario,omitempty"`
	Base              string `json:"base,omitempty"`
	KeepArtifacts     bool   `json:"keepArtifacts,omitempty"`
	WithoutMattermost bool   `json:"withoutMattermost,omitempty"`
	SkipWeb           bool   `json:"skipWeb,omitempty"`
}

type Status struct {
	CheckedAt      time.Time      `json:"checkedAt"`
	VirtualMachine EndpointStatus `json:"virtualMachine"`
	SSH            EndpointStatus `json:"ssh"`
	Admin          EndpointStatus `json:"admin"`
	Mattermost     EndpointStatus `json:"mattermost"`
	AdminURL       string         `json:"adminURL,omitempty"`
	MattermostURL  string         `json:"mattermostURL,omitempty"`
	LastResult     string         `json:"lastResult,omitempty"`
	CleanupNeeded  bool           `json:"cleanupNeeded"`
	StatePath      string         `json:"statePath,omitempty"`
}

type EndpointStatus struct {
	State   string `json:"state"`
	Message string `json:"message,omitempty"`
}

type CommandPlan struct {
	DirectoryPath string
	Name          string
	Arguments     []string
	Environment   []string
}

type Logger interface {
	Info(message string)
}
