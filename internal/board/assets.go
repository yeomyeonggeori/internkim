package board

import (
	"os"
	"path/filepath"
)

func AssetsPath(scriptDir string) string {
	return filepath.Join(scriptDir, "assets", "board")
}

func SkillsPath(scriptDir string) string {
	return filepath.Join(AssetsPath(scriptDir), "skills")
}

func SendFilePath(scriptDir string) string {
	return filepath.Join(AssetsPath(scriptDir), "send-file")
}

func GasSourcePath(scriptDir string) string {
	return filepath.Join(AssetsPath(scriptDir), "gas", "Code.gs")
}

func ReadGasBridgeCode(scriptDir string) (string, error) {
	data, error := os.ReadFile(GasSourcePath(scriptDir))
	if error != nil {
		return "", error
	}
	return string(data), nil
}
