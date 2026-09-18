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

func deviceBrowsersWhoseLaunchWaits(capacity int) (*browserruntime.DeviceBrowsers, <-chan struct{}, func()) {
	launchStarted := make(chan struct{}, capacity)
	launchGate := make(chan struct{})
	browsers := browserruntime.NewDeviceBrowsers(browserruntime.DeviceBrowserSettings{
		StateDirectory: "/var/lib/internkim/device-browsers",
		FirstPort:      browserruntime.DeviceBrowsersFirstPort,
		Capacity:       capacity,
		Now:            time.Now,
		Launch: func(context.Context, browserruntime.DeviceBrowserLaunch) (browserruntime.RunningDeviceBrowser, error) {
			launchStarted <- struct{}{}
			<-launchGate
			return &fakeRunningBrowser{exited: make(chan struct{})}, nil
		},
	})
	return browsers, launchStarted, func() { close(launchGate) }
}
