package src

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"threadfin/src/internal/authentication"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/config"
	"threadfin/src/internal/imgcache"
	jsonserializer "threadfin/src/internal/json-serializer"
	systemSettings "threadfin/src/internal/settings"
	"threadfin/src/internal/structs"
	"threadfin/src/internal/utilities"
)

// Einstellungen ändern (WebUI)
func updateServerSettings(request structs.RequestStruct) (settings structs.SettingsStruct, err error) {

	var oldSettings = jsonToMap(jsonserializer.MapToJSON(config.Settings))
	var newSettings = jsonToMap(jsonserializer.MapToJSON(request.Settings))
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
				err = checkFolder(value.(string))
				if err == nil {

					err = checkFilePermission(value.(string))
					if err != nil {
						return
					}

				}

				if err != nil {
					return
				}

			case "temp.path":
				value = strings.TrimRight(value.(string), string(os.PathSeparator)) + string(os.PathSeparator)
				err = checkFolder(value.(string))
				if err == nil {

					err = checkFilePermission(value.(string))
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

					err = checkFile(path)
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

			err = buildDatabaseDVR()
			if err != nil {
				return
			}

			buildXEPG(false)

		}

		if cacheImages {

			if config.Settings.EpgSource == "XEPG" && config.System.ImageCachingInProgress == 0 {

				config.Data.Cache.Images, err = imgcache.New(config.System.Folder.ImagesCache, fmt.Sprintf("%s://%s/images/", config.System.ServerProtocol.WEB, config.System.Domain), config.Settings.CacheImages)
				if err != nil {
					cli.ShowError(err, 0)
				}

				switch config.Settings.CacheImages {

				case false:
					err = createXMLTVFile()
					if err != nil {
						cli.ShowError(err, 0)
					}
					createM3UFile()

				case true:
					go func() {

						err := createXMLTVFile()
						if err != nil {
							cli.ShowError(err, 0)
						}
						createM3UFile()

						config.System.ImageCachingInProgress = 1
						cli.ShowInfo("Image Caching:Images are cached")

						config.Data.Cache.Images.Image.Caching()
						cli.ShowInfo("Image Caching:Done")

						config.System.ImageCachingInProgress = 0

						buildXEPG(false)

					}()

				}

			}

		}

		if createXEPGFiles {

			go func() {
				err = createXMLTVFile()
				if err != nil {
					cli.ShowError(err, 0)
				}
				createM3UFile()
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
			err = getProviderData(fileType, dataID)
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

			err = buildDatabaseDVR()
			if err != nil {
				return err
			}

			buildXEPG(false)

		}

		config.Settings, _ = loadSettings()

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

		err = getProviderData(fileType, dataID)
		if err == nil {
			// For playlist updates, just update EPG data and Live Event channel names
			updateXEPG(false)
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
			filterMap[dataID] = jsonToMap(jsonserializer.MapToJSON(newData))
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

	err = buildDatabaseDVR()
	if err != nil {
		return
	}

	buildXEPG(false)

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

	err = saveMapToJSONFile(config.System.File.XEPG, request.EpgMapping)
	if err != nil {
		return err
	}

	config.Data.XEPG.Channels = request.EpgMapping

	if config.System.ScanInProgress == 0 {

		config.System.ScanInProgress = 1
		err = createXMLTVFile()
		if err != nil {
			cli.ShowError(err, 0)
		}
		createM3UFile()
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
			err = createXMLTVFile()
			if err != nil {
				cli.ShowError(err, 0)
			}
			createM3UFile()
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

	var wizard = jsonToMap(jsonserializer.MapToJSON(request.Wizard))

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

				err = getProviderData(key, dataID)

				if err != nil {
					cli.ShowError(err, 000)
					delete(filesMap, dataID)
					return
				}

				err = buildDatabaseDVR()
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

				err = getProviderData(key, dataID)

				if err != nil {

					cli.ShowError(err, 000)
					delete(filesMap, dataID)
					return

				}

				buildXEPG(false)
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

// Filterregeln erstellen
func createFilterRules() (err error) {

	config.Data.Filter = nil
	var dataFilter structs.Filter

	for _, f := range config.Settings.Filter {

		var filter structs.FilterStruct

		var exclude, include string

		err = json.Unmarshal([]byte(jsonserializer.MapToJSON(f)), &filter)
		if err != nil {
			return
		}

		switch filter.Type {

		case "custom-filter":
			dataFilter.CaseSensitive = filter.CaseSensitive
			dataFilter.Rule = filter.Filter
			dataFilter.Type = filter.Type

			config.Data.Filter = append(config.Data.Filter, dataFilter)

		case "group-title":
			if len(filter.Include) > 0 {
				include = fmt.Sprintf(" {%s}", filter.Include)
			}

			if len(filter.Exclude) > 0 {
				exclude = fmt.Sprintf(" !{%s}", filter.Exclude)
			}

			dataFilter.CaseSensitive = filter.CaseSensitive
			dataFilter.LiveEvent = filter.LiveEvent
			dataFilter.Rule = fmt.Sprintf("%s%s%s", filter.Filter, include, exclude)
			dataFilter.Type = filter.Type

			config.Data.Filter = append(config.Data.Filter, dataFilter)
		}

	}

	return
}

// Datenbank für das DVR System erstellen
func buildDatabaseDVR() (err error) {

	config.System.ScanInProgress = 1

	config.Data.Streams.All = make([]interface{}, 0, config.System.UnfilteredChannelLimit)
	config.Data.Streams.Active = make([]interface{}, 0, config.System.UnfilteredChannelLimit)
	config.Data.Streams.Inactive = make([]interface{}, 0, config.System.UnfilteredChannelLimit)
	config.Data.Playlist.M3U.Groups.Text = []string{}
	config.Data.Playlist.M3U.Groups.Value = []string{}
	config.Data.StreamPreviewUI.Active = []string{}
	config.Data.StreamPreviewUI.Inactive = []string{}

	var availableFileTypes = []string{"m3u", "hdhr"}

	var tmpGroupsM3U = make(map[string]int64)

	err = createFilterRules()
	if err != nil {
		return
	}

	for _, fileType := range availableFileTypes {

		var playlistFile = getLocalProviderFiles(fileType)

		for n, i := range playlistFile {

			var channels []interface{}
			var groupTitle, tvgID, uuid = 0, 0, 0
			var keys = []string{"group-title", "tvg-id", "uuid"}
			var compatibility = make(map[string]int)

			var id = strings.TrimSuffix(getFilenameFromPath(i), path.Ext(getFilenameFromPath(i)))
			var playlistName = getProviderParameter(id, fileType, "name")

			switch fileType {

			case "m3u":
				channels, err = parsePlaylist(i, fileType)
			case "hdhr":
				channels, err = parsePlaylist(i, fileType)

			}

			if err != nil {
				cli.ShowError(err, 1005)
				err = errors.New(playlistName + ": Local copy of the file no longer exists")
				cli.ShowError(err, 0)
				playlistFile = append(playlistFile[:n], playlistFile[n+1:]...)
			}

			// Streams analysieren
			for _, stream := range channels {
				var s = stream.(map[string]string)
				s["_file.m3u.path"] = i
				s["_file.m3u.name"] = playlistName
				s["_file.m3u.id"] = id

				// Kompatibilität berechnen
				for _, key := range keys {

					switch key {
					case "uuid":
						if value, ok := s["_uuid.key"]; ok {
							if len(value) > 0 {
								uuid++
							}
						}

					case "group-title":
						if value, ok := s[key]; ok {
							if len(value) > 0 {
								tmpGroupsM3U[value]++
								groupTitle++
							}
						}

					case "tvg-id":
						if value, ok := s[key]; ok {
							if len(value) > 0 {
								tvgID++
							}
						}

					}

				}

				config.Data.Streams.All = append(config.Data.Streams.All, stream)

				// Neuer Filter ab Version 1.3.0
				var preview string
				var status bool

				if config.Settings.IgnoreFilters {
					status = true
				} else {
					var liveEvent bool
					status, liveEvent = filterThisStream(stream)
					s["liveEvent"] = strconv.FormatBool(liveEvent)
				}

				if name, ok := s["name"]; ok {
					var group string

					if v, ok := s["group-title"]; ok {
						group = v
					}

					preview = fmt.Sprintf("%s [%s]", name, group)

				}

				switch status {

				case true:
					config.Data.StreamPreviewUI.Active = append(config.Data.StreamPreviewUI.Active, preview)
					config.Data.Streams.Active = append(config.Data.Streams.Active, stream)

				case false:
					config.Data.StreamPreviewUI.Inactive = append(config.Data.StreamPreviewUI.Inactive, preview)
					config.Data.Streams.Inactive = append(config.Data.Streams.Inactive, stream)

				}

			}

			if tvgID == 0 {
				compatibility["tvg.id"] = 0
			} else {
				compatibility["tvg.id"] = int(tvgID * 100 / len(channels))
			}

			if groupTitle == 0 {
				compatibility["group.title"] = 0
			} else {
				compatibility["group.title"] = int(groupTitle * 100 / len(channels))
			}

			if uuid == 0 {
				compatibility["stream.id"] = 0
			} else {
				compatibility["stream.id"] = int(uuid * 100 / len(channels))
			}

			compatibility["streams"] = len(channels)

			setProviderCompatibility(id, fileType, compatibility)

		}

	}

	for group, count := range tmpGroupsM3U {
		var text = fmt.Sprintf("%s (%d)", group, count)
		config.Data.Playlist.M3U.Groups.Text = append(config.Data.Playlist.M3U.Groups.Text, text)
		config.Data.Playlist.M3U.Groups.Value = append(config.Data.Playlist.M3U.Groups.Value, group)
	}

	sort.Strings(config.Data.Playlist.M3U.Groups.Text)
	sort.Strings(config.Data.Playlist.M3U.Groups.Value)

	if len(config.Data.Streams.Active) == 0 && len(config.Data.Streams.All) <= config.System.UnfilteredChannelLimit && len(config.Settings.Filter) == 0 {
		config.Data.Streams.Active = config.Data.Streams.All
		config.Data.Streams.Inactive = make([]interface{}, 0)

		config.Data.StreamPreviewUI.Active = config.Data.StreamPreviewUI.Inactive
		config.Data.StreamPreviewUI.Inactive = []string{}

	}

	if len(config.Data.Streams.Active) > config.System.PlexChannelLimit {
		cli.ShowWarning(2000)
	}

	if len(config.Settings.Filter) == 0 && len(config.Data.Streams.All) > config.System.UnfilteredChannelLimit {
		cli.ShowWarning(2001)
	}

	config.System.ScanInProgress = 0
	cli.ShowInfo(fmt.Sprintf("All streams:%d", len(config.Data.Streams.All)))
	cli.ShowInfo(fmt.Sprintf("Active streams:%d", len(config.Data.Streams.Active)))
	cli.ShowInfo(fmt.Sprintf("Filter:%d", len(config.Data.Filter)))

	sort.Strings(config.Data.StreamPreviewUI.Active)
	sort.Strings(config.Data.StreamPreviewUI.Inactive)

	return
}

// Speicherort aller lokalen Providerdateien laden, immer für eine Dateityp (M3U, XMLTV usw.)
func getLocalProviderFiles(fileType string) (localFiles []string) {

	var fileExtension string
	var dataMap = make(map[string]interface{})

	switch fileType {

	case "m3u":
		fileExtension = ".m3u"
		dataMap = config.Settings.Files.M3U

	case "hdhr":
		fileExtension = ".json"
		dataMap = config.Settings.Files.HDHR

	case "xmltv":
		fileExtension = ".xml"
		dataMap = config.Settings.Files.XMLTV

	}

	for dataID := range dataMap {
		localFiles = append(localFiles, config.System.Folder.Data+dataID+fileExtension)
	}

	return
}

// Providerparameter anhand von dem Key ausgeben
func getProviderParameter(id, fileType, key string) (s string) {

	var dataMap = make(map[string]interface{})

	switch fileType {
	case "m3u":
		dataMap = config.Settings.Files.M3U

	case "hdhr":
		dataMap = config.Settings.Files.HDHR

	case "xmltv":
		dataMap = config.Settings.Files.XMLTV
	}

	if data, ok := dataMap[id].(map[string]interface{}); ok {

		if v, ok := data[key].(string); ok {
			s = v
		}

		if v, ok := data[key].(float64); ok {
			s = strconv.Itoa(int(v))
		}

	}

	return
}

// Provider Statistiken Kompatibilität aktualisieren
func setProviderCompatibility(id, fileType string, compatibility map[string]int) {

	var dataMap = make(map[string]interface{})

	switch fileType {
	case "m3u":
		dataMap = config.Settings.Files.M3U

	case "hdhr":
		dataMap = config.Settings.Files.HDHR

	case "xmltv":
		dataMap = config.Settings.Files.XMLTV
	}

	if data, ok := dataMap[id].(map[string]interface{}); ok {

		data["compatibility"] = compatibility

		switch fileType {
		case "m3u":
			config.Settings.Files.M3U = dataMap
		case "hdhr":
			config.Settings.Files.HDHR = dataMap
		case "xmltv":
			config.Settings.Files.XMLTV = dataMap
		}

		err := systemSettings.SaveSettings(config.Settings)
		if err != nil {
			cli.ShowError(err, 0)
		}
	}

}
