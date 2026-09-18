package browser

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const DeviceBrowsersStateDirectory = "/var/lib/internkim/device-browsers"
const DeviceBrowsersFirstPort = 9230
const DeviceBrowsersCapacity = 4
const DeviceBrowsersUserName = "blueclaw"
const deviceBrowserIdleTimeout = 10 * time.Minute
const deviceBrowserTidyInterval = time.Minute
const unattributedBrowserKey = "unattributed"

var ErrDeviceBrowsersFull = errors.New("every device browser is in use by someone else")

type DeviceBrowserLaunch struct {
	ExecutablePath   string
	Port             int
	MemberDirectory  string
	ProfileDirectory string
	CacheDirectory   string
	LogPath          string
	UserName         string
}

type RunningDeviceBrowser interface {
	Exited() <-chan struct{}
	Stop()
}

type DeviceBrowserSettings struct {
	ExecutablePath string
	StateDirectory string
	FirstPort      int
	Capacity       int
	UserName       string
	IdleTimeout    time.Duration
	Launch         func(context.Context, DeviceBrowserLaunch) (RunningDeviceBrowser, error)
	Now            func() time.Time
}

type DeviceBrowser struct {
	DevtoolsURL string
	SessionName string
}

type DeviceBrowsers struct {
	settings DeviceBrowserSettings
	mutex    sync.Mutex
	members  map[string]*memberBrowser
}

type memberBrowser struct {
	key         string
	port        int
	lastUsedAt  time.Time
	running     RunningDeviceBrowser
	launched    chan struct{}
	launchError error
}

func NewDeviceBrowsers(settings DeviceBrowserSettings) *DeviceBrowsers {
	if settings.Launch == nil {
		settings.Launch = LaunchDeviceBrowserProcess
	}
	if settings.Now == nil {
		settings.Now = time.Now
	}
	if settings.IdleTimeout <= 0 {
		settings.IdleTimeout = deviceBrowserIdleTimeout
	}
	if settings.Capacity <= 0 {
		settings.Capacity = DeviceBrowsersCapacity
	}
	return &DeviceBrowsers{settings: settings, members: map[string]*memberBrowser{}}
}

func (browsers *DeviceBrowsers) BrowserFor(ctx context.Context, requesterEmail string) (DeviceBrowser, error) {
	member, errorValue := browsers.memberFor(memberKeyOf(requesterEmail))
	if errorValue != nil {
		return DeviceBrowser{}, errorValue
	}
	select {
	case <-member.launched:
	case <-ctx.Done():
		return DeviceBrowser{}, ctx.Err()
	}
	if member.launchError != nil {
		return DeviceBrowser{}, member.launchError
	}
	return DeviceBrowser{
		DevtoolsURL: fmt.Sprintf("http://127.0.0.1:%d", member.port),
		SessionName: "internkim-device-" + member.key,
	}, nil
}

func (browsers *DeviceBrowsers) KeepTidy(ctx context.Context) {
	ticker := time.NewTicker(deviceBrowserTidyInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			browsers.StopAll()
			return
		case <-ticker.C:
			browsers.StopIdle()
		}
	}
}

func (browsers *DeviceBrowsers) StopIdle() {
	browsers.mutex.Lock()
	idleSince := browsers.settings.Now().Add(-browsers.settings.IdleTimeout)
	idle := []*memberBrowser{}
	for _, member := range browsers.members {
		if browsers.canStop(member) && member.lastUsedAt.Before(idleSince) {
			idle = append(idle, member)
			delete(browsers.members, member.key)
		}
	}
	browsers.mutex.Unlock()
	stopEach(idle)
}

func (browsers *DeviceBrowsers) StopAll() {
	browsers.mutex.Lock()
	everyone := []*memberBrowser{}
	for _, member := range browsers.members {
		if member.running != nil {
			everyone = append(everyone, member)
		}
	}
	browsers.members = map[string]*memberBrowser{}
	browsers.mutex.Unlock()
	stopEach(everyone)
}

func (browsers *DeviceBrowsers) memberFor(key string) (*memberBrowser, error) {
	browsers.mutex.Lock()
	defer browsers.mutex.Unlock()
	if member, isKnown := browsers.members[key]; isKnown {
		member.lastUsedAt = browsers.settings.Now()
		return member, nil
	}
	if len(browsers.members) >= browsers.settings.Capacity {
		leastRecent := browsers.leastRecentlyUsedStoppable()
		if leastRecent == nil {
			return nil, ErrDeviceBrowsersFull
		}
		delete(browsers.members, leastRecent.key)
		go leastRecent.running.Stop()
	}
	member := &memberBrowser{key: key, port: browsers.freePort(), lastUsedAt: browsers.settings.Now(), launched: make(chan struct{})}
	browsers.members[key] = member
	go browsers.launch(member)
	return member, nil
}

func (browsers *DeviceBrowsers) launch(member *memberBrowser) {
	running, errorValue := browsers.settings.Launch(context.Background(), browsers.launchOf(member))
	browsers.mutex.Lock()
	member.running = running
	member.launchError = errorValue
	if errorValue != nil && browsers.members[member.key] == member {
		delete(browsers.members, member.key)
	}
	browsers.mutex.Unlock()
	close(member.launched)
	if errorValue == nil {
		go browsers.forgetWhenExited(member)
	}
}

func (browsers *DeviceBrowsers) forgetWhenExited(member *memberBrowser) {
	<-member.running.Exited()
	browsers.mutex.Lock()
	defer browsers.mutex.Unlock()
	if browsers.members[member.key] == member {
		delete(browsers.members, member.key)
	}
}

func (browsers *DeviceBrowsers) launchOf(member *memberBrowser) DeviceBrowserLaunch {
	memberDirectory := filepath.Join(browsers.settings.StateDirectory, "members", member.key)
	return DeviceBrowserLaunch{
		ExecutablePath:   browsers.settings.ExecutablePath,
		Port:             member.port,
		MemberDirectory:  memberDirectory,
		ProfileDirectory: filepath.Join(memberDirectory, "profile"),
		CacheDirectory:   filepath.Join(memberDirectory, "cache"),
		LogPath:          filepath.Join(memberDirectory, "moli.log"),
		UserName:         browsers.settings.UserName,
	}
}

func (browsers *DeviceBrowsers) canStop(member *memberBrowser) bool {
	return member.running != nil
}

func (browsers *DeviceBrowsers) leastRecentlyUsedStoppable() *memberBrowser {
	var leastRecent *memberBrowser
	for _, member := range browsers.members {
		if !browsers.canStop(member) {
			continue
		}
		if leastRecent == nil || member.lastUsedAt.Before(leastRecent.lastUsedAt) {
			leastRecent = member
		}
	}
	return leastRecent
}

func (browsers *DeviceBrowsers) freePort() int {
	taken := map[int]bool{}
	for _, member := range browsers.members {
		taken[member.port] = true
	}
	port := browsers.settings.FirstPort
	for taken[port] {
		port++
	}
	return port
}

func stopEach(members []*memberBrowser) {
	for _, member := range members {
		member.running.Stop()
	}
}

func memberKeyOf(requesterEmail string) string {
	email := strings.ToLower(strings.TrimSpace(requesterEmail))
	if email == "" {
		return unattributedBrowserKey
	}
	digest := sha256.Sum256([]byte(email))
	return hex.EncodeToString(digest[:8])
}
