package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/coder/websocket"
	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/capabilities"
	"github.com/yeomyeonggeori/internkim/internal/capabilityd"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
	"github.com/yeomyeonggeori/internkim/internal/workqueue"
)

const largestWorkFrameBytes = 16 << 20

var (
	working        sync.Mutex
	browserWorker  capabilityd.Service
	startedBrowser sync.Once
)

type handedWork struct {
	Kind      string                         `json:"kind"`
	RequestID string                         `json:"requestID"`
	Work      capabilities.ToolInvokeRequest `json:"work"`
}

type readyForWork struct {
	Kind string `json:"kind"`
	Free int    `json:"free"`
}

type workAnswer struct {
	Kind      string                          `json:"kind"`
	RequestID string                          `json:"requestID"`
	Status    int                             `json:"status"`
	Body      capabilities.ToolInvokeResponse `json:"body"`
}

func startWorkingForTheServiceBox(ctx context.Context, session box.Session) {
	if !working.TryLock() {
		return
	}
	startedBrowser.Do(func() {
		browserWorker = capabilityd.NewBrowserWorker(context.Background(), browserWorkerConfiguration())
	})
	go func() {
		defer working.Unlock()
		if errorValue := workUntilTheGatewayLetsGo(ctx, session); errorValue != nil {
			log.Printf("taking browser work from the service box: %v; connecting again at the next check", errorValue)
		}
	}()
}

func browserWorkerConfiguration() capabilityd.Configuration {
	layout := blueclaw.LinuxCompanyHostLayout()
	return capabilityd.Configuration{
		AgentBrowserPath:            programOnTheHostPath(layout, blueclaw.AgentBrowserName),
		DeviceBrowserExecutablePath: layout.BinaryPath(blueclaw.DeviceBrowserName),
		DeviceBrowserStateDirectory: blueclaw.CompanyHostBrowserStatePath,
		DeviceBrowserFirstPort:      blueclaw.CompanyHostBrowserFirstPort,
		DeviceBrowserCapacity:       blueclaw.CompanyHostBrowserCapacity,
		DeviceBrowserUserName:       blueclaw.BlueclawUser,
	}
}

func programOnTheHostPath(layout blueclaw.CompanyHostLayout, programName string) string {
	for _, directory := range filepath.SplitList(layout.SearchPath()) {
		candidate := filepath.Join(directory, programName)
		if information, errorValue := os.Stat(candidate); errorValue == nil && !information.IsDir() {
			return candidate
		}
	}
	return programName
}

func workUntilTheGatewayLetsGo(ctx context.Context, session box.Session) error {
	address := workqueue.SocketAddressOf(session.Configuration.GatewayURL) + "/company/" + session.Configuration.Company.ID + "/worker"
	connection, _, errorValue := websocket.Dial(ctx, address, &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": []string{"Bearer " + session.Session.AccessToken}},
	})
	if errorValue != nil {
		return fmt.Errorf("connecting to %s: %w", address, errorValue)
	}
	defer connection.CloseNow()
	log.Printf("taking browser work from %s's queue", session.Configuration.Company.Name)
	connection.SetReadLimit(largestWorkFrameBytes)
	answers := &sync.Mutex{}
	inFlight := &atomic.Int64{}
	if errorValue := sayHowMuchRoomIsLeft(ctx, connection, answers, inFlight); errorValue != nil {
		return errorValue
	}
	for {
		_, frame, errorValue := connection.Read(ctx)
		if errorValue != nil {
			return errorValue
		}
		var work handedWork
		if json.Unmarshal(frame, &work) != nil || work.Kind != "work" || work.RequestID == "" {
			continue
		}
		inFlight.Add(1)
		go func() {
			answerTheWork(ctx, connection, answers, work)
			inFlight.Add(-1)
			if errorValue := sayHowMuchRoomIsLeft(ctx, connection, answers, inFlight); errorValue != nil {
				log.Printf("telling the company queue how much room is left: %v", errorValue)
			}
		}()
	}
}

func answerTheWork(ctx context.Context, connection *websocket.Conn, answers *sync.Mutex, work handedWork) {
	answer := workAnswer{Kind: "result", RequestID: work.RequestID, Status: http.StatusOK}
	response, errorValue := browserWorker.InvokeLocalDeviceBrowserTool(ctx, work.Work)
	if errorValue != nil {
		answer.Status = http.StatusBadGateway
		response = capabilities.ToolInvokeResponse{ToolName: work.Work.ToolName, Outcome: capabilities.ToolOutcomeFailed, Status: "error", Content: errorValue.Error(), IsError: true}
	}
	answer.Body = response
	document, errorValue := json.Marshal(answer)
	if errorValue != nil {
		log.Printf("answering browser work %s: %v", work.RequestID, errorValue)
		return
	}
	answers.Lock()
	defer answers.Unlock()
	if errorValue := connection.Write(ctx, websocket.MessageText, document); errorValue != nil {
		log.Printf("answering browser work %s: %v", work.RequestID, errorValue)
	}
}

func sayHowMuchRoomIsLeft(ctx context.Context, connection *websocket.Conn, answers *sync.Mutex, inFlight *atomic.Int64) error {
	free := max(blueclaw.CompanyHostBrowserCapacity-int(inFlight.Load()), 0)
	document, errorValue := json.Marshal(readyForWork{Kind: "ready", Free: free})
	if errorValue != nil {
		return errorValue
	}
	answers.Lock()
	defer answers.Unlock()
	return connection.Write(ctx, websocket.MessageText, document)
}
