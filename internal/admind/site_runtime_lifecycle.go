package admind

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const sitePocketBaseIdleTimeout = 15 * time.Minute
const sitePocketBaseStartTimeout = 8 * time.Second
const sitePocketBaseJanitorInterval = time.Minute

type siteRuntimeActivity struct {
	startMutex    sync.Mutex
	lastRequestAt time.Time
	inflightCount int
	isRunning     bool
}

func (service *Service) siteRuntimeActivityFor(siteID string) *siteRuntimeActivity {
	service.siteRuntimeMutex.Lock()
	defer service.siteRuntimeMutex.Unlock()
	if service.siteRuntimeActivities == nil {
		service.siteRuntimeActivities = map[string]*siteRuntimeActivity{}
	}
	activity := service.siteRuntimeActivities[siteID]
	if activity == nil {
		activity = &siteRuntimeActivity{}
		service.siteRuntimeActivities[siteID] = activity
	}
	return activity
}

func (service *Service) ensureSitePocketBaseRunning(ctx context.Context, site *SiteRecord) error {
	activity := service.siteRuntimeActivityFor(site.SiteID)
	activity.startMutex.Lock()
	defer activity.startMutex.Unlock()
	service.siteRuntimeMutex.Lock()
	if activity.isRunning {
		activity.lastRequestAt = time.Now()
		activity.inflightCount++
		service.siteRuntimeMutex.Unlock()
		return nil
	}
	service.siteRuntimeMutex.Unlock()
	if _, errorValue := service.runCommand(ctx, "systemctl", "start", siteServiceName(site.SiteID)); errorValue != nil {
		return errorValue
	}
	if errorValue := waitForSitePocketBasePort(ctx, site.Port); errorValue != nil {
		return errorValue
	}
	service.siteRuntimeMutex.Lock()
	activity.isRunning = true
	activity.lastRequestAt = time.Now()
	activity.inflightCount++
	service.siteRuntimeMutex.Unlock()
	return nil
}

func waitForSitePocketBasePort(ctx context.Context, port int) error {
	deadline := time.Now().Add(sitePocketBaseStartTimeout)
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	for {
		connection, errorValue := net.DialTimeout("tcp", address, 500*time.Millisecond)
		if errorValue == nil {
			return connection.Close()
		}
		if time.Now().After(deadline) {
			return errorValue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (service *Service) finishSitePocketBaseRequest(siteID string) {
	activity := service.siteRuntimeActivityFor(siteID)
	service.siteRuntimeMutex.Lock()
	defer service.siteRuntimeMutex.Unlock()
	activity.lastRequestAt = time.Now()
	if activity.inflightCount > 0 {
		activity.inflightCount--
	}
}

func (service *Service) markSitePocketBaseRunning(siteID string, isRunning bool) {
	activity := service.siteRuntimeActivityFor(siteID)
	service.siteRuntimeMutex.Lock()
	defer service.siteRuntimeMutex.Unlock()
	activity.isRunning = isRunning
	activity.lastRequestAt = time.Now()
}

func (service *Service) serveDraftSitePocketBase(responseWriter http.ResponseWriter, request *http.Request, site *SiteRecord) {
	if !directoryHasOperationalFiles(filepath.Join(site.HostSourcePath, "pocketbase", "pb_migrations")) &&
		!directoryHasOperationalFiles(filepath.Join(site.HostSourcePath, "pocketbase", "pb_hooks")) {
		http.NotFound(responseWriter, request)
		return
	}
	if !service.siteVersionHasPocketBaseBackend(site, site.CurrentVersionID) {
		http.Error(responseWriter, "site backend starts with the first publish; publish the site once before using its API", http.StatusServiceUnavailable)
		return
	}
	service.proxySitePocketBase(responseWriter, request, site)
}

func (service *Service) startSiteRuntimeJanitor(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(sitePocketBaseJanitorInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				service.stopIdleSitePocketBaseRuntimes(ctx)
			}
		}
	}()
}

func (service *Service) stopIdleSitePocketBaseRuntimes(ctx context.Context) {
	for _, siteID := range service.idleSiteRuntimeIDs() {
		if _, errorValue := service.runCommand(ctx, "systemctl", "stop", siteServiceName(siteID)); errorValue != nil {
			continue
		}
		service.markSitePocketBaseRunning(siteID, false)
	}
}

func (service *Service) idleSiteRuntimeIDs() []string {
	service.siteRuntimeMutex.Lock()
	defer service.siteRuntimeMutex.Unlock()
	idleSiteIDs := []string{}
	for siteID, activity := range service.siteRuntimeActivities {
		if activity.isRunning && activity.inflightCount == 0 && time.Since(activity.lastRequestAt) > sitePocketBaseIdleTimeout {
			idleSiteIDs = append(idleSiteIDs, siteID)
		}
	}
	return idleSiteIDs
}
