package admind

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

func (service *Service) handleGoogleOAuthCallback(writer http.ResponseWriter, request *http.Request) {
	service.cleanupExpiredGoogleOAuthStates(time.Now())
	if callbackError := strings.TrimSpace(request.URL.Query().Get("error")); callbackError != "" {
		service.redirectGoogleOAuthCallbackError(writer, request, callbackError)
		return
	}
	state, code, ok := readGoogleOAuthCallbackParams(writer, request)
	if !ok {
		return
	}
	record, ok := service.consumeGoogleOAuthState(writer, request, state)
	if !ok {
		return
	}
	_, exchangeOK := service.completeGoogleOAuthExchange(writer, request, record, code)
	if !exchangeOK {
		respondGoogleOAuthResult(writer, request, record, googleOAuthReturnStatusFailed)
		return
	}
	respondGoogleOAuthResult(writer, request, record, googleOAuthReturnStatusConnected)
}

func readGoogleOAuthCallbackParams(writer http.ResponseWriter, request *http.Request) (string, string, bool) {
	query := request.URL.Query()
	text := googleOAuthResponseTextForRequest(request)
	state := strings.TrimSpace(query.Get("state"))
	code := strings.TrimSpace(query.Get("code"))
	if state == "" || code == "" {
		respondGoogleOAuthErrorHTML(writer, request, http.StatusBadRequest, text.MissingCallbackParameters)
		return "", "", false
	}
	return state, code, true
}

func (service *Service) redirectGoogleOAuthCallbackError(writer http.ResponseWriter, request *http.Request, callbackError string) {
	text := googleOAuthResponseTextForRequest(request)
	state := strings.TrimSpace(request.URL.Query().Get("state"))
	if state == "" {
		respondGoogleOAuthErrorHTML(writer, request, http.StatusBadRequest,
			fmt.Sprintf(text.GoogleReturnedErrorTemplate, callbackError))
		return
	}
	record, ok := service.consumeGoogleOAuthState(writer, request, state)
	if !ok {
		return
	}
	respondGoogleOAuthResult(writer, request, record, googleOAuthReturnStatusFailed)
}

func (service *Service) consumeGoogleOAuthState(writer http.ResponseWriter, request *http.Request, state string) (*googleOAuthStateRecord, bool) {
	text := googleOAuthResponseTextForRequest(request)
	recordRaw, found := service.googleOAuthStates.LoadAndDelete(state)
	if !found {
		respondGoogleOAuthErrorHTML(writer, request, http.StatusBadRequest, text.InvalidState)
		return nil, false
	}
	record, recordOK := recordRaw.(*googleOAuthStateRecord)
	if !recordOK {
		respondGoogleOAuthErrorHTML(writer, request, http.StatusInternalServerError, text.StateRecordTypeError)
		return nil, false
	}
	if time.Since(record.CreatedAt) > googleOAuthStateTTL {
		respondGoogleOAuthErrorHTML(writer, request, http.StatusBadRequest, text.ExpiredState)
		return nil, false
	}
	if strings.TrimSpace(record.ReturnURL) == "" {
		record.ReturnURL = googleOAuthDefaultReturnURL
	}
	return record, true
}

func (service *Service) completeGoogleOAuthExchange(writer http.ResponseWriter, request *http.Request, record *googleOAuthStateRecord, code string) (remoteCalendarAccount, bool) {
	ctx := request.Context()
	configuration, errorValue := service.buildGoogleOAuthConfig(record.RedirectURI)
	if errorValue != nil {
		log.Printf("google oauth callback config: %v", errorValue)
		return remoteCalendarAccount{}, false
	}
	exchangeCtx := context.WithValue(ctx, oauth2.HTTPClient, service.googleOAuthHTTPClient())
	token, errorValue := configuration.Exchange(exchangeCtx, code)
	if errorValue != nil {
		log.Printf("google oauth exchange: %v", errorValue)
		return remoteCalendarAccount{}, false
	}
	email, errorValue := service.fetchGoogleUserEmail(ctx, token)
	if errorValue != nil {
		log.Printf("google oauth userinfo: %v", errorValue)
		return remoteCalendarAccount{}, false
	}
	account, errorValue := service.saveGoogleOAuthTokenAndAccount(ctx, token, email)
	if errorValue != nil {
		log.Printf("google oauth save: %v", errorValue)
		return remoteCalendarAccount{}, false
	}
	return account, true
}

func respondGoogleOAuthResult(writer http.ResponseWriter, request *http.Request, record *googleOAuthStateRecord, status string) {
	if record.Popup {
		respondGoogleOAuthPopupResult(writer, request, record.ReturnURL, status)
		return
	}
	redirectGoogleOAuthResult(writer, request, record.ReturnURL, status)
}
