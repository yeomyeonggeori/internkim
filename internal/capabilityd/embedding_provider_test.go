package capabilityd

import (
	"context"
	"testing"
)

type namedEmbeddingProvider struct {
	name string
}

func (provider namedEmbeddingProvider) CreateEmbedding(context.Context, EmbeddingRequest) (EmbeddingResponse, error) {
	return EmbeddingResponse{}, nil
}

func (provider namedEmbeddingProvider) Name() string {
	return provider.name
}

func TestAutomaticEmbeddingProvidersRejectDifferentVectorSpaces(t *testing.T) {
	service := Service{Configuration: Configuration{
		LlamaCppEmbeddingModel:   "embeddinggemma",
		OpenRouterEmbeddingModel: "openai/text-embedding-3-small",
	}}
	providers := service.automaticEmbeddingProviders(
		namedEmbeddingProvider{name: "local"},
		namedEmbeddingProvider{name: "companion"},
		namedEmbeddingProvider{name: "remote"},
	)

	if providerNames(providers) != "local" {
		t.Fatalf("expected only the local vector space, got %q", providerNames(providers))
	}
}

func TestAutomaticEmbeddingProvidersAllowSameModelFallback(t *testing.T) {
	service := Service{Configuration: Configuration{
		LlamaCppEmbeddingModel:   "baai/bge-m3",
		OpenRouterEmbeddingModel: "BAAI/BGE-M3",
	}}
	providers := service.automaticEmbeddingProviders(
		namedEmbeddingProvider{name: "local"},
		namedEmbeddingProvider{name: "companion"},
		namedEmbeddingProvider{name: "remote"},
	)

	if providerNames(providers) != "local,remote" {
		t.Fatalf("expected same-model remote fallback, got %q", providerNames(providers))
	}
}

func providerNames(providers []EmbeddingProvider) string {
	names := ""
	for _, provider := range providers {
		if names != "" {
			names += ","
		}
		names += provider.(interface{ Name() string }).Name()
	}
	return names
}
