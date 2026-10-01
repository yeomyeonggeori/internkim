package browser

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeDeviceBrowser struct {
	launch   DeviceBrowserLaunch
	exited   chan struct{}
	stopOnce sync.Once
}

func (browser *fakeDeviceBrowser) Exited() <-chan struct{} {
	return browser.exited
}

func (browser *fakeDeviceBrowser) Stop() {
	browser.stopOnce.Do(func() { close(browser.exited) })
}

func (browser *fakeDeviceBrowser) isStopped() bool {
	select {
	case <-browser.exited:
		return true
	default:
		return false
	}
}

type fakeDeviceBrowserHost struct {
	mutex    sync.Mutex
	now      time.Time
	launched []*fakeDeviceBrowser
}

func (host *fakeDeviceBrowserHost) launch(_ context.Context, launch DeviceBrowserLaunch) (RunningDeviceBrowser, error) {
	host.mutex.Lock()
	defer host.mutex.Unlock()
	browser := &fakeDeviceBrowser{launch: launch, exited: make(chan struct{})}
	host.launched = append(host.launched, browser)
	return browser, nil
}

func (host *fakeDeviceBrowserHost) clock() time.Time {
	host.mutex.Lock()
	defer host.mutex.Unlock()
	return host.now
}

func (host *fakeDeviceBrowserHost) advance(duration time.Duration) {
	host.mutex.Lock()
	defer host.mutex.Unlock()
	host.now = host.now.Add(duration)
}

func newFakeDeviceBrowsers(capacity int) (*DeviceBrowsers, *fakeDeviceBrowserHost) {
	host := &fakeDeviceBrowserHost{now: time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC)}
	browsers := NewDeviceBrowsers(DeviceBrowserSettings{
		ExecutablePath: "/usr/local/bin/moli",
		StateDirectory: "/var/lib/internkim/device-browsers",
		FirstPort:      9230,
		Capacity:       capacity,
		UserName:       "blueclaw",
		IdleTimeout:    10 * time.Minute,
		Launch:         host.launch,
		Now:            host.clock,
	})
	return browsers, host
}

func browserFor(t *testing.T, browsers *DeviceBrowsers, requesterEmail string) DeviceBrowser {
	t.Helper()
	browser, errorValue := browsers.BrowserFor(context.Background(), requesterEmail)
	if errorValue != nil {
		t.Fatalf("expected a browser for %q: %v", requesterEmail, errorValue)
	}
	return browser
}

func TestEachMemberGetsTheirOwnBrowserAndProfile(t *testing.T) {
	browsers, host := newFakeDeviceBrowsers(4)

	first := browserFor(t, browsers, "Kim@Example.com")
	again := browserFor(t, browsers, " kim@example.com ")
	second := browserFor(t, browsers, "lee@example.com")

	if first != again {
		t.Fatalf("the same member should reuse their browser, got %+v and %+v", first, again)
	}
	if first.DevtoolsURL == second.DevtoolsURL || first.SessionName == second.SessionName {
		t.Fatalf("two members should not share a browser, got %+v and %+v", first, second)
	}
	if len(host.launched) != 2 {
		t.Fatalf("expected two launches, got %d", len(host.launched))
	}
	firstProfile := host.launched[0].launch.ProfileDirectory
	secondProfile := host.launched[1].launch.ProfileDirectory
	if firstProfile == secondProfile || !strings.HasPrefix(firstProfile, "/var/lib/internkim/device-browsers/members/") {
		t.Fatalf("expected separate member profiles under the state directory, got %q and %q", firstProfile, secondProfile)
	}
	if strings.Contains(filepath.Base(filepath.Dir(firstProfile)), "kim") {
		t.Fatalf("the profile path should not carry the email, got %q", firstProfile)
	}
}

func TestACallWithoutARequesterUsesABrowserOfItsOwn(t *testing.T) {
	browsers, _ := newFakeDeviceBrowsers(4)

	unattributed := browserFor(t, browsers, "")
	member := browserFor(t, browsers, "kim@example.com")

	if unattributed.SessionName != "internkim-device-unattributed" || unattributed.DevtoolsURL == member.DevtoolsURL {
		t.Fatalf("expected an unattributed browser apart from members, got %+v and %+v", unattributed, member)
	}
}

func TestAFullDeviceStopsTheLeastRecentlyUsedBrowser(t *testing.T) {
	browsers, host := newFakeDeviceBrowsers(2)

	first := browserFor(t, browsers, "first@example.com")
	host.advance(time.Minute)
	browserFor(t, browsers, "second@example.com")
	host.advance(time.Minute)
	browserFor(t, browsers, "first@example.com")
	host.advance(time.Minute)
	third := browserFor(t, browsers, "third@example.com")

	waitUntil(t, host.launched[1].isStopped)
	if host.launched[0].isStopped() {
		t.Fatal("expected the recently used browser to keep running")
	}
	if third.DevtoolsURL != "http://127.0.0.1:9231" || first.DevtoolsURL != "http://127.0.0.1:9230" {
		t.Fatalf("expected the freed port to be reused, got first=%s third=%s", first.DevtoolsURL, third.DevtoolsURL)
	}
}

func TestAnIdleBrowserStopsAndStartsAgainWhenNeeded(t *testing.T) {
	browsers, host := newFakeDeviceBrowsers(4)

	browserFor(t, browsers, "kim@example.com")
	host.advance(9 * time.Minute)
	browsers.StopIdle()
	if host.launched[0].isStopped() {
		t.Fatal("expected a browser used nine minutes ago to keep running")
	}

	host.advance(2 * time.Minute)
	browsers.StopIdle()
	if !host.launched[0].isStopped() {
		t.Fatal("expected a browser idle for eleven minutes to stop")
	}

	browserFor(t, browsers, "kim@example.com")
	if len(host.launched) != 2 || host.launched[1].launch.ProfileDirectory != host.launched[0].launch.ProfileDirectory {
		t.Fatal("expected the member's browser to start again on the same profile")
	}
}

func TestABrowserThatExitsIsStartedAgain(t *testing.T) {
	browsers, host := newFakeDeviceBrowsers(4)

	browserFor(t, browsers, "kim@example.com")
	host.launched[0].Stop()
	waitUntil(t, func() bool {
		browsers.mutex.Lock()
		defer browsers.mutex.Unlock()
		return len(browsers.members) == 0
	})

	browserFor(t, browsers, "kim@example.com")
	if len(host.launched) != 2 {
		t.Fatalf("expected a second launch, got %d", len(host.launched))
	}
}

func TestAFailedLaunchIsReportedAndTriedAgainNextTime(t *testing.T) {
	attempts := 0
	browsers := NewDeviceBrowsers(DeviceBrowserSettings{
		FirstPort: 9230,
		Capacity:  1,
		Launch: func(context.Context, DeviceBrowserLaunch) (RunningDeviceBrowser, error) {
			attempts++
			if attempts == 1 {
				return nil, errors.New("moli is missing")
			}
			return &fakeDeviceBrowser{exited: make(chan struct{})}, nil
		},
	})

	if _, errorValue := browsers.BrowserFor(context.Background(), "kim@example.com"); errorValue == nil {
		t.Fatal("expected the failed launch to be reported")
	}
	browserFor(t, browsers, "kim@example.com")
	if attempts != 2 {
		t.Fatalf("expected a second attempt, got %d", attempts)
	}
}

func TestConcurrentCallsForOneMemberShareOneLaunch(t *testing.T) {
	release := make(chan struct{})
	launches := 0
	var launchMutex sync.Mutex
	browsers := NewDeviceBrowsers(DeviceBrowserSettings{
		FirstPort: 9230,
		Capacity:  4,
		Launch: func(context.Context, DeviceBrowserLaunch) (RunningDeviceBrowser, error) {
			launchMutex.Lock()
			launches++
			launchMutex.Unlock()
			<-release
			return &fakeDeviceBrowser{exited: make(chan struct{})}, nil
		},
	})

	var waiting sync.WaitGroup
	for range 3 {
		waiting.Add(1)
		go func() {
			defer waiting.Done()
			browserFor(t, browsers, "kim@example.com")
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	waiting.Wait()

	if launches != 1 {
		t.Fatalf("expected one launch, got %d", launches)
	}
}

func TestTheMoliProcessServesDevtoolsAndStops(t *testing.T) {
	executablePath := os.Getenv("INTERNKIM_TEST_MOLI")
	if executablePath == "" {
		t.Skip("INTERNKIM_TEST_MOLI names a moli binary to start")
	}
	memberDirectory := filepath.Join(t.TempDir(), "members", "probe")
	running, errorValue := LaunchDeviceBrowserProcess(context.Background(), DeviceBrowserLaunch{
		ExecutablePath:   executablePath,
		Port:             9391,
		MemberDirectory:  memberDirectory,
		ProfileDirectory: filepath.Join(memberDirectory, "profile"),
		CacheDirectory:   filepath.Join(memberDirectory, "cache"),
		LogPath:          filepath.Join(memberDirectory, "moli.log"),
	})
	if errorValue != nil {
		t.Fatalf("expected moli to start: %v", errorValue)
	}
	if !devtoolsAnswers(context.Background(), "http://127.0.0.1:9391/json/version") {
		t.Fatal("expected devtools to answer")
	}
	running.Stop()
	select {
	case <-running.Exited():
	default:
		t.Fatal("expected the process to have exited")
	}
	if devtoolsAnswers(context.Background(), "http://127.0.0.1:9391/json/version") {
		t.Fatal("expected devtools to stop answering")
	}
}

func waitUntil(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("condition never held")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
