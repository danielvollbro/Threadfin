package settings

import (
	"os"
	"threadfin/src/internal/config"
	jsonserializer "threadfin/src/internal/json-serializer"
	"threadfin/src/internal/plex"
	"threadfin/src/internal/storage"
	"threadfin/src/internal/structs"
)

// Einstellungen speichern (Threadfin)
func SaveSettings(settings structs.SettingsStruct) (err error) {

	if settings.BackupKeep == 0 {
		settings.BackupKeep = 10
	}

	if len(settings.BackupPath) == 0 {
		settings.BackupPath = config.System.Folder.Backup
	}

	if settings.BufferTimeout < 0 {
		settings.BufferTimeout = 0
	}

	config.System.Folder.Temp = settings.TempPath + settings.UUID + string(os.PathSeparator)

	err = storage.WriteByteToFile(config.System.File.Settings, []byte(jsonserializer.MapToJSON(settings)))
	if err != nil {
		return
	}

	config.Settings = settings

	if config.System.Dev {
		config.Settings.UUID = "2019-01-DEV-Threadfin!"
	}

	plex.SetDeviceID()

	return
}
