package llmbackend

import (
	"context"
	"errors"
)

type StreamingBackend interface {
	Backend
	StreamText(ctx context.Context, request TextRequest, emit func(token string)) error
}

func StreamFirstStreamingBackend(ctx context.Context, backends []Backend, request TextRequest, emit func(token string)) error {
	for _, backend := range backends {
		streaming, isStreaming := backend.(StreamingBackend)
		if !isStreaming {
			continue
		}
		return streaming.StreamText(ctx, request, emit)
	}
	return errors.New("no streaming backend available")
}
