package admind

import (
	"net/http"
	"strings"
)

// A signed message cannot be edited in place, so rewriting the eleven hundred
// that link here would leave that many edit marks on conversations people have
// already read. Sending the reader on costs nothing and keeps working as the
// record takes more of what this device still holds.
func (service *Service) forwardedToTheCompany(request *http.Request, identifier string, resolve func(string) string) (string, bool) {
	appURL := strings.TrimRight(strings.TrimSpace(service.Configuration.CentralPlaneAppURL), "/")
	if appURL == "" || request.Method != http.MethodGet {
		return "", false
	}
	query := request.URL.Query()
	given := strings.TrimSpace(query.Get(identifier))
	if given != "" {
		if resolved := resolve(given); resolved != "" {
			query.Set(identifier, resolved)
		} else {
			query.Del(identifier)
		}
	}
	forwarded := appURL + request.URL.Path
	if encoded := query.Encode(); encoded != "" {
		forwarded += "?" + encoded
	}
	return forwarded, true
}

func (service *Service) forwardOldLink(
	responseWriter http.ResponseWriter,
	request *http.Request,
	identifier string,
	resolve func(string) string,
) bool {
	forwarded, canForward := service.forwardedToTheCompany(request, identifier, resolve)
	if !canForward {
		return false
	}
	http.Redirect(responseWriter, request, forwarded, http.StatusFound)
	return true
}
