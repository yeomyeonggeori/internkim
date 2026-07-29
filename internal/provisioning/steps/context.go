package setup

import (
	"errors"
	"net/http"
)

type Backend string

const (
	BackendSSH Backend = "ssh"
	BackendSD  Backend = "sd"
)

const (
	BoardJetsonOrinNano = "jetson-orin-nano"
	BoardCloudShared    = "cloud-shared"
	BoardSimulation     = "sim"
)

var ErrUnsupportedBackend = errors.New("step does not support this backend")

type BoardConnection interface {
	Run(command string) string
	SCP(localPath, remotePath string) error
}

type SDStage interface {
	WriteFile(stagePath string, data []byte, mode int) error
	RootPath() string
}

type GoogleAuth struct {
	AccessToken string
	Email       string
}

type Callbacks struct {
	Translate func(korean, english string) string

	LoadState func(key string) string
	SaveState func(key, value string)

	GetOpenRouterKey   func(force bool) (string, error)
	GetLiteRTModelPath func(force bool) (string, error)
	GetBuzzKeySeed     func(force bool) (string, error)

	GetGasWebhookURL func(accessToken string) (string, error)

	BinariesVersion             func() string
	InstallBinariesSSH          func(context *Context) error
	InstallBuzzRelayBinariesSSH func(context *Context) error
	InstallAdmindSSH            func(context *Context) error
	InstallCapabilitydSSH       func(context *Context) error
	StageBinariesSD             func(context *Context) error
	SkillsManifest              func() string
	InstallSkillsSSH            func(context *Context) error
	StageSkillsSD               func(context *Context) error
	BlueclawRuntimeManifest     func() string
	InstallBlueclawRuntimeSSH   func(context *Context) error
	BlueclawPayloadManifest     func() string
	InstallBlueclawPayloadSSH   func(context *Context) error

	AdminWebVersion func() string
	DeployAdminWeb  func(context *Context) error

	SyncCloudflareAccess func(context *Context) error

	ConfigureWifiSSH func(context *Context) error
	StageWifiSD      func(context *Context) error

	ProvisionTunnelSSH func(context *Context) error
	StageTunnelSD      func(context *Context) error

	StageBootstrapSD func(context *Context) error

	ConfigureSlackTokenSSH func(context *Context) error
	StageSlackTokenSD      func(context *Context) error

	InstallUsersSyncSSH func(context *Context) error
	StageUsersSyncSD    func(context *Context) error

	InstallMattermost func(context *Context) error
	SetupMattermost   func(context *Context) error
}

type Context struct {
	Backend Backend

	SSH BoardConnection
	SD  SDStage

	Language     string
	StateDir     string
	ScriptDir    string
	BoardType    string
	BoardIP      string
	PublicURL    string
	SetupCommand string
	SetupSteps   string
	SetupLockID  string
	Force        bool
	PlannedSteps map[string]bool

	Google *GoogleAuth

	Callbacks Callbacks

	HTTP *http.Client
}

func (context *Context) T(korean, english string) string {
	if context.Callbacks.Translate != nil {
		return context.Callbacks.Translate(korean, english)
	}
	return korean
}
