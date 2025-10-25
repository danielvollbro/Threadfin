package src

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"threadfin/src/internal/authentication"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/config"
	"threadfin/src/internal/dvr"
	"threadfin/src/internal/imgcache"
	jsonserializer "threadfin/src/internal/json-serializer"
	"threadfin/src/internal/m3u"
	"threadfin/src/internal/provider"
	systemSettings "threadfin/src/internal/settings"
	"threadfin/src/internal/storage"
	"threadfin/src/internal/structs"
	"threadfin/src/internal/utilities"
	"threadfin/src/internal/xepg"
	"threadfin/src/internal/xmltv"
)

// Einstellungen ändern (WebUI)
func updateServerSettings(request structs.RequestStruct) (settings structs.SettingsStruct, err error) {

	var oldSettings = jsonserializer.JSONToMap(jsonserializer.MapToJSON(config.Settings))
	var newSettings = jsonserializer.JSONToMap(jsonserializer.MapToJSON(request.Settings))
	var reloadData = false
	var cacheImages = false
	var createXEPGFiles = false
	var debug string

	// -vvv [URL] --sout '#transcode{vcodec=mp4v, acodec=mpga} :standard{access=http, mux=ogg}'

	for key, value := range newSettings {

		if _, ok := oldSettings[key]; ok {

			switch key {

			case "tuner":
				cli.ShowWarning(2105)

			case "epgSource":
				reloadData = true

			case "update":
				// Leerzeichen aus den Werten entfernen und Formatierung der Uhrzeit überprüfen (0000 - 2359)
				var newUpdateTimes = make([]string, 0)

				for _, v := range value.([]any) {

					v = strings.ReplaceAll(v.(string), " ", "")

					_, err := time.Parse("1504", v.(string))
					if err != nil {
						cli.ShowError(err, 1012)
						return config.Settings, err
					}

					newUpdateTimes = append(newUpdateTimes, v.(string))

				}

				value = newUpdateTimes

			case "cache.images":
				cacheImages = true

			case "xepg.replace.missing.images":
			case "xepg.replace.channel.title":
				createXEPGFiles = true

			case "backup.path":
				value = strings.TrimRight(value.(string), string(os.PathSeparator)) + string(os.PathSeparator)
				err = storage.CheckFolder(value.(string))
				if err == nil {

					err = storage.CheckFilePermission(value.(string))
					if err != nil {
						return
					}

				}

				if err != nil {
					return
				}

			case "temp.path":
				value = strings.TrimRight(value.(string), string(os.PathSeparator)) + string(os.PathSeparator)
				err = storage.CheckFolder(value.(string))
				if err == nil {

					err = storage.CheckFilePermission(value.(string))
					if err != nil {
						return
					}

				}

				if err != nil {
					return
				}

			case "ffmpeg.path", "vlc.path":
				var path = value.(string)
				if len(path) > 0 {

					err = storage.CheckFile(path)
					if err != nil {
						return
					}

				}

			case "scheme.m3u", "scheme.xml":
				createXEPGFiles = true

			}

			oldSettings[key] = value

			switch fmt.Sprintf("%T", value) {

			case "bool":
				debug = fmt.Sprintf("Save Setting:Key: %s | Value: %t (%T)", key, value, value)

			case "string":
				debug = fmt.Sprintf("Save Setting:Key: %s | Value: %s (%T)", key, value, value)

			case "[]interface {}":
				debug = fmt.Sprintf("Save Setting:Key: %s | Value: %v (%T)", key, value, value)

			case "float64":
				debug = fmt.Sprintf("Save Setting:Key: %s | Value: %d (%T)", key, int(value.(float64)), value)

			default:
				debug = fmt.Sprintf("%T", value)
			}

			cli.ShowDebug(debug, 1)

		}

	}

	// Einstellungen aktualisieren
	err = json.Unmarshal([]byte(jsonserializer.MapToJSON(oldSettings)), &config.Settings)
	if err != nil {
		return
	}

	if !config.Settings.AuthenticationWEB {

		config.Settings.AuthenticationAPI = false
		config.Settings.AuthenticationM3U = false
		config.Settings.AuthenticationPMS = false
		config.Settings.AuthenticationWEB = false
		config.Settings.AuthenticationXML = false

	}

	// Buffer Einstellungen überprüfen
	if len(config.Settings.FFmpegOptions) == 0 {
		config.Settings.FFmpegOptions = config.System.FFmpeg.DefaultOptions
	}

	if len(config.Settings.VLCOptions) == 0 {
		config.Settings.VLCOptions = config.System.VLC.DefaultOptions
	}

	switch config.Settings.Buffer {

	case "ffmpeg":

		if len(config.Settings.FFmpegPath) == 0 {
			err = errors.New(cli.GetErrMsg(2020))
			return
		}

	case "vlc":

		if len(config.Settings.VLCPath) == 0 {
			err = errors.New(cli.GetErrMsg(2021))
			return
		}

	}

	err = systemSettings.SaveSettings(config.Settings)
	if err == nil {

		settings = config.Settings

		if reloadData {

			err = dvr.BuildDatabase()
			if err != nil {
				return
			}

			xepg.BuildXEPG(false)

		}

		if cacheImages {

			if config.Settings.EpgSource == "XEPG" && config.System.ImageCachingInProgress == 0 {

				config.Data.Cache.Images, err = imgcache.New(config.System.Folder.ImagesCache, fmt.Sprintf("%s://%s/images/", config.System.ServerProtocol.WEB, config.System.Domain), config.Settings.CacheImages)
				if err != nil {
					cli.ShowError(err, 0)
				}

				switch config.Settings.CacheImages {

				case false:
					err = xmltv.CreateFile()
					if err != nil {
						cli.ShowError(err, 0)
					}
					m3u.CreateFile()

				case true:
					go func() {

						err := xmltv.CreateFile()
						if err != nil {
							cli.ShowError(err, 0)
						}
						m3u.CreateFile()

						config.System.ImageCachingInProgress = 1
						cli.ShowInfo("Image Caching:Images are cached")

						config.Data.Cache.Images.Image.Caching()
						cli.ShowInfo("Image Caching:Done")

						config.System.ImageCachingInProgress = 0

						xepg.BuildXEPG(false)

					}()

				}

			}

		}

		if createXEPGFiles {

			go func() {
				err = xmltv.CreateFile()
				if err != nil {
					cli.ShowError(err, 0)
				}
				m3u.CreateFile()
			}()

		}

	}

	return
}

// Providerdaten speichern (WebUI)
func saveFiles(request structs.RequestStruct, fileType string) (err error) {

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

		err = systemSettings.SaveSettings(config.Settings)
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

		config.Settings, _ = systemSettings.Load()
	}

	return
}

// Providerdaten manuell aktualisieren (WebUI)
func updateFile(request structs.RequestStruct, fileType string) (err error) {

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

// Filtereinstellungen speichern (WebUI)
func saveFilter(request structs.RequestStruct) (settings structs.SettingsStruct, err error) {
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

// XEPG Mapping speichern
func saveXEpgMapping(request structs.RequestStruct) (err error) {

	var tmp = config.Data.XEPG

	config.Data.Cache.StreamingURLS = make(map[string]structs.StreamInfo)

	config.Data.Cache.Images, err = imgcache.New(config.System.Folder.ImagesCache, fmt.Sprintf("%s://%s/images/", config.System.ServerProtocol.WEB, config.System.Domain), config.Settings.CacheImages)
	if err != nil {
		cli.ShowError(err, 0)
	}

	err = json.Unmarshal([]byte(jsonserializer.MapToJSON(request.EpgMapping)), &tmp)
	if err != nil {
		return
	}

	err = storage.SaveMapToJSONFile(config.System.File.XEPG, request.EpgMapping)
	if err != nil {
		return err
	}

	config.Data.XEPG.Channels = request.EpgMapping

	if config.System.ScanInProgress == 0 {

		config.System.ScanInProgress = 1
		err = xmltv.CreateFile()
		if err != nil {
			cli.ShowError(err, 0)
		}
		m3u.CreateFile()
		config.System.ScanInProgress = 0
		cli.ShowInfo("XEPG: Ready to use")

	} else {

		// Wenn während des erstellen der Datanbank das Mapping erneut gespeichert wird, wird die Datenbank erst später erneut aktualisiert.
		go func() {

			if config.System.BackgroundProcess {
				return
			}

			config.System.BackgroundProcess = true

			for {
				time.Sleep(time.Duration(1) * time.Second)
				if config.System.ScanInProgress == 0 {
					break
				}

			}

			config.System.ScanInProgress = 1
			err = xmltv.CreateFile()
			if err != nil {
				cli.ShowError(err, 0)
			}
			m3u.CreateFile()
			config.System.ScanInProgress = 0
			cli.ShowInfo("XEPG: Ready to use")

			config.System.BackgroundProcess = false

		}()

	}

	return
}

// Benutzerdaten speichern (WebUI)
func saveUserData(request structs.RequestStruct) (err error) {

	var userData = request.UserData

	var newCredentials = func(userID string, newUserData map[string]interface{}) (err error) {

		var newUsername, newPassword string
		if username, ok := newUserData["username"].(string); ok {
			newUsername = username
		}

		if password, ok := newUserData["password"].(string); ok {
			newPassword = password
		}

		if len(newUsername) > 0 {
			err = authentication.ChangeCredentials(userID, newUsername, newPassword)
		}

		return
	}

	for userID, newUserData := range userData {

		err = newCredentials(userID, newUserData.(map[string]interface{}))
		if err != nil {
			return
		}

		if request.DeleteUser {
			err = authentication.RemoveUser(userID)
			return
		}

		delete(newUserData.(map[string]interface{}), "password")
		delete(newUserData.(map[string]interface{}), "confirm")

		if _, ok := newUserData.(map[string]interface{})["delete"]; ok {

			err = authentication.RemoveUser(userID)
			if err != nil {
				log.Println("failed to remove user: ", err)
				return
			}

		} else {

			err = authentication.WriteUserData(userID, newUserData.(map[string]interface{}))
			if err != nil {
				return
			}

		}

	}

	return
}

// Neuen Benutzer anlegen (WebUI)
func saveNewUser(request structs.RequestStruct) (err error) {

	var data = request.UserData
	var username = data["username"].(string)
	var password = data["password"].(string)

	delete(data, "password")
	delete(data, "confirm")

	userID, err := authentication.CreateNewUser(username, password)
	if err != nil {
		return
	}

	err = authentication.WriteUserData(userID, data)
	return
}

// Wizard (WebUI)
func saveWizard(request structs.RequestStruct) (nextStep int, err error) {
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

	err = systemSettings.SaveSettings(config.Settings)
	if err != nil {
		return
	}

	return
}
