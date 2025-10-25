package webui

import (
	"errors"
	"threadfin/internal/cli"
	"threadfin/internal/config"
	"threadfin/internal/dvr"
	jsonserializer "threadfin/internal/json-serializer"
	systemSettings "threadfin/internal/settings"
	"threadfin/internal/structs"
	"threadfin/internal/xepg"
)

// Filtereinstellungen speichern (WebUI)
func SaveFilter(request structs.RequestStruct) (settings structs.SettingsStruct, err error) {
	var defaultFilter structs.FilterStruct
	var newFilter = false

	defaultFilter.Active = true
	defaultFilter.CaseSensitive = false
	defaultFilter.LiveEvent = false

	var filterMap = config.Settings.Filter
	var newData = request.Filter
	var createNewID = func() (id int64) {

	newID:
		if _, ok := filterMap[id]; ok {
			id++
			goto newID
		}

		return id
	}

	for dataID, data := range newData {
		if dataID == -1 {

			// New Filter
			newFilter = true
			dataID = createNewID()
			filterMap[dataID] = jsonserializer.JSONToMap(jsonserializer.MapToJSON(newData))
		}

		// Update / delete filters
		for key, value := range data.(map[string]interface{}) {

			// Clear Filters
			if _, ok := data.(map[string]interface{})["delete"]; ok {
				delete(filterMap, dataID)
				break
			}

			if filter, ok := data.(map[string]interface{})["filter"].(string); ok {

				if len(filter) == 0 {

					err = errors.New(cli.GetErrMsg(1014))
					if newFilter {
						delete(filterMap, dataID)
					}

					return
				}

			}

			if oldData, ok := filterMap[dataID].(map[string]interface{}); ok {
				oldData[key] = value
			}

		}

	}

	err = systemSettings.SaveSettings(config.Settings)
	if err != nil {
		return
	}

	settings = config.Settings

	err = dvr.BuildDatabase()
	if err != nil {
		return
	}

	xepg.BuildXEPG(false)

	return
}
