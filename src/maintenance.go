package src

import (
	"fmt"
	"math/rand"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/config"
	"time"
)

// InitMaintenance : Wartungsprozess initialisieren
func InitMaintenance() (err error) {

	rand.Seed(time.Now().Unix())
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
					err := ThreadfinAutoBackup()
					if err != nil {
						cli.ShowError(err, 000)
					}

					// Playlist und XMLTV Dateien aktualisieren
					getProviderData("m3u", "")
					getProviderData("hdhr", "")

					if config.Settings.EpgSource == "XEPG" {
						getProviderData("xmltv", "")
					}

					// Datenbank für DVR erstellen
					err = buildDatabaseDVR()
					if err != nil {
						cli.ShowError(err, 000)
					}

					config.SystemMutex.Lock()
					if !config.Settings.CacheImages && config.System.ImageCachingInProgress == 0 {
						config.SystemMutex.Unlock()
						removeChildItems(config.System.Folder.ImagesCache)
					} else {
						config.SystemMutex.Unlock()
					}

					// XEPG Dateien erstellen
					buildXEPG(true)

				}

			}

			// Update Threadfin (Binary)
			config.SystemMutex.Lock()
			if config.System.TimeForAutoUpdate == t.Format("1504") {
				config.SystemMutex.Unlock()
				BinaryUpdate()
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
	rand.Seed(time.Now().Unix())
	return rand.Intn(max-min) + min
}
