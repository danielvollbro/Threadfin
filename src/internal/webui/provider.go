package webui

import (
	"os"
	"threadfin/internal/cli"
	"threadfin/internal/config"
	"threadfin/internal/dvr"
	"threadfin/internal/provider"
	"threadfin/internal/settings"
	"threadfin/internal/structs"
	"threadfin/internal/utilities"
	"threadfin/internal/xepg"
)

// Providerdaten speichern (WebUI)
func SaveFiles(request structs.RequestStruct, fileType string) (err error) {

	var filesMap = make(map[string]interface{})
	var newData = make(map[string]interface{})
	var indicator string
	var reloadData = false

	switch fileType {
	case "m3u":
		filesMap = config.Settings.Files.M3U
		newData = request.Files.M3U
		indicator = "M"

	case "hdhr":
		filesMap = config.Settings.Files.HDHR
		newData = request.Files.HDHR
		indicator = "H"

	case "xmltv":
		filesMap = config.Settings.Files.XMLTV
		newData = request.Files.XMLTV
		indicator = "X"
	}

	if len(filesMap) == 0 {
		filesMap = make(map[string]interface{})
	}

	for dataID, data := range newData {

		if dataID == "-" {

			// Neue Providerdatei
			dataID = indicator + utilities.RandomString(19)
			data.(map[string]interface{})["new"] = true
			filesMap[dataID] = data

		} else {

			// Bereits vorhandene Providerdatei
			for key, value := range data.(map[string]interface{}) {

				var oldData = filesMap[dataID].(map[string]interface{})
				oldData[key] = value

			}

		}

		switch fileType {

		case "m3u":
			config.Settings.Files.M3U = filesMap

		case "hdhr":
			config.Settings.Files.HDHR = filesMap

		case "xmltv":
			config.Settings.Files.XMLTV = filesMap

		}

		// Neue Providerdatei
		if _, ok := data.(map[string]interface{})["new"]; ok {

			reloadData = true
			err = provider.GetData(fileType, dataID)
			delete(data.(map[string]interface{}), "new")

			if err != nil {
				delete(filesMap, dataID)
				return
			}

		}

		if _, ok := data.(map[string]interface{})["delete"]; ok {

			deleteLocalProviderFiles(dataID, fileType)
			reloadData = true

		}

		err = settings.SaveSettings(config.Settings)
		if err != nil {
			return
		}

		if reloadData {

			err = dvr.BuildDatabase()
			if err != nil {
				return err
			}

			xepg.BuildXEPG(false)

		}

		config.Settings, _ = settings.Load()
	}

	return
}

// Providerdaten manuell aktualisieren (WebUI)
func UpdateFile(request structs.RequestStruct, fileType string) (err error) {

	var updateData = make(map[string]interface{})

	switch fileType {

	case "m3u":
		updateData = request.Files.M3U

	case "hdhr":
		updateData = request.Files.HDHR

	case "xmltv":
		updateData = request.Files.XMLTV
	}

	for dataID := range updateData {

		err = provider.GetData(fileType, dataID)
		if err == nil {
			// For playlist updates, just update EPG data and Live Event channel names
			xepg.UpdateXEPG(false)
		}

	}

	return
}

// Providerdaten löschen (WebUI)
func deleteLocalProviderFiles(dataID, fileType string) {
	var removeData = make(map[string]interface{})
	var fileExtension string

	switch fileType {

	case "m3u":
		removeData = config.Settings.Files.M3U
		fileExtension = ".m3u"

	case "hdhr":
		removeData = config.Settings.Files.HDHR
		fileExtension = ".json"

	case "xmltv":
		removeData = config.Settings.Files.XMLTV
		fileExtension = ".xml"
	}

	if _, ok := removeData[dataID]; ok {
		delete(removeData, dataID)
		err := os.RemoveAll(config.System.Folder.Data + dataID + fileExtension)
		if err != nil {
			cli.ShowError(err, 0)
		}
	}
}
