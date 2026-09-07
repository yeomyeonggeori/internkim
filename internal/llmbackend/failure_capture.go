package llmbackend

import (
	"bytes"
	"io"
	"net/http"
	"sync"
	"time"
)

type FailureCaptureAttempt struct {
	StartedAt        time.Time `json:"startedAt"`
	DurationMS       int64     `json:"durationMs"`
	Method           string    `json:"method"`
	Status           int       `json:"status"`
	RequestBody      string    `json:"requestBody"`
	ResponseBody     string    `json:"responseBody"`
	ResponseComplete bool      `json:"responseComplete"`
	ErrorCategory    string    `json:"errorCategory,omitempty"`
}

type FailureCaptureSnapshot struct {
	Attempts []FailureCaptureAttempt `json:"attempts"`
}

type FailureCapture struct {
	mutex    sync.Mutex
	attempts []FailureCaptureAttempt
}

func NewFailureCapture(client *http.Client) (*http.Client, *FailureCapture) {
	if client == nil {
		client = http.DefaultClient
	}
	capture := &FailureCapture{}
	capturedClient := *client
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	capturedClient.Transport = failureCaptureTransport{base: transport, capture: capture}
	return &capturedClient, capture
}

func (capture *FailureCapture) Snapshot() FailureCaptureSnapshot {
	capture.mutex.Lock()
	defer capture.mutex.Unlock()
	attempts := append([]FailureCaptureAttempt(nil), capture.attempts...)
	return FailureCaptureSnapshot{Attempts: attempts}
}

type failureCaptureTransport struct {
	base    http.RoundTripper
	capture *FailureCapture
}

func (transport failureCaptureTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	startedAt := time.Now()
	attempt := transport.capture.startAttempt(startedAt, request.Method)
	requestBody, requestError := readRequestBody(request)
	transport.capture.updateRequestBody(attempt, requestBody)
	if requestError != nil {
		transport.capture.setErrorCategory(attempt, "request_body")
		transport.capture.finishAttempt(attempt, startedAt, 0)
		return nil, requestError
	}
	response, transportError := transport.base.RoundTrip(request)
	if transportError != nil {
		transport.capture.setErrorCategory(attempt, "transport")
		transport.capture.finishAttempt(attempt, startedAt, 0)
		return response, transportError
	}
	if response == nil || response.Body == nil {
		transport.capture.finishAttempt(attempt, startedAt, responseStatus(response))
		transport.capture.setResponseComplete(attempt, true)
		return response, nil
	}
	response.Body = &failureCaptureBody{ReadCloser: response.Body, capture: transport.capture, attempt: attempt, startedAt: startedAt, status: response.StatusCode}
	return response, nil
}

func readRequestBody(request *http.Request) ([]byte, error) {
	if request.Body == nil {
		return nil, nil
	}
	body, errorValue := io.ReadAll(request.Body)
	_ = request.Body.Close()
	request.Body = io.NopCloser(bytes.NewReader(body))
	return body, errorValue
}

func responseStatus(response *http.Response) int {
	if response == nil {
		return 0
	}
	return response.StatusCode
}

func (capture *FailureCapture) startAttempt(startedAt time.Time, method string) int {
	capture.mutex.Lock()
	defer capture.mutex.Unlock()
	capture.attempts = append(capture.attempts, FailureCaptureAttempt{StartedAt: startedAt.UTC(), Method: method})
	return len(capture.attempts) - 1
}

func (capture *FailureCapture) updateRequestBody(attempt int, body []byte) {
	capture.mutex.Lock()
	defer capture.mutex.Unlock()
	capture.attempts[attempt].RequestBody = string(body)
}

func (capture *FailureCapture) setErrorCategory(attempt int, category string) {
	capture.mutex.Lock()
	defer capture.mutex.Unlock()
	if capture.attempts[attempt].ErrorCategory == "" {
		capture.attempts[attempt].ErrorCategory = category
	}
}

func (capture *FailureCapture) finishAttempt(attempt int, startedAt time.Time, status int) {
	capture.mutex.Lock()
	defer capture.mutex.Unlock()
	capture.attempts[attempt].DurationMS = time.Since(startedAt).Milliseconds()
	capture.attempts[attempt].Status = status
}

func (capture *FailureCapture) setResponseComplete(attempt int, complete bool) {
	capture.mutex.Lock()
	defer capture.mutex.Unlock()
	capture.attempts[attempt].ResponseComplete = complete
}

type failureCaptureBody struct {
	io.ReadCloser
	capture          *FailureCapture
	attempt          int
	startedAt        time.Time
	status           int
	buffer           bytes.Buffer
	finished         bool
	responseComplete bool
	mutex            sync.Mutex
}

func (body *failureCaptureBody) Read(destination []byte) (int, error) {
	count, errorValue := body.ReadCloser.Read(destination)
	body.mutex.Lock()
	if count > 0 {
		_, _ = body.buffer.Write(destination[:count])
	}
	body.mutex.Unlock()
	if errorValue == io.EOF {
		body.mutex.Lock()
		body.responseComplete = true
		body.mutex.Unlock()
		body.finish()
	} else if errorValue != nil {
		body.capture.setErrorCategory(body.attempt, "response_body")
		body.finish()
	}
	return count, errorValue
}

func (body *failureCaptureBody) Close() error {
	errorValue := body.ReadCloser.Close()
	body.finish()
	return errorValue
}

func (body *failureCaptureBody) finish() {
	body.mutex.Lock()
	defer body.mutex.Unlock()
	if body.finished {
		return
	}
	body.finished = true
	responseBody := body.buffer.String()
	responseComplete := body.responseComplete
	body.capture.mutex.Lock()
	defer body.capture.mutex.Unlock()
	body.capture.attempts[body.attempt].ResponseBody = responseBody
	body.capture.attempts[body.attempt].ResponseComplete = responseComplete
	body.capture.attempts[body.attempt].DurationMS = time.Since(body.startedAt).Milliseconds()
	body.capture.attempts[body.attempt].Status = body.status
}
