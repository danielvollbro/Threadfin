package wizard

import (
	"threadfin/internal/cli"
	"threadfin/internal/config"
	"threadfin/internal/dvr"
	jsonserializer "threadfin/internal/json-serializer"
	"threadfin/internal/provider"
	"threadfin/internal/settings"
	"threadfin/internal/structs"
	"threadfin/internal/utilities"
	"threadfin/internal/xepg"
)

// Wizard (WebUI)
func Save(request structs.RequestStruct) (nextStep int, err error) {
	var wizard = jsonserializer.JSONToMap(jsonserializer.MapToJSON(request.Wizard))

	for key, value := range wizard {

		switch key {

		case "tuner":
			config.Settings.Tuner = int(value.(float64))
			nextStep = 1

		case "epgSource":
			config.Settings.EpgSource = value.(string)
			nextStep = 2

		case "m3u", "xmltv":

			var filesMap = make(map[string]interface{})
			var data = make(map[string]interface{})
			var indicator, dataID string

			data["type"] = key
			data["new"] = true

			switch key {

			case "m3u":
				filesMap = config.Settings.Files.M3U
				data["name"] = "M3U"
				indicator = "M"

			case "xmltv":
				filesMap = config.Settings.Files.XMLTV
				data["name"] = "XMLTV"
				indicator = "X"

			}

			dataID = indicator + utilities.RandomString(19)
			data["file.source"] = value.(string)

			filesMap[dataID] = data

			switch key {
			case "m3u":
				config.Settings.Files.M3U = filesMap
				nextStep = 3

				err = provider.GetData(key, dataID)

				if err != nil {
					cli.ShowError(err, 000)
					delete(filesMap, dataID)
					return
				}

				err = dvr.BuildDatabase()
				if err != nil {
					cli.ShowError(err, 000)
					delete(filesMap, dataID)
					return
				}

				if config.Settings.EpgSource == "PMS" {
					nextStep = 10
				}

			case "xmltv":
				config.Settings.Files.XMLTV = filesMap
				nextStep = 10

				err = provider.GetData(key, dataID)

				if err != nil {

					cli.ShowError(err, 000)
					delete(filesMap, dataID)
					return

				}

				xepg.BuildXEPG(false)
				config.System.ScanInProgress = 0

			}

		}

	}

	err = settings.SaveSettings(config.Settings)
	if err != nil {
		return
	}

	return
}
