package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"threadfin/src/internal/cli"
)

func WriteByteToFile(file string, data []byte) (err error) {
	var filename = GetPlatformFile(file)
	err = os.WriteFile(filename, data, 0644)

	return
}

// Dateipfad für das laufende OS generieren
func GetPlatformFile(filename string) (osFilePath string) {

	path, file := filepath.Split(filename)
	var newPath = filepath.Dir(path)
	osFilePath = newPath + string(os.PathSeparator) + file

	return
}

func LoadJSONFileToMap(file string) (tmpMap map[string]any, err error) {
	f, err := os.Open(GetPlatformFile(file))
	if err != nil {
		return
	}

	defer func() {
		err = f.Close()
	}()
	if err != nil {
		return
	}

	content, err := io.ReadAll(f)

	if err == nil {
		err = json.Unmarshal([]byte(content), &tmpMap)
	}

	err = f.Close()

	return
}

// Checks whether the file exists in the file system
func CheckFile(filename string) (err error) {
	var file = GetPlatformFile(filename)

	if _, err = os.Stat(file); os.IsNotExist(err) {
		return err
	}

	fi, err := os.Stat(file)
	if err != nil {
		return err
	}

	switch mode := fi.Mode(); {
	case mode.IsDir():
		err = fmt.Errorf("%s: %s", file, cli.GetErrMsg(1072))
	}

	return
}

// Generate folder path for the running OS
func GetPlatformPath(path string) string {
	return filepath.Dir(path) + string(os.PathSeparator)
}
