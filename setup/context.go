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
	AccessToken  string
	RefreshToken string
	Email        string
	ClientID     string
	ClientSecret string
}

// AuthorizedUserJSON returns an ADC-format authorized_user credentials JSON
// that gws CLI reads via GOOGLE_APPLICATION_CREDENTIALS. Empty string when
// RefreshToken is missing (e.g. when OAuth was not consented offline).
func (auth *GoogleAuth) AuthorizedUserJSON() string {
	if auth == nil || auth.RefreshToken == "" {
		return ""
	}
	return `{
  "type": "authorized_user",
  "client_id": "` + auth.ClientID + `",
  "client_secret": "` + auth.ClientSecret + `",
  "refresh_token": "` + auth.RefreshToken + `"
}
`
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
