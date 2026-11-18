package capabilityd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/modelpayer"
	"github.com/yeomyeonggeori/internkim/internal/servicetoken"
)

func (configuration Configuration) routesModels() bool {
	return strings.TrimSpace(configuration.CentralPlaneURL) != "" && strings.TrimSpace(configuration.ModelRouteListenAddress) != ""
}

// startModelRoute answers OpenRouter's paths on loopback for the agent and for
// capabilityd's own model calls, and sends each one to whoever pays now. It is
// given the configuration before reachingModels points the model calls at it,
// because the route itself still needs the company key's own path.
func startModelRoute(ctx context.Context, configuration Configuration) error {
	if !configuration.routesModels() {
		return nil
	}
	if strings.TrimSpace(configuration.ServiceTokenPath) == "" {
		return errors.New("the model route answers only callers carrying the service token, and --service-token names none")
	}
	listener, errorValue := net.Listen("tcp", configuration.ModelRouteListenAddress)
	if errorValue != nil {
		return fmt.Errorf("listening for model calls on %s: %w", configuration.ModelRouteListenAddress, errorValue)
	}
	router := modelpayer.NewRouter(configuration.modelPayerConfiguration())
	go router.KeepAsking(ctx)
	go serveModelRoute(ctx, listener, modelRouteHandler(configuration.ServiceTokenPath, router))
	return nil
}

func (configuration Configuration) modelPayerConfiguration() modelpayer.Configuration {
	return modelpayer.Configuration{
		CentralPlaneURL: configuration.CentralPlaneURL,
		CompanyKeyPath:  configuration.OpenRouterKeyPath,
		CredentialPath:  configuration.HostSessionPath,
	}
}

func serveModelRoute(ctx context.Context, listener net.Listener, handler http.Handler) {
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
	}()
	if errorValue := server.Serve(listener); errorValue != nil && !errors.Is(errorValue, http.ErrServerClosed) {
		log.Printf("capability.model_route_stopped: address=%s error=%v", listener.Addr(), errorValue)
	}
}

func modelRouteHandler(serviceTokenPath string, router http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if !servicetoken.IsCarriedBy(request, serviceTokenPath) {
			http.Error(responseWriter, "the model route answers only the host services, which carry the service token", http.StatusUnauthorized)
			return
		}
		router.ServeHTTP(responseWriter, request)
	})
}
