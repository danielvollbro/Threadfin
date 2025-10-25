package maintenance

import (
	"fmt"
	"math/rand"
	"threadfin/internal/cli"
	"threadfin/internal/config"
	"threadfin/internal/dvr"
	"threadfin/internal/provider"
	"threadfin/internal/storage"
	"threadfin/internal/system"
	"threadfin/internal/update"
	"threadfin/internal/xepg"
	"time"
)

// InitMaintenance : Wartungsprozess initialisieren
func InitMaintenance() (err error) {
	config.System.TimeForAutoUpdate = fmt.Sprintf("0%d%d", randomTime(0, 2), randomTime(10, 59))

	go maintenance()

	return
}

func maintenance() {

	for {

		var t = time.Now()

		// Aktualisierung der Playlist und XMLTV Dateien
		config.SystemMutex.Lock()
		if config.System.ScanInProgress == 0 {
			config.SystemMutex.Unlock()
			for _, schedule := range config.Settings.Update {

				if schedule == t.Format("1504") {

					cli.ShowInfo("Update:" + schedule)

					// Backup erstellen
					err := system.ThreadfinAutoBackup()
					if err != nil {
						cli.ShowError(err, 000)
					}

					// Playlist und XMLTV Dateien aktualisieren
					err = provider.GetData("m3u", "")
					if err != nil {
						cli.ShowError(err, 000)
					}

					err = provider.GetData("hdhr", "")
					if err != nil {
						cli.ShowError(err, 000)
					}

					if config.Settings.EpgSource == "XEPG" {
						err = provider.GetData("xmltv", "")
						if err != nil {
							cli.ShowError(err, 000)
						}
					}

					// Datenbank für DVR erstellen
					err = dvr.BuildDatabase()
					if err != nil {
						cli.ShowError(err, 000)
					}

					config.SystemMutex.Lock()
					if !config.Settings.CacheImages && config.System.ImageCachingInProgress == 0 {
						config.SystemMutex.Unlock()
						err = storage.RemoveChildItems(config.System.Folder.ImagesCache)
						if err != nil {
							cli.ShowError(err, 000)
						}
					} else {
						config.SystemMutex.Unlock()
					}

					// XEPG Dateien erstellen
					xepg.BuildXEPG(true)

				}

			}

			// Update Threadfin (Binary)
			config.SystemMutex.Lock()
			if config.System.TimeForAutoUpdate == t.Format("1504") {
				config.SystemMutex.Unlock()
				err := update.BinaryUpdate()
				if err != nil {
					cli.ShowError(err, 0)
				}
			} else {
				config.SystemMutex.Unlock()
			}

		} else {
			config.SystemMutex.Unlock()
		}

		time.Sleep(60 * time.Second)

	}
}

func randomTime(min, max int) int {
	return rand.Intn(max-min) + min
}
