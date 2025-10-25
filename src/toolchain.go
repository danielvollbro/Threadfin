package src

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/storage"
)

// --- System Tools ---

// Checks if the folder exists, if not, the folder is created
func checkFolder(path string) (err error) {

	var debug string
	_, err = os.Stat(filepath.Dir(path))

	if os.IsNotExist(err) {
		// Ordner existiert nicht, wird jetzt erstellt

		err = os.MkdirAll(storage.GetPlatformPath(path), 0755)
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

// GetUserHomeDirectory : Benutzer Homer Verzeichnis
func GetUserHomeDirectory() (userHomeDirectory string) {

	usr, err := user.Current()

	if err != nil {

		for _, name := range []string{"HOME", "USERPROFILE"} {

			if dir := os.Getenv(name); dir != "" {
				userHomeDirectory = dir
				break
			}

		}

	} else {
		userHomeDirectory = usr.HomeDir
	}

	return
}

// Prüft Dateiberechtigung
func checkFilePermission(dir string) (err error) {

	var filename = dir + "permission.test"

	err = os.WriteFile(filename, []byte(""), 0644)
	if err == nil {
		err = os.RemoveAll(filename)
	}

	return
}

// Dateinamen aus dem Dateipfad ausgeben
func getFilenameFromPath(path string) (file string) {
	return filepath.Base(path)
}

// Sucht eine Datei im OS
func searchFileInOS(file string) (path string) {

	switch runtime.GOOS {

	case "linux", "darwin", "freebsd":
		var args = file
		var cmd = exec.Command("which", strings.Split(args, " ")...)

		out, err := cmd.CombinedOutput()
		if err == nil {

			var slice = strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")

			if len(slice) > 0 {
				path = strings.Trim(slice[0], "\r\n")
			}

		}

	default:
		return

	}

	return
}

func removeChildItems(dir string) error {

	files, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		return err
	}

	for _, file := range files {

		err = os.RemoveAll(file)
		if err != nil {
			return err
		}

	}

	return nil
}

func parseTemplate(content string, tmpMap map[string]interface{}) (result string) {

	t := template.Must(template.New("template").Parse(content))

	var tpl bytes.Buffer

	if err := t.Execute(&tpl, tmpMap); err != nil {
		cli.ShowError(err, 0)
	}
	result = tpl.String()

	return
}

func getBaseUrl(host string, port string) string {
	if strings.Contains(host, ":") {
		return host
	} else {
		return fmt.Sprintf("%s:%s", host, port)
	}
}
