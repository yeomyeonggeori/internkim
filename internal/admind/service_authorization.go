package admind

import (
	"net"
	"net/http"
	"path/filepath"

	"strings"
)

func (service *Service) isAuthorized(request *http.Request) bool {
	if isLocalRequest(request) {
		return true
	}
	callerEmail := service.adminConsoleActorEmail(request)
	if callerEmail == "" {
		return false
	}
	return service.isTaskAdminEmail(request.Context(), callerEmail)
}

func (service *Service) adminConsoleActorEmail(request *http.Request) string {
	return service.webActorEmail(request)
}

func (service *Service) authenticatedCallerEmail(request *http.Request) string {
	if email := service.cloudflareAccessVerifier().verifiedEmail(request.Context(), request); email != "" {
		return email
	}
	// Deployments that do not use Cloudflare front the app with their own
	// identity-aware reverse proxy (oauth2-proxy, Authelia, Authentik, Pomerium)
	// that authenticates the user and injects a trusted email header. The
	// operator opts in with TrustProxyForwardedEmail, asserting the proxy is the
	// only ingress. Absent that, a verified Cloudflare Access JWT is required,
	// except on loopback for local development and tests.
	if service.Configuration.TrustProxyForwardedEmail {
		return forwardedProxyEmail(request)
	}
	if service.cloudflareAccessVerifier().isConfigured() {
		return ""
	}
	if !trustsForwardedIdentity(service.Configuration.ListenAddress) {
		return ""
	}
	return forwardedProxyEmail(request)
}

func forwardedProxyEmail(request *http.Request) string {
	return strings.ToLower(strings.TrimSpace(firstNonEmpty(
		request.Header.Get("Cf-Access-Authenticated-User-Email"),
		request.Header.Get("CF-Access-Authenticated-User-Email"),
		request.Header.Get("X-Forwarded-Email"),
		request.Header.Get("X-Auth-Request-Email"),
	)))
}

func (service *Service) cloudflareAccessVerifier() *cloudflareAccessVerifier {
	service.cloudflareAccessOnce.Do(func() {
		audiences := strings.Split(service.Configuration.CloudflareAccessAUDs, ",")
		service.cloudflareAccessCheck = newCloudflareAccessVerifier(
			service.Configuration.CloudflareAccessTeamDomain,
			audiences,
			service.httpClient(),
		)
	})
	return service.cloudflareAccessCheck
}

func trustsForwardedIdentity(listenAddress string) bool {
	host, _, splitError := net.SplitHostPort(strings.TrimSpace(listenAddress))
	if splitError != nil {
		host = strings.TrimSpace(listenAddress)
	}
	switch host {
	case "127.0.0.1", "::1", "localhost":
		return true
	default:
		return false
	}
}

func (service *Service) hasDeviceAuth() bool {
	fleetID := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath))
	fleetSecret := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetSecretPath))
	return fleetID != "" && fleetSecret != ""
}

func (service *Service) isClaimedAdminEmail(email string) bool {
	claimedEmail := service.claimedAdminEmail()
	return claimedEmail != "" && strings.EqualFold(claimedEmail, email)
}

func (service *Service) claimedAdminEmail() string {
	paths := append([]string{service.Configuration.ClaimedAdminEmailPath}, legacyClaimedAdminEmailPaths(service.Configuration.ClaimedAdminEmailPath)...)
	return readLowerTrimmedFirstExistingFile(paths...)
}

func (service *Service) seedAdminEmail() string {
	paths := append([]string{service.Configuration.AdminEmailPath}, legacyAdminEmailPaths(service.Configuration.AdminEmailPath)...)
	return readLowerTrimmedFirstExistingFile(paths...)
}

func isLocalRequest(request *http.Request) bool {
	host, _, splitError := net.SplitHostPort(request.RemoteAddr)
	if splitError != nil {
		host = request.RemoteAddr
	}
	return host == "127.0.0.1" || host == "::1" || host == "localhost"
}

func readLowerTrimmedFirstExistingFile(paths ...string) string {
	for _, path := range paths {
		value := strings.ToLower(strings.TrimSpace(readTrimmedFile(path)))
		if value != "" {
			return value
		}
	}
	return ""
}

func legacyAdminEmailPaths(path string) []string {
	if filepath.Base(filepath.Dir(path)) != "config" || filepath.Base(path) != "admin-email" {
		return nil
	}
	return []string{filepath.Join(filepath.Dir(filepath.Dir(path)), "admin-email")}
}

func legacyClaimedAdminEmailPaths(path string) []string {
	if filepath.Base(filepath.Dir(path)) != "admin" || filepath.Base(path) != "claimed-admin-email" {
		return nil
	}
	statePath := filepath.Dir(filepath.Dir(path))
	if filepath.Base(statePath) != "state" {
		return nil
	}
	return []string{filepath.Join(filepath.Dir(statePath), "claimed-admin-email")}
}
