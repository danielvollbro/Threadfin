package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"threadfin/src/internal/cli"

	"github.com/avfs/avfs"
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

// CheckVFSFolder : Checks whether the Folder exists in provided virtual filesystem, if not, the Folder is created
func CheckVFSFolder(path string, vfs avfs.VFS) (err error) {
	var debug string
	_, err = vfs.Stat(filepath.Dir(path))

	if FSIsNotExistErr(err) {
		// Folder does not exist, will now be created

		// If we are on Windows and the cache location path is NOT on C:\ we need to create the volume it is located on
		// Failure to do so here will result in a panic error and the stream not playing
		vm := vfs.(avfs.VolumeManager)
		if vfs.OSType() == avfs.OsWindows && avfs.VolumeName(vfs, path) != "C:" {
			err = vm.VolumeAdd(path)
			if err != nil {
				return err
			}
		}

		err = vfs.MkdirAll(GetPlatformPath(path), 0755)
		if err == nil {

			debug = fmt.Sprintf("Create virtual filesystem Folder:%s", path)
			cli.ShowDebug(debug, 1)

		} else {
			return err
		}

		return nil
	}

	return nil
}

// FSIsNotExistErr : Returns true whether the <err> is known to report that a file or directory does not exist,
// including virtual file system errors
func FSIsNotExistErr(err error) bool {
	if errors.Is(err, fs.ErrNotExist) ||
		errors.Is(err, avfs.ErrWinPathNotFound) ||
		errors.Is(err, avfs.ErrNoSuchFileOrDir) ||
		errors.Is(err, avfs.ErrWinFileNotFound) {
		return true
	}

	return false
}

func ReadStringFromFile(file string) (str string, err error) {

	var content []byte
	var filename = GetPlatformFile(file)

	err = CheckFile(filename)
	if err != nil {
		return
	}

	content, err = os.ReadFile(filename)
	if err != nil {
		cli.ShowError(err, 0)
		return
	}

	str = string(content)

	return
}

func ReadByteFromFile(file string) (content []byte, err error) {
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

	content, err = io.ReadAll(f)
	err = f.Close()

	return
}

func SaveMapToJSONFile(file string, tmpMap interface{}) error {
	var filename = GetPlatformFile(file)
	jsonString, err := json.MarshalIndent(tmpMap, "", "  ")

	if err != nil {
		return err
	}

	_, err = os.Create(filename)
	if err != nil {
		return err
	}

	err = os.WriteFile(filename, []byte(jsonString), 0644)
	if err != nil {
		return err
	}

	return nil
}

// Checks if the folder exists, if not, the folder is created
func CheckFolder(path string) (err error) {
	var debug string
	_, err = os.Stat(filepath.Dir(path))

	if os.IsNotExist(err) {
		// Ordner existiert nicht, wird jetzt erstellt

		err = os.MkdirAll(GetPlatformPath(path), 0755)
		if err == nil {

			debug = fmt.Sprintf("Create Folder:%s", path)
			cli.ShowDebug(debug, 1)

		} else {
			return err
		}

		return nil
	}

	return nil
}

func GetUserHomeDirectory() (userHomeDirectory string) {
	usr, err := user.Current()

	if err == nil {
		userHomeDirectory = usr.HomeDir
		return
	}

	for _, name := range []string{"HOME", "USERPROFILE"} {
		if dir := os.Getenv(name); dir != "" {
			userHomeDirectory = dir
			break
		}
	}

	return
}

func CheckFilePermission(dir string) (err error) {
	var filename = dir + "permission.test"

	err = os.WriteFile(filename, []byte(""), 0644)
	if err == nil {
		err = os.RemoveAll(filename)
	}

	return
}

func GetFilenameFromPath(path string) (file string) {
	return filepath.Base(path)
}
