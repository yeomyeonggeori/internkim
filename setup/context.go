package setup

import "net/http"

// BoardConn abstracts the connection to the target board (SSH / sim).
// The concrete implementation lives in the main package.
type BoardConn interface {
	// Run executes cmd and returns combined stdout+stderr.
	Run(cmd string) string
	// SCP copies a local file to remote on the board.
	SCP(localPath, remotePath string) error
}

// GoogleAuth carries the access token obtained from Google OAuth.
// Nil when Google login was skipped or failed.
type GoogleAuth struct {
	AccessToken string
	Email       string
}

// Callbacks holds function pointers into the main package so the setup
// package stays free of HTTP / OAuth / GCP concrete dependencies.
type Callbacks struct {
	// T translates (ko, en) → string according to Context.Lang.
	T func(ko, en string) string

	// State helpers (~/.quickclaw key-value file store).
	LoadState func(key string) string
	SaveState func(key, value string)

	// Google Workspace.
	GoogleAuth           func() (*GoogleAuth, error)
	ResolveGoogleProject func(client *http.Client, token, deviceID string) (string, error)
	EnableGoogleAPIs     func(client *http.Client, token, projectID string) error
	CreateGoogleSA       func(deviceID, accessToken string) (string, error)
}

// Context is the shared state passed to every Step.Run.
type Context struct {
	SSH       BoardConn
	Lang      string
	StateDir  string
	ScriptDir string
	BoardIP   string
	Force     bool

	// Auth tokens obtained before the pipeline starts.
	Google *GoogleAuth

	Cb Callbacks

	// HTTP is a pre-built client for Google / external calls.
	HTTP *http.Client
}

// T translates via the injected callback.
func (c *Context) T(ko, en string) string {
	if c.Cb.T != nil {
		return c.Cb.T(ko, en)
	}
	return ko
}
