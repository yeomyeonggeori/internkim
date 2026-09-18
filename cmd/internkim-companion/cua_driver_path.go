package main

import (
	"os"
	"os/exec"
	"path/filepath"
)

func defaultCuaDriverPath() string {
	if path := os.Getenv("INTERNKIM_CUA_DRIVER_PATH"); executableFilePath(path) != "" {
		return path
	}
	if path, errorValue := exec.LookPath("cua-driver"); errorValue == nil {
		return path
	}
	homeDirectory, errorValue := os.UserHomeDir()
	if errorValue != nil {
		return ""
	}
	return executableFilePath(filepath.Join(homeDirectory, ".local", "bin", "cua-driver"))
}

func executableFilePath(path string) string {
	if path == "" {
		return ""
	}
	information, errorValue := os.Stat(path)
	if errorValue != nil || information.IsDir() || information.Mode()&0o111 == 0 {
		return ""
	}
	return path
}
