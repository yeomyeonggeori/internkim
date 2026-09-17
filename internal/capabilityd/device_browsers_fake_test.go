package capabilityd

import (
	"context"
	"sync"
	"time"

	browserruntime "gitlab.com/eastriver/internkim/internal/browser"
)

type fakeRunningBrowser struct {
	exited   chan struct{}
	stopOnce sync.Once
}

func (browser *fakeRunningBrowser) Exited() <-chan struct{} {
	return browser.exited
}

func (browser *fakeRunningBrowser) Stop() {
	browser.stopOnce.Do(func() { close(browser.exited) })
}

func fakeDeviceBrowsers(capacity int, now time.Time) *browserruntime.DeviceBrowsers {
	return browserruntime.NewDeviceBrowsers(browserruntime.DeviceBrowserSettings{
		StateDirectory: "/var/lib/internkim/device-browsers",
		FirstPort:      browserruntime.DeviceBrowsersFirstPort,
		Capacity:       capacity,
		Now:            func() time.Time { return now },
		Launch: func(context.Context, browserruntime.DeviceBrowserLaunch) (browserruntime.RunningDeviceBrowser, error) {
			return &fakeRunningBrowser{exited: make(chan struct{})}, nil
		},
	})
}
