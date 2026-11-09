package mediagate

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

const keepWaitLimit = 30 * time.Second

type Gate struct {
	Upstream  *url.URL
	KeepFiles func(ctx context.Context) error
}

func (gate Gate) Handler() http.Handler {
	return &httputil.ReverseProxy{
		Rewrite: func(request *httputil.ProxyRequest) {
			request.SetURL(gate.Upstream)
			request.Out.Host = request.In.Host
		},
		ModifyResponse: gate.keepBeforeAnswering,
	}
}

func (gate Gate) keepBeforeAnswering(response *http.Response) error {
	if gate.KeepFiles == nil || !changesAnObject(response.Request.Method) || response.StatusCode/100 != 2 {
		return nil
	}
	waiting, cancel := context.WithTimeout(response.Request.Context(), keepWaitLimit)
	defer cancel()
	errorValue := gate.KeepFiles(waiting)
	if errorValue == nil {
		return nil
	}
	log.Printf("an attachment was stored and could not be kept off the box, so the messenger is told it failed: %s %s: %v", response.Request.Method, response.Request.URL.Path, errorValue)
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>ServiceUnavailable</Code><Message>%s</Message></Error>`, "the attachment was not kept off the box; try again")
	response.StatusCode = http.StatusServiceUnavailable
	response.Status = http.StatusText(http.StatusServiceUnavailable)
	response.Header = http.Header{"Content-Type": {"application/xml"}}
	response.ContentLength = int64(len(body))
	response.Body = io.NopCloser(bytes.NewReader([]byte(body)))
	return nil
}

func changesAnObject(method string) bool {
	return method == http.MethodPut || method == http.MethodPost || method == http.MethodDelete
}
