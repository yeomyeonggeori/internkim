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

	GoogleAuth           func() (*GoogleAuth, error)
	ResolveGoogleProject func(httpClient *http.Client, accessToken, deviceID string) (string, error)
	EnableGoogleAPIs     func(httpClient *http.Client, accessToken, projectID string) error
	CreateGoogleSA       func(deviceID, accessToken string) (string, error)

	GetOpenRouterKey func(force bool) (string, error)

	// GetGasWebhookURL returns the user's deployed Google Apps Script Web
	// App URL. accessToken is the OAuth token the caller already holds
	// (typically from a prior GoogleAuth() call in the same run), so this
	// implementation does not trigger a second browser consent flow.
	// When no token is known, pass "" — implementations may obtain their
	// own. Implementations persist the URL under the host state directory
	// so subsequent invocations short-circuit.
	GetGasWebhookURL func(accessToken string) (string, error)

	GwsSkillsInstallScript string

	InstallBinariesSSH func(context *Context) error
	StageBinariesSD    func(context *Context) error

	ConfigureWifiSSH func(context *Context) error
	StageWifiSD      func(context *Context) error

	ProvisionTunnelSSH func(context *Context) error
	StageTunnelSD      func(context *Context) error

	InstallMattermost func(context *Context) error
	SetupMattermost   func(context *Context) error
}

type Context struct {
	Backend Backend

	SSH BoardConnection
	SD  SDStage

	Language  string
	StateDir  string
	ScriptDir string
	BoardIP   string
	Force     bool

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
