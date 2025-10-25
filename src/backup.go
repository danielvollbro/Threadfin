package src

import (
	b64 "encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/config"
	"threadfin/src/internal/storage"
	"time"
)

func ThreadfinAutoBackup() (err error) {

	var archiv = "threadfin_auto_backup_" + time.Now().Format("20060102_1504") + ".zip"
	var target string
	var sourceFiles = make([]string, 0)
	var oldBackupFiles = make([]string, 0)
	var debug string

	if len(config.Settings.BackupPath) > 0 {
		config.System.Folder.Backup = config.Settings.BackupPath
	}

	cli.ShowInfo("Backup Path:" + config.System.Folder.Backup)

	err = checkFolder(config.System.Folder.Backup)
	if err != nil {
		cli.ShowError(err, 1070)
		return
	}

	// Alte Backups löschen
	files, err := os.ReadDir(config.System.Folder.Backup)

	if err == nil {

		for _, file := range files {

			if filepath.Ext(file.Name()) == ".zip" && strings.Contains(file.Name(), "threadfin_auto_backup") {
				oldBackupFiles = append(oldBackupFiles, file.Name())
			}

		}

		// Alle Backups löschen
		var end int
		switch config.Settings.BackupKeep {
		case 0:
			end = 0
		default:
			end = config.Settings.BackupKeep - 1
		}

		for i := 0; i < len(oldBackupFiles)-end; i++ {

			err = os.RemoveAll(config.System.Folder.Backup + oldBackupFiles[i])
			if err != nil {
				cli.ShowError(err, 0)
			}

			debug = fmt.Sprintf("Delete backup file:%s", oldBackupFiles[i])
			cli.ShowDebug(debug, 1)
		}

		if config.Settings.BackupKeep == 0 {
			return
		}

	} else {

		return

	}

	// Backup erstellen
	if err == nil {

		target = config.System.Folder.Backup + archiv

		for _, i := range config.SystemFiles {
			sourceFiles = append(sourceFiles, config.System.Folder.Config+i)
		}

		sourceFiles = append(sourceFiles, config.System.Folder.ImagesUpload)

		err = zipFiles(sourceFiles, target)

		if err == nil {

			debug = fmt.Sprintf("Create backup file:%s", target)
			cli.ShowDebug(debug, 1)

			cli.ShowInfo("Backup file:" + target)

		}

	}

	return
}

func ThreadfinBackup() (archiv string, err error) {

	err = checkFolder(config.System.Folder.Temp)
	if err != nil {
		return
	}

	archiv = "threadfin_backup_" + time.Now().Format("20060102_1504") + ".zip"

	var target = config.System.Folder.Temp + archiv
	var sourceFiles = make([]string, 0)

	for _, i := range config.SystemFiles {
		sourceFiles = append(sourceFiles, config.System.Folder.Config+i)
	}

	sourceFiles = append(sourceFiles, config.System.Folder.Data)

	err = zipFiles(sourceFiles, target)
	if err != nil {
		cli.ShowError(err, 0)
		return
	}

	return
}

func ThreadfinRestore(archive string) (newWebURL string, err error) {

	var newPort, oldPort, backupVersion, tmpRestore string

	tmpRestore = config.System.Folder.Temp + "restore" + string(os.PathSeparator)

	err = checkFolder(tmpRestore)
	if err != nil {
		return
	}

	// Zip Archiv in tmp entpacken
	err = extractZIP(archive, tmpRestore)
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
	err = extractZIP(archive, config.System.Folder.Config)
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
		_, err = loadSettings()
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

func ThreadfinRestoreFromWeb(input string) (newWebURL string, err error) {

	// Base64 Json String in base64 umwandeln
	b64data := input[strings.IndexByte(input, ',')+1:]

	// Base64 in bytes umwandeln und speichern
	sDec, err := b64.StdEncoding.DecodeString(b64data)

	if err != nil {
		return
	}

	var archive = config.System.Folder.Temp + "restore.zip"

	err = storage.WriteByteToFile(archive, sDec)
	if err != nil {
		return
	}

	newWebURL, err = ThreadfinRestore(archive)

	return
}

// ThreadfinRestoreFromCLI : Wiederherstellung über die Kommandozeile
func ThreadfinRestoreFromCLI(archive string) (err error) {

	var confirm string

	println()
	cli.ShowInfo(fmt.Sprintf("Version:%s Build: %s", config.System.Version, config.System.Build))
	cli.ShowInfo(fmt.Sprintf("Backup File:%s", archive))
	cli.ShowInfo(fmt.Sprintf("System Folder:%s", storage.GetPlatformPath(config.System.Folder.Config)))
	println()

	fmt.Print("All data will be replaced with those from the backup. Should the files be restored? [yes|no]:")

	_, err = fmt.Scanln(&confirm)
	if err != nil {
		cli.ShowError(err, 500)
		return
	}

	switch strings.ToLower(confirm) {

	case "yes":
		break

	case "no":
		return

	default:
		fmt.Println("Invalid input")
		return

	}

	if len(config.System.Folder.Config) > 0 {

		err = checkFilePermission(config.System.Folder.Config)
		if err != nil {
			return
		}

		_, err = ThreadfinRestore(archive)
		if err != nil {
			return
		}

		cli.ShowHighlight(fmt.Sprintf("Restor:Backup was successfully restored. %s can now be started normally", config.System.Name))

	}
	return
}
