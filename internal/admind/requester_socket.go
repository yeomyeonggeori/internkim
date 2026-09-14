package admind

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	blueclawruntime "gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

const (
	requesterEmailHeader      = "X-INTERNKIM-REQUESTER-EMAIL"
	requesterPermissionHeader = "X-INTERNKIM-REQUESTER-PERMISSION"
	idempotencyKeyHeader      = "X-INTERNKIM-IDEMPOTENCY-KEY"

	requesterSocketMode      = 0o660
	requesterSocketOwnerName = blueclawruntime.RelayUserName
)

type assertedRequesterListenerMark struct{}

func markRequestsAsAssertedByTheListener(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		markedContext := context.WithValue(request.Context(), assertedRequesterListenerMark{}, assertedRequesterListenerMark{})
		handler.ServeHTTP(responseWriter, request.WithContext(markedContext))
	})
}

func arrivedOnRequesterSocket(request *http.Request) bool {
	return request.Context().Value(assertedRequesterListenerMark{}) != nil
}

func assertedRequesterEmail(request *http.Request) string {
	if !arrivedOnRequesterSocket(request) {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(request.Header.Get(requesterEmailHeader)))
}

func assertedRequesterPermission(request *http.Request) string {
	if !arrivedOnRequesterSocket(request) {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(request.Header.Get(requesterPermissionHeader)))
}

func listenOnRequesterSocket(socketPath string) (net.Listener, error) {
	if errorValue := os.MkdirAll(filepath.Dir(socketPath), 0o755); errorValue != nil {
		return nil, errorValue
	}
	if errorValue := os.Remove(socketPath); errorValue != nil && !errors.Is(errorValue, os.ErrNotExist) {
		return nil, errorValue
	}
	listener, errorValue := net.Listen("unix", socketPath)
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := os.Chmod(socketPath, requesterSocketMode); errorValue != nil {
		_ = listener.Close()
		return nil, errorValue
	}
	userID, groupID, lookupError := lookupUserAndGroupID(requesterSocketOwnerName)
	if lookupError == nil {
		_ = os.Chown(socketPath, userID, groupID)
	}
	return listener, nil
}

func lookupUserAndGroupID(name string) (int, int, error) {
	account, errorValue := user.Lookup(name)
	if errorValue != nil {
		return 0, 0, errorValue
	}
	userID, errorValue := strconv.Atoi(account.Uid)
	if errorValue != nil {
		return 0, 0, errorValue
	}
	groupID, errorValue := strconv.Atoi(account.Gid)
	if errorValue != nil {
		return 0, 0, errorValue
	}
	return userID, groupID, nil
}
