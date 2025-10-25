package src

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/config"
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

func jsonToMap(content string) map[string]interface{} {

	var tmpMap = make(map[string]interface{})
	err := json.Unmarshal([]byte(content), &tmpMap)
	if err != nil {
		return make(map[string]interface{})
	}

	return (tmpMap)
}

func jsonToInterface(content string) (tmpMap interface{}, err error) {

	err = json.Unmarshal([]byte(content), &tmpMap)
	return

}

func saveMapToJSONFile(file string, tmpMap interface{}) error {

	var filename = storage.GetPlatformFile(file)
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

// Binary
func readByteFromFile(file string) (content []byte, err error) {

	f, err := os.Open(storage.GetPlatformFile(file))
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

func readStringFromFile(file string) (str string, err error) {

	var content []byte
	var filename = storage.GetPlatformFile(file)

	err = storage.CheckFile(filename)
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

// Netzwerk
func resolveHostIP() error {
	interfaces, err := net.Interfaces()
	if err != nil {
		return err
	}

	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return err
		}

		for _, addr := range addrs {
			networkIP, ok := addr.(*net.IPNet)
			config.System.IPAddressesList = append(config.System.IPAddressesList, networkIP.IP.String())

			if ok {
				ip := networkIP.IP.String()

				if networkIP.IP.To4() != nil {
					// Skip unwanted IPs
					if !strings.HasPrefix(ip, "169.254") {
						config.System.IPAddressesV4 = append(config.System.IPAddressesV4, ip)
						config.System.IPAddress = ip
					}
				} else {
					config.System.IPAddressesV6 = append(config.System.IPAddressesV6, ip)
				}
			}
		}
	}

	if len(config.System.IPAddress) == 0 {
		if len(config.System.IPAddressesV4) > 0 {
			config.System.IPAddress = config.System.IPAddressesV4[0]
		} else if len(config.System.IPAddressesV6) > 0 {
			config.System.IPAddress = config.System.IPAddressesV6[0]
		}
	}

	config.System.Hostname, err = os.Hostname()
	if err != nil {
		return err
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

func indexOfFloat64(element float64, data []float64) int {

	for k, v := range data {
		if element == v {
			return (k)
		}
	}

	return -1
}

func getBaseUrl(host string, port string) string {
	if strings.Contains(host, ":") {
		return host
	} else {
		return fmt.Sprintf("%s:%s", host, port)
	}
}
