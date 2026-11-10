package llmbackend

import (
	"context"
	"net/http"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/modelladder"
)

const RequesterEmailHeader = modelladder.RequesterEmailHeader

type requesterEmailKey struct{}

func ContextWithRequesterEmail(ctx context.Context, email string) context.Context {
	trimmed := strings.TrimSpace(email)
	if trimmed == "" {
		return ctx
	}
	return context.WithValue(ctx, requesterEmailKey{}, trimmed)
}

func NameRequester(request *http.Request) {
	email, _ := request.Context().Value(requesterEmailKey{}).(string)
	if email == "" {
		return
	}
	request.Header.Set(RequesterEmailHeader, email)
}

func (backend OpenRouterBackend) nameRequester(request *http.Request) {
	if !backend.NamesRequester {
		return
	}
	NameRequester(request)
}
