package system

import (
	"errors"
	"os"
	"strings"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/compression"
	"threadfin/src/internal/config"
	"threadfin/src/internal/settings"
	"threadfin/src/internal/storage"
)

func ThreadfinRestore(archive string) (newWebURL string, err error) {

	var newPort, oldPort, backupVersion, tmpRestore string

	tmpRestore = config.System.Folder.Temp + "restore" + string(os.PathSeparator)

	err = storage.CheckFolder(tmpRestore)
	if err != nil {
		return
	}

	// Zip Archiv in tmp entpacken
	err = compression.ExtractZIP(archive, tmpRestore)
	if err != nil {
		return
	}

	// Neue Config laden um den Port und die Version zu überprüfen
	newConfig, err := storage.LoadJSONFileToMap(tmpRestore + "settings.json")
	if err != nil {
		cli.ShowError(err, 0)
		return
	}

	backupVersion = newConfig["version"].(string)
	if backupVersion < config.System.Compatibility {
		err = errors.New(cli.GetErrMsg(1013))
		return
	}

	// Zip Archiv in den Config Ordner entpacken
	err = compression.ExtractZIP(archive, config.System.Folder.Config)
	if err != nil {
		return
	}

	// Neue Config laden um den Port und die Version zu überprüfen
	newConfig, err = storage.LoadJSONFileToMap(config.System.Folder.Config + "settings.json")
	if err != nil {
		cli.ShowError(err, 0)
		return
	}

	newPort = newConfig["port"].(string)
	oldPort = config.Settings.Port

	if newPort == oldPort {
		_, err = settings.Load()
		if err != nil {
			cli.ShowError(err, 0)
			return
		}

		err := Init()
		if err != nil {
			cli.ShowError(err, 0)
			return "", err
		}

		err = StartSystem(true)
		if err != nil {
			cli.ShowError(err, 0)
			return "", err
		}

		return "", err
	}

	var url = config.System.URLBase + "/web/"
	newWebURL = strings.Replace(url, ":"+oldPort, ":"+newPort, 1)

	err = os.RemoveAll(tmpRestore)

	return
}
