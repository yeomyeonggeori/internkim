//go:build darwin

package machost

import (
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// Neither virtiofsd nor vfkit can export a share read-only, and macOS has no bind mount to
// carry the flag the way the Linux host does. Immutable flags hold instead: vfkit serves the
// share as this user, so a write arriving from the guest is refused, and the virtio-fs
// protocol carries no operation that could clear a BSD file flag.
func SealDeliveryDirectory(layout Layout) error {
	paths, errorValue := deliveryTreePaths(layout)
	if errorValue != nil {
		return errorValue
	}
	for index := len(paths) - 1; index >= 0; index-- {
		if errorValue := unix.Chflags(paths[index], unix.UF_IMMUTABLE); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func UnsealDeliveryDirectory(layout Layout) error {
	paths, errorValue := deliveryTreePaths(layout)
	if errorValue != nil {
		return errorValue
	}
	for _, path := range paths {
		if errorValue := unix.Chflags(path, 0); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func deliveryTreePaths(layout Layout) ([]string, error) {
	if _, errorValue := os.Stat(layout.DeliveryPath()); os.IsNotExist(errorValue) {
		return nil, nil
	}
	paths := []string{}
	errorValue := filepath.WalkDir(layout.DeliveryPath(), func(currentPath string, entry fs.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		paths = append(paths, currentPath)
		return nil
	})
	return paths, errorValue
}
