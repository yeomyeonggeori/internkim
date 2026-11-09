//go:build !linux

package filestream

import (
	"context"
	"errors"
	"io/fs"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/hostbackup"
)

var errWatchingNeedsLinux = errors.New("the company's files are followed on a Linux box, where inotify reports each change")

type Watcher struct{}

func NewWatcher(roots []hostbackup.FileRoot) (*Watcher, error) {
	return nil, errWatchingNeedsLinux
}

func (watcher *Watcher) Close() error {
	return nil
}

func (watcher *Watcher) Next(ctx context.Context) ([]Change, error) {
	return nil, errWatchingNeedsLinux
}

func lastChangeOf(information fs.FileInfo) time.Time {
	return information.ModTime()
}
