package llmbackend

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

func TestFailureCapturePreservesRequestAndResponseBodies(t *testing.T) {
	baseClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, errorValue := io.ReadAll(request.Body)
		if errorValue != nil {
			return nil, errorValue
		}
		return &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(strings.NewReader("plain response")), Header: make(http.Header), Request: request, ContentLength: int64(len(body))}, nil
	})}
	client, capture := NewFailureCapture(baseClient)
	response, errorValue := client.Post("https://secret.example/v1", "application/json", strings.NewReader(`{"prompt":"secret"}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	responseBody, errorValue := io.ReadAll(response.Body)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_ = response.Body.Close()
	if string(responseBody) != "plain response" {
		t.Fatalf("response body = %q", responseBody)
	}
	attempts := capture.Snapshot().Attempts
	if len(attempts) != 1 || attempts[0].RequestBody != `{"prompt":"secret"}` || attempts[0].ResponseBody != "plain response" || attempts[0].Status != http.StatusBadRequest || attempts[0].Method != http.MethodPost || attempts[0].DurationMS < 0 || !attempts[0].ResponseComplete {
		t.Fatalf("unexpected attempt: %#v", attempts)
	}
	if strings.Contains(string(responseBody), "secret") || strings.Contains(attempts[0].RequestBody, "Authorization") {
		t.Fatal("unexpected credential capture")
	}
}

func TestFailureCaptureRecordsRetriesAndTransportErrors(t *testing.T) {
	attempt := 0
	baseClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		attempt++
		if attempt == 1 {
			return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader("retry")), Header: make(http.Header)}, nil
		}
		return nil, errors.New("connection refused")
	})}
	client, capture := NewFailureCapture(baseClient)
	for range 2 {
		request, _ := http.NewRequest(http.MethodPost, "https://secret.example", strings.NewReader("request"))
		response, _ := client.Do(request)
		if response != nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
		}
	}
	attempts := capture.Snapshot().Attempts
	if len(attempts) != 2 || attempts[0].Status != http.StatusServiceUnavailable || attempts[1].ErrorCategory != "transport" {
		t.Fatalf("unexpected attempts: %#v", attempts)
	}
	if attempts[0].ResponseBody != "retry" || attempts[1].ResponseBody != "" {
		t.Fatalf("unexpected bodies: %#v", attempts)
	}
}

func TestFailureCaptureSnapshotIsSafeDuringConcurrentRequests(t *testing.T) {
	client, capture := NewFailureCapture(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte("ok"))), Header: make(http.Header)}, nil
	})})
	var waitGroup sync.WaitGroup
	for range 10 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			request, _ := http.NewRequest(http.MethodPost, "https://secret.example", strings.NewReader("body"))
			response, errorValue := client.Do(request)
			if errorValue == nil {
				_, _ = io.Copy(io.Discard, response.Body)
				_ = response.Body.Close()
			}
		}()
	}
	for range 20 {
		_ = capture.Snapshot()
	}
	waitGroup.Wait()
	if len(capture.Snapshot().Attempts) != 10 {
		t.Fatalf("attempt count = %d", len(capture.Snapshot().Attempts))
	}
}
