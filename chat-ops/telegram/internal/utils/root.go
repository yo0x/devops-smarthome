package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func GetProgressbar(progressPercent, progressBarLen int) (progressBar string) {
	i := 0
	for ; i < progressPercent/(100/progressBarLen); i++ {
		progressBar += "▰"
	}
	for ; i < progressBarLen; i++ {
		progressBar += "▱"
	}
	progressBar += " " + fmt.Sprint(progressPercent) + "%"
	return
}

func FilenameWithoutExt(fileName string) string {
	return fileName[:len(fileName)-len(filepath.Ext(fileName))]
}

func ReadEnvFile(filename string) {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		return
	}
	fileLines := strings.Split(string(bytes), "\n")
	for i := range fileLines {
		trimmedLine := strings.TrimSpace(fileLines[i])
		if len(trimmedLine) == 0 || trimmedLine[0] == '#' {
			continue
		}
		trimmedLine = strings.TrimPrefix(trimmedLine, "export ")
		spaceIndex := strings.Index(trimmedLine, "=")
		if spaceIndex == -1 {
			return
		}
		os.Setenv(trimmedLine[:spaceIndex], trimmedLine[spaceIndex+1:])
	}
}
