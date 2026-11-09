//go:build linux

package filestream

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/yeomyeonggeori/internkim/internal/hostbackup"
)

const (
	watchedEvents = unix.IN_CLOSE_WRITE | unix.IN_MODIFY | unix.IN_ATTRIB | unix.IN_CREATE | unix.IN_DELETE |
		unix.IN_MOVED_FROM | unix.IN_MOVED_TO | unix.IN_DONT_FOLLOW | unix.IN_EXCL_UNLINK
	pollMilliseconds = 500
	eventBufferBytes = 64 * 1024
)

type watchedDirectory struct {
	root     hostbackup.FileRoot
	relative string
}

type Watcher struct {
	descriptor  int
	directories map[int32]watchedDirectory
	pending     []Change
}

func NewWatcher(roots []hostbackup.FileRoot) (*Watcher, error) {
	descriptor, errorValue := unix.InotifyInit1(unix.IN_CLOEXEC | unix.IN_NONBLOCK)
	if errorValue != nil {
		return nil, fmt.Errorf("starting to watch the company's files: %w", errorValue)
	}
	watcher := &Watcher{descriptor: descriptor, directories: map[int32]watchedDirectory{}}
	for _, root := range roots {
		if _, errorValue := watcher.watchTree(root, ""); errorValue != nil {
			watcher.Close()
			return nil, errorValue
		}
	}
	return watcher, nil
}

func (watcher *Watcher) Close() error {
	return unix.Close(watcher.descriptor)
}

func (watcher *Watcher) watchTree(root hostbackup.FileRoot, relative string) ([]Change, error) {
	found := []Change{}
	start := filepath.Join(root.Path, filepath.FromSlash(relative))
	errorValue := filepath.WalkDir(start, func(filePath string, entry fs.DirEntry, walkError error) error {
		if errors.Is(walkError, fs.ErrNotExist) {
			return nil
		}
		if walkError != nil {
			return walkError
		}
		inside, errorValue := filepath.Rel(root.Path, filePath)
		if errorValue != nil {
			return errorValue
		}
		inside = filepath.ToSlash(inside)
		if inside == "." {
			inside = ""
		}
		if inside != "" && root.Excludes(inside) && !isBarrierPath(inside) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if inside != "" {
			found = append(found, Change{Root: root, Relative: inside})
		}
		if !entry.IsDir() {
			return nil
		}
		return watcher.watchDirectory(root, inside, filePath)
	})
	return found, errorValue
}

func (watcher *Watcher) watchDirectory(root hostbackup.FileRoot, relative string, directoryPath string) error {
	watch, errorValue := unix.InotifyAddWatch(watcher.descriptor, directoryPath, watchedEvents)
	if errors.Is(errorValue, unix.ENOENT) {
		return nil
	}
	if errors.Is(errorValue, unix.ENOSPC) {
		return fmt.Errorf("watching %s: the kernel's inotify watch limit is reached; raise fs.inotify.max_user_watches", directoryPath)
	}
	if errorValue != nil {
		return fmt.Errorf("watching %s: %w", directoryPath, errorValue)
	}
	watcher.directories[int32(watch)] = watchedDirectory{root: root, relative: relative}
	return nil
}

func (watcher *Watcher) Next(ctx context.Context) ([]Change, error) {
	for {
		if len(watcher.pending) > 0 {
			changes := watcher.pending
			watcher.pending = nil
			return changes, nil
		}
		if errorValue := ctx.Err(); errorValue != nil {
			return nil, errorValue
		}
		polled := []unix.PollFd{{Fd: int32(watcher.descriptor), Events: unix.POLLIN}}
		count, errorValue := unix.Poll(polled, pollMilliseconds)
		if errors.Is(errorValue, unix.EINTR) || count == 0 {
			continue
		}
		if errorValue != nil {
			return nil, fmt.Errorf("waiting for changes to the company's files: %w", errorValue)
		}
		if errorValue := watcher.read(); errorValue != nil {
			return nil, errorValue
		}
	}
}

func (watcher *Watcher) read() error {
	buffer := make([]byte, eventBufferBytes)
	for {
		length, errorValue := unix.Read(watcher.descriptor, buffer)
		if errors.Is(errorValue, unix.EAGAIN) {
			return nil
		}
		if errorValue != nil {
			return fmt.Errorf("reading changes to the company's files: %w", errorValue)
		}
		if errorValue := watcher.takeEvents(buffer[:length]); errorValue != nil {
			return errorValue
		}
	}
}

func (watcher *Watcher) takeEvents(events []byte) error {
	for offset := 0; offset+unix.SizeofInotifyEvent <= len(events); {
		event := (*unix.InotifyEvent)(unsafe.Pointer(&events[offset]))
		nameStart := offset + unix.SizeofInotifyEvent
		nameBytes := events[nameStart : nameStart+int(event.Len)]
		offset = nameStart + int(event.Len)
		name := string(nameBytes[:nameLength(nameBytes)])
		if errorValue := watcher.takeEvent(event, name); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func (watcher *Watcher) takeEvent(event *unix.InotifyEvent, name string) error {
	if event.Mask&unix.IN_Q_OVERFLOW != 0 {
		watcher.pending = append(watcher.pending, Change{IsOverflow: true})
		return nil
	}
	directory, isWatched := watcher.directories[event.Wd]
	if event.Mask&unix.IN_IGNORED != 0 {
		delete(watcher.directories, event.Wd)
		return nil
	}
	if !isWatched || name == "" {
		return nil
	}
	relative := path.Join(directory.relative, name)
	if directory.root.Excludes(relative) && !isBarrierPath(relative) {
		return nil
	}
	watcher.pending = append(watcher.pending, Change{Root: directory.root, Relative: relative})
	if event.Mask&unix.IN_ISDIR != 0 && event.Mask&(unix.IN_CREATE|unix.IN_MOVED_TO) != 0 {
		inside, errorValue := watcher.watchTree(directory.root, relative)
		if errorValue != nil {
			return errorValue
		}
		watcher.pending = append(watcher.pending, inside...)
	}
	return nil
}

func nameLength(bytes []byte) int {
	for index, value := range bytes {
		if value == 0 {
			return index
		}
	}
	return len(bytes)
}

func lastChangeOf(information fs.FileInfo) time.Time {
	status, isUnix := information.Sys().(*unix.Stat_t)
	if !isUnix {
		return information.ModTime()
	}
	changed := time.Unix(status.Ctim.Unix())
	if changed.After(information.ModTime()) {
		return changed
	}
	return information.ModTime()
}
