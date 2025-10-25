package storage

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
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
