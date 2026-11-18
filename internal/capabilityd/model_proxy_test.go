package capabilityd

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/modelladder"
	"github.com/yeomyeonggeori/internkim/internal/modelpayer"
)

const requesterForModelRouteTest = "member1@example.com"

type heardCall struct {
	path           string
	authorization  string
	requesterEmail string
}

type modelRouteRig struct {
	route             *httptest.Server
	serviceToken      string
	usesOwnModelKey   atomic.Bool
	refusesForCredits atomic.Bool
	payerFails        atomic.Bool
	payerAsks         atomic.Int32
	failsFirstAsks    atomic.Int32
	releaseStream     chan struct{}
	releasing         sync.Once

	mutex           sync.Mutex
	heardOpenRouter []heardCall
	heardGateway    []heardCall
}

func aModelRouteRig(t *testing.T, holdsCompanyKey bool, askEvery time.Duration) *modelRouteRig {
	t.Helper()
	return aModelRouteRigRetrying(t, holdsCompanyKey, askEvery, 0, nil)
}

func aModelRouteRigRetrying(t *testing.T, holdsCompanyKey bool, askEvery time.Duration, firstRetry time.Duration, prepare func(*modelRouteRig)) *modelRouteRig {
	t.Helper()
	rig := &modelRouteRig{serviceToken: "service-token", releaseStream: make(chan struct{})}
	if prepare != nil {
		prepare(rig)
	}
	directory := t.TempDir()
	companyKeyPath := filepath.Join(directory, "openrouter-key")
	if holdsCompanyKey {
		writeFileForModelReachTest(t, companyKeyPath, "sk-company")
	}
	credentialPath := filepath.Join(directory, "agent-key")
	writeFileForModelReachTest(t, credentialPath, "agent-key")
	serviceTokenPath := filepath.Join(directory, "service-token")
	writeFileForModelReachTest(t, serviceTokenPath, rig.serviceToken)

	openRouter := httptest.NewServer(http.HandlerFunc(rig.answerAsOpenRouter))
	t.Cleanup(openRouter.Close)
	plane := httptest.NewServer(http.HandlerFunc(rig.answerAsThePlane))
	t.Cleanup(plane.Close)

	router := modelpayer.NewRouter(modelpayer.Configuration{
		CentralPlaneURL:  plane.URL,
		CompanyKeyPath:   companyKeyPath,
		CredentialPath:   credentialPath,
		OpenRouterOrigin: openRouter.URL,
		AskEvery:         askEvery,
		FirstRetry:       firstRetry,
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go router.KeepAsking(ctx)
	rig.route = httptest.NewServer(modelRouteHandler(serviceTokenPath, router))
	t.Cleanup(rig.route.Close)
	t.Cleanup(rig.release)
	return rig
}

func (rig *modelRouteRig) release() {
	rig.releasing.Do(func() { close(rig.releaseStream) })
}

func (rig *modelRouteRig) answerAsOpenRouter(writer http.ResponseWriter, request *http.Request) {
	rig.mutex.Lock()
	rig.heardOpenRouter = append(rig.heardOpenRouter, heardCallOf(request))
	rig.mutex.Unlock()
	if request.URL.Path != "/api/v1/chat/completions" {
		writer.Write([]byte(`{"answeredBy":"openrouter"}`))
		return
	}
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Write([]byte("data: first-from-openrouter\n\n"))
	writer.(http.Flusher).Flush()
	<-rig.releaseStream
	writer.Write([]byte("data: second-from-openrouter\n\n"))
}

func (rig *modelRouteRig) answerAsThePlane(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path == modelladder.ModelGatewayPayerPath {
		rig.payerAsks.Add(1)
		if rig.payerFails.Load() || rig.payerAsks.Load() <= rig.failsFirstAsks.Load() {
			http.Error(writer, "the plane is having a bad day", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(writer).Encode(map[string]bool{"usesOwnModelKey": rig.usesOwnModelKey.Load()})
		return
	}
	rig.mutex.Lock()
	rig.heardGateway = append(rig.heardGateway, heardCallOf(request))
	rig.mutex.Unlock()
	if request.URL.Path == "/functions/v1/model-gateway/api/v1/chat/completions" && rig.refusesForCredits.Load() {
		http.Error(writer, `{"message":"the company has no credits left"}`, http.StatusPaymentRequired)
		return
	}
	writer.Write([]byte(`{"answeredBy":"gateway"}`))
}

func heardCallOf(request *http.Request) heardCall {
	return heardCall{path: request.URL.Path, authorization: request.Header.Get("Authorization"), requesterEmail: request.Header.Get(modelladder.RequesterEmailHeader)}
}

func (rig *modelRouteRig) call(t *testing.T, method string, path string, authorization string) *http.Response {
	t.Helper()
	request, errorValue := http.NewRequest(method, rig.route.URL+path, strings.NewReader(`{"model":"example/model","stream":true}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request.Header.Set("Authorization", authorization)
	request.Header.Set(modelladder.RequesterEmailHeader, requesterForModelRouteTest)
	response, errorValue := http.DefaultClient.Do(request)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return response
}

func (rig *modelRouteRig) answerTo(t *testing.T, method string, path string) (int, string) {
	t.Helper()
	response := rig.call(t, method, path, "Bearer "+rig.serviceToken)
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	return response.StatusCode, string(body)
}

func (rig *modelRouteRig) heard() ([]heardCall, []heardCall) {
	rig.mutex.Lock()
	defer rig.mutex.Unlock()
	return append([]heardCall{}, rig.heardOpenRouter...), append([]heardCall{}, rig.heardGateway...)
}

func eventuallyForModelRoute(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("waited five seconds for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (rig *modelRouteRig) untilTheModelListIsAnsweredBy(t *testing.T, payer string) {
	t.Helper()
	eventuallyForModelRoute(t, "the model list to come from "+payer, func() bool {
		_, body := rig.answerTo(t, http.MethodGet, "/api/v1/models")
		return strings.Contains(body, payer)
	})
}

func TestTheModelRouteFollowsAChangedPayerAndFinishesAnOpenStreamWhereItStarted(t *testing.T) {
	rig := aModelRouteRig(t, true, 20*time.Millisecond)
	rig.usesOwnModelKey.Store(true)
	rig.untilTheModelListIsAnsweredBy(t, "openrouter")

	stream := rig.call(t, http.MethodPost, "/api/v1/chat/completions", "Bearer "+rig.serviceToken)
	defer stream.Body.Close()
	reader := bufio.NewReader(stream.Body)
	firstLine, errorValue := reader.ReadString('\n')
	if errorValue != nil || !strings.Contains(firstLine, "first-from-openrouter") {
		t.Fatalf("the stream's first event must arrive before the stream ends, got %q, %v", firstLine, errorValue)
	}

	rig.usesOwnModelKey.Store(false)
	rig.untilTheModelListIsAnsweredBy(t, "gateway")
	rig.release()
	rest, errorValue := io.ReadAll(reader)
	if errorValue != nil || !strings.Contains(string(rest), "second-from-openrouter") {
		t.Fatalf("a stream open when the payer changed must finish on OpenRouter, got %q, %v", rest, errorValue)
	}

	status, body := rig.answerTo(t, http.MethodPost, "/api/v1/chat/completions")
	if status != http.StatusOK || !strings.Contains(body, "gateway") {
		t.Fatalf("the call after the change must go to the gateway, got %d %s", status, body)
	}
	heardByOpenRouter, heardByGateway := rig.heard()
	for _, call := range heardByOpenRouter {
		if call.requesterEmail != "" || call.authorization != "Bearer sk-company" {
			t.Fatalf("OpenRouter is called with the company key and never told who asked, heard %+v", call)
		}
	}
	for _, call := range heardByGateway {
		if call.requesterEmail != requesterForModelRouteTest || call.authorization != "Bearer agent-key" {
			t.Fatalf("the gateway is called as this computer and told who asked, heard %+v", call)
		}
	}
}

func TestTheModelRoutePassesOnARefusalForCreditsAndAsksWhoPaysAtOnce(t *testing.T) {
	rig := aModelRouteRig(t, false, time.Hour)
	rig.untilTheModelListIsAnsweredBy(t, "gateway")
	asksBefore := rig.payerAsks.Load()

	rig.refusesForCredits.Store(true)
	status, body := rig.answerTo(t, http.MethodPost, "/api/v1/chat/completions")

	if status != http.StatusPaymentRequired || !strings.Contains(body, "no credits left") {
		t.Fatalf("a refusal for credits is the caller's to see, never quietly paid with another key; got %d %s", status, body)
	}
	eventuallyForModelRoute(t, "the plane to be asked again after the refusal", func() bool {
		return rig.payerAsks.Load() > asksBefore
	})
	heardByOpenRouter, _ := rig.heard()
	if len(heardByOpenRouter) != 0 {
		t.Fatalf("OpenRouter was called %d times by a company paying in credits", len(heardByOpenRouter))
	}
}

func TestTheModelRouteSaysWhyWhenThePlaneHasNeverSaidWhoPays(t *testing.T) {
	rig := aModelRouteRig(t, true, time.Hour)
	rig.payerFails.Store(true)
	eventuallyForModelRoute(t, "the first ask to fail", func() bool { return rig.payerAsks.Load() > 0 })

	eventuallyForModelRoute(t, "a 503 naming the plane's answer", func() bool {
		status, body := rig.answerTo(t, http.MethodGet, "/api/v1/models")
		return status == http.StatusServiceUnavailable && strings.Contains(body, "the plane is having a bad day")
	})
}

func TestTheModelRouteRefusesACallerWithoutTheServiceToken(t *testing.T) {
	rig := aModelRouteRig(t, false, time.Hour)
	rig.untilTheModelListIsAnsweredBy(t, "gateway")
	for _, authorization := range []string{"", "Bearer agent-key", "Bearer not-the-service-token"} {
		response := rig.call(t, http.MethodGet, "/api/v1/models", authorization)
		response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("a caller offering %q got %d", authorization, response.StatusCode)
		}
	}
}

func TestTheModelRouteRetriesSoonWhileItHasNeverHeardWhoPays(t *testing.T) {
	rig := aModelRouteRigRetrying(t, true, time.Hour, 20*time.Millisecond, func(rig *modelRouteRig) {
		rig.failsFirstAsks.Store(1)
		rig.usesOwnModelKey.Store(true)
	})

	rig.untilTheModelListIsAnsweredBy(t, "openrouter")

	if asks := rig.payerAsks.Load(); asks != 2 {
		t.Fatalf("a failed first ask is retried after the first pause, not after the hour-long steady interval; asked %d times", asks)
	}
}
