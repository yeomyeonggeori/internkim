package admind

import "time"

const (
	googleAuthURL             = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL            = "https://oauth2.googleapis.com/token"
	googleUserinfoURL         = "https://www.googleapis.com/oauth2/v2/userinfo"
	googleCalendarScope       = "https://www.googleapis.com/auth/calendar"
	googleOpenIDScope         = "openid"
	googleUserinfoEmailScope  = "https://www.googleapis.com/auth/userinfo.email"
	googleOAuthStartPath      = "/calendar/oauth/google/start"
	googleOAuthCallbackPath   = "/calendar/oauth/google/callback"
	googleOAuthClientFileName = "client.json"
	googleOAuthAccountPrefix  = "google-"
	googleOAuthStateTTL       = 10 * time.Minute
	googleOAuthSwitchQuery    = "switchAccount"
	googleOAuthPopupQuery     = "popup"
	googleOAuthReturnSignal   = "internkim:calendar:google-oauth-return"

	googleAuthURLOverrideEnv     = "INTERNKIM_GOOGLE_AUTH_URL"
	googleTokenURLOverrideEnv    = "INTERNKIM_GOOGLE_TOKEN_URL"
	googleUserinfoURLOverrideEnv = "INTERNKIM_GOOGLE_USERINFO_URL"
)
