// Package modelpayer sends each model call to whoever pays for it at the moment
// the call starts: OpenRouter with the company's own key, or the central plane's
// model gateway with this computer's credential. It keeps asking the plane, so a
// changed payer is followed while the services run.
package modelpayer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/modelladder"
)

const (
	askEveryByDefault        = time.Minute
	firstRetryBeforeAnAnswer = time.Second
)

type Configuration struct {
	CentralPlaneURL  string
	CompanyKeyPath   string
	CredentialPath   string
	OpenRouterOrigin string
	AskEvery         time.Duration
	FirstRetry       time.Duration
}

type Router struct {
	configuration Configuration
	askAgain      chan struct{}

	mutex              sync.RWMutex
	isAnswered         bool
	paysThroughGateway bool
	lastFailure        error
}

type upstream struct {
	origin         *url.URL
	keyPath        string
	namesRequester bool
}

func NewRouter(configuration Configuration) *Router {
	if configuration.OpenRouterOrigin == "" {
		configuration.OpenRouterOrigin = modelladder.OpenRouterOrigin()
	}
	if configuration.AskEvery <= 0 {
		configuration.AskEvery = askEveryByDefault
	}
	if configuration.FirstRetry <= 0 {
		configuration.FirstRetry = firstRetryBeforeAnAnswer
	}
	return &Router{configuration: configuration, askAgain: make(chan struct{}, 1)}
}

// KeepAsking asks the plane who pays now, again every AskEvery, and at once
// after a call was refused for want of credits. Until the first answer, every
// call is refused, so a failed ask is retried after a pause that doubles from
// FirstRetry up to AskEvery.
func (router *Router) KeepAsking(ctx context.Context) {
	retry := router.configuration.FirstRetry
	for {
		router.ask()
		pause := router.configuration.AskEvery
		if !router.hasAnswer() {
			pause, retry = retry, min(2*retry, router.configuration.AskEvery)
		}
		select {
		case <-ctx.Done():
			return
		case <-router.askAgain:
		case <-time.After(pause):
		}
	}
}

func (router *Router) hasAnswer() bool {
	router.mutex.RLock()
	defer router.mutex.RUnlock()
	return router.isAnswered
}

func (router *Router) ask() {
	reportingTheKey := modelladder.GatewaySaysWhetherTheCompanyKeyIsInUse(router.configuration.CentralPlaneURL, router.configuration.CredentialPath)
	paysThroughGateway, errorValue := modelladder.PaysThroughGateway(router.configuration.CentralPlaneURL, router.configuration.CompanyKeyPath, reportingTheKey)
	router.mutex.Lock()
	defer router.mutex.Unlock()
	if errorValue != nil {
		log.Printf("model_payer.ask_failed: keeping the payer last heard, if any: %v", errorValue)
		router.lastFailure = errorValue
		return
	}
	if !router.isAnswered || router.paysThroughGateway != paysThroughGateway {
		log.Printf("model_payer.followed: pays_through_gateway=%v", paysThroughGateway)
	}
	router.isAnswered, router.paysThroughGateway, router.lastFailure = true, paysThroughGateway, nil
}

func (router *Router) askSoon() {
	select {
	case router.askAgain <- struct{}{}:
	default:
	}
}

func (router *Router) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	chosen, errorValue := router.currentUpstream()
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusServiceUnavailable)
		return
	}
	key, errorValue := readKey(chosen.keyPath)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusServiceUnavailable)
		return
	}
	proxy := &httputil.ReverseProxy{
		Rewrite:        chosen.rewrite(key),
		FlushInterval:  -1,
		ModifyResponse: router.askSoonWhenCreditsRunOut,
	}
	proxy.ServeHTTP(responseWriter, request)
}

func (router *Router) currentUpstream() (upstream, error) {
	router.mutex.RLock()
	defer router.mutex.RUnlock()
	if !router.isAnswered && router.lastFailure != nil {
		return upstream{}, fmt.Errorf("this computer has not yet learned from the central plane who pays for its models: %w", router.lastFailure)
	}
	if !router.isAnswered {
		return upstream{}, errors.New("this computer is still asking the central plane who pays for its models")
	}
	if router.paysThroughGateway {
		return router.upstreamAt(modelladder.GatewayOrigin(router.configuration.CentralPlaneURL), router.configuration.CredentialPath, true)
	}
	return router.upstreamAt(router.configuration.OpenRouterOrigin, router.configuration.CompanyKeyPath, false)
}

func (router *Router) upstreamAt(origin string, keyPath string, namesRequester bool) (upstream, error) {
	parsed, errorValue := url.Parse(origin)
	if errorValue != nil {
		return upstream{}, fmt.Errorf("the model payer's address %q is not a URL: %w", origin, errorValue)
	}
	return upstream{origin: parsed, keyPath: keyPath, namesRequester: namesRequester}, nil
}

func (chosen upstream) rewrite(key string) func(*httputil.ProxyRequest) {
	return func(proxyRequest *httputil.ProxyRequest) {
		proxyRequest.SetURL(chosen.origin)
		proxyRequest.Out.Header.Set("Authorization", "Bearer "+key)
		if !chosen.namesRequester {
			proxyRequest.Out.Header.Del(modelladder.RequesterEmailHeader)
		}
	}
}

func (router *Router) askSoonWhenCreditsRunOut(response *http.Response) error {
	if response.StatusCode == http.StatusPaymentRequired {
		router.askSoon()
	}
	return nil
}

func readKey(path string) (string, error) {
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return "", fmt.Errorf("reading the key the model payer is called with: %w", errorValue)
	}
	key := strings.TrimSpace(string(document))
	if key == "" {
		return "", fmt.Errorf("the key the model payer is called with, at %s, is empty", path)
	}
	return key, nil
}
