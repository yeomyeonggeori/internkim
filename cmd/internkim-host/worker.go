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

	"github.com/coder/websocket"
	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/browserhome"
	"github.com/yeomyeonggeori/internkim/internal/capabilities"
	"github.com/yeomyeonggeori/internkim/internal/capabilityd"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
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
	startedBrowser.Do(func() { browserWorker = capabilityd.NewBrowserWorker(context.Background(), browserWorkerConfiguration()) })
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
	address := browserhome.SocketAddressOf(session.Configuration.GatewayURL) + "/company/" + session.Configuration.Company.ID + "/worker"
	connection, _, errorValue := websocket.Dial(ctx, address, &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": []string{"Bearer " + session.Session.AccessToken}},
	})
	if errorValue != nil {
		return fmt.Errorf("connecting to %s: %w", address, errorValue)
	}
	defer connection.CloseNow()
	connection.SetReadLimit(largestWorkFrameBytes)
	answers := &sync.Mutex{}
	for {
		_, frame, errorValue := connection.Read(ctx)
		if errorValue != nil {
			return errorValue
		}
		var work handedWork
		if json.Unmarshal(frame, &work) != nil || work.Kind != "work" || work.RequestID == "" {
			continue
		}
		go answerTheWork(ctx, connection, answers, work)
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
