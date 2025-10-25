package storage

import (
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
