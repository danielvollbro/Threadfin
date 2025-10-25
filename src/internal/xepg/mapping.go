package xepg

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"path"
	"strings"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/config"
	"threadfin/src/internal/imgcache"
	jsonserializer "threadfin/src/internal/json-serializer"
	"threadfin/src/internal/m3u"
	"threadfin/src/internal/provider"
	"threadfin/src/internal/storage"
	"threadfin/src/internal/structs"
	"threadfin/src/internal/xmltv"
	"time"
)

// Mapping Menü für die XMLTV Dateien erstellen
func createXEPGMapping() {
	config.Data.XMLTV.Files = provider.GetLocalFiles("xmltv")
	config.Data.XMLTV.Mapping = make(map[string]interface{})

	var tmpMap = make(map[string]interface{})

	var friendlyDisplayName = func(channel structs.Channel) (displayName string) {
		var dn = channel.DisplayName
		if len(dn) > 0 {
			switch len(dn) {
			case 1:
				displayName = dn[0].Value
			default:
				displayName = fmt.Sprintf("%s (%s)", dn[0].Value, dn[1].Value)
			}
		}

		return
	}

	if len(config.Data.XMLTV.Files) > 0 {

		// For multiple large files, process in parallel for better performance
		if len(config.Data.XMLTV.Files) > 1 {
			cli.ShowInfo("XEPG:" + fmt.Sprintf("Processing %d XMLTV files in parallel", len(config.Data.XMLTV.Files)))
		}

		for i := len(config.Data.XMLTV.Files) - 1; i >= 0; i-- {

			var file = config.Data.XMLTV.Files[i]

			var err error
			var fileID = strings.TrimSuffix(storage.GetFilenameFromPath(file), path.Ext(storage.GetFilenameFromPath(file)))
			cli.ShowInfo("XEPG:" + "Parse XMLTV file: " + provider.GetProviderParameter(fileID, "xmltv", "name"))

			//xmltv, err = getLocalXMLTV(file)
			var xmltvStruct structs.XMLTV
			err = xmltv.GetLocal(file, &xmltvStruct)
			if err != nil {
				config.Data.XMLTV.Files = append(config.Data.XMLTV.Files, config.Data.XMLTV.Files[i+1:]...)
				var errMsg = err.Error()
				err = errors.New(provider.GetProviderParameter(fileID, "xmltv", "name") + ": " + errMsg)
				cli.ShowError(err, 000)
			}

			// XML Parsen (Provider Datei)
			if err == nil {
				var imgc = config.Data.Cache.Images
				// Daten aus der XML Datei in eine temporäre Map schreiben
				var xmltvMap = make(map[string]interface{}, len(xmltvStruct.Channel)) // Pre-allocate

				for _, c := range xmltvStruct.Channel {
					var channel = make(map[string]interface{}, 4) // Pre-allocate

					channel["id"] = c.ID
					channel["display-name"] = friendlyDisplayName(*c)
					channel["icon"] = imgc.Image.GetURL(c.Icon.Src, config.Settings.HttpThreadfinDomain, config.Settings.Port, config.Settings.ForceHttps, config.Settings.HttpsPort, config.Settings.HttpsThreadfinDomain)
					channel["active"] = c.Active

					xmltvMap[c.ID] = channel

				}

				tmpMap[storage.GetFilenameFromPath(file)] = xmltvMap
				config.Data.XMLTV.Mapping[storage.GetFilenameFromPath(file)] = xmltvMap

			}

		}

		config.Data.XMLTV.Mapping = tmpMap
	} else {

		if !config.System.ConfigurationWizard {
			cli.ShowWarning(1007)
		}

	}

	// Auswahl für den Dummy erstellen
	var dummy = make(map[string]any)
	var times = []string{"30", "60", "90", "120", "180", "240", "360", "PPV"}

	for _, i := range times {

		var dummyChannel = make(map[string]string)
		if i == "PPV" {
			dummyChannel["display-name"] = "PPV Event"
			dummyChannel["id"] = "PPV"
		} else {
			dummyChannel["display-name"] = i + " Minutes"
			dummyChannel["id"] = i + "_Minutes"
		}
		dummyChannel["icon"] = ""

		dummy[dummyChannel["id"]] = dummyChannel

	}

	config.Data.XMLTV.Mapping["Threadfin Dummy"] = dummy
}

// Kanäle automatisch zuordnen und das Mapping überprüfen
func mapping() (err error) {
	cli.ShowInfo("XEPG:" + "Map channels")

	for xepg, dxc := range config.Data.XEPG.Channels {

		var xepgChannel structs.XEPGChannelStruct
		err = json.Unmarshal([]byte(jsonserializer.MapToJSON(dxc)), &xepgChannel)
		if err != nil {
			return
		}

		if xepgChannel.TvgName == "" {
			xepgChannel.TvgName = xepgChannel.Name
		}

		if (xepgChannel.XBackupChannel1 != "" && xepgChannel.XBackupChannel1 != "-") || (xepgChannel.XBackupChannel2 != "" && xepgChannel.XBackupChannel2 != "-") || (xepgChannel.XBackupChannel3 != "" && xepgChannel.XBackupChannel3 != "-") {
			for _, stream := range config.Data.Streams.Active {
				var m3uChannel structs.M3UChannelStructXEPG

				err = json.Unmarshal([]byte(jsonserializer.MapToJSON(stream)), &m3uChannel)
				if err != nil {
					return err
				}

				if m3uChannel.TvgName == "" {
					m3uChannel.TvgName = m3uChannel.Name
				}

				backup_channel1 := strings.Trim(xepgChannel.XBackupChannel1, " ")
				if m3uChannel.TvgName == backup_channel1 {
					xepgChannel.BackupChannel1 = &structs.BackupStream{PlaylistID: m3uChannel.FileM3UID, URL: m3uChannel.URL}
				}

				backup_channel2 := strings.Trim(xepgChannel.XBackupChannel2, " ")
				if m3uChannel.TvgName == backup_channel2 {
					xepgChannel.BackupChannel2 = &structs.BackupStream{PlaylistID: m3uChannel.FileM3UID, URL: m3uChannel.URL}
				}

				backup_channel3 := strings.Trim(xepgChannel.XBackupChannel3, " ")
				if m3uChannel.TvgName == backup_channel3 {
					xepgChannel.BackupChannel3 = &structs.BackupStream{PlaylistID: m3uChannel.FileM3UID, URL: m3uChannel.URL}
				}
			}
		}

		// Automatische Mapping für neue Kanäle. Wird nur ausgeführt, wenn der Kanal deaktiviert ist und keine XMLTV Datei und kein XMLTV Kanal zugeordnet ist.
		if !xepgChannel.XActive {
			// Werte kann "-" sein, deswegen len < 1
			if len(xepgChannel.XmltvFile) < 1 {

				var tvgID = xepgChannel.TvgID

				xepgChannel.XmltvFile = "-"
				xepgChannel.XMapping = "-"

				config.Data.XEPG.Channels[xepg] = xepgChannel
				for file, xmltvChannels := range config.Data.XMLTV.Mapping {
					channelsMap, ok := xmltvChannels.(map[string]interface{})
					if !ok {
						continue
					}
					if channel, ok := channelsMap[tvgID]; ok {

						filters := []structs.FilterStruct{}
						for _, filter := range config.Settings.Filter {
							filter_json, _ := json.Marshal(filter)
							f := structs.FilterStruct{}
							err = json.Unmarshal(filter_json, &f)
							if err != nil {
								log.Println("XEPG:mapping:Error unmarshalling filter:", err)
								return
							}

							filters = append(filters, f)
						}
						for _, filter := range filters {
							if xepgChannel.GroupTitle == filter.Filter {
								category := &structs.Category{}
								category.Value = filter.Category
								category.Lang = "en"
								xepgChannel.XCategory = filter.Category
							}
						}

						chmap, okk := channel.(map[string]interface{})
						if !okk {
							continue
						}

						if channelID, ok := chmap["id"].(string); ok {
							xepgChannel.XmltvFile = file
							xepgChannel.XMapping = channelID

							// Falls in der XMLTV Datei ein Logo existiert, wird dieses verwendet. Falls nicht, dann das Logo aus der M3U Datei
							/*if icon, ok := chmap["icon"].(string); ok {
								if len(icon) > 0 {
									xepgChannel.TvgLogo = icon
								}
							}*/

							config.Data.XEPG.Channels[xepg] = xepgChannel
							break

						}

					}

				}
			}
		}

		// Überprüfen, ob die zugeordneten XMLTV Dateien und Kanäle noch existieren.
		if xepgChannel.XActive && !xepgChannel.XHideChannel {

			var mapping = xepgChannel.XMapping
			var file = xepgChannel.XmltvFile

			if file != "Threadfin Dummy" && !xepgChannel.Live {

				if value, ok := config.Data.XMLTV.Mapping[file].(map[string]interface{}); ok {

					if _, ok := value[mapping].(map[string]interface{}); ok {

						filters := []structs.FilterStruct{}
						for _, filter := range config.Settings.Filter {
							filter_json, _ := json.Marshal(filter)
							f := structs.FilterStruct{}
							err = json.Unmarshal(filter_json, &f)
							if err != nil {
								log.Println("XEPG:mapping:Error unmarshalling filter:", err)
								return
							}
							filters = append(filters, f)
						}
						for _, filter := range filters {
							if xepgChannel.GroupTitle == filter.Filter {
								category := &structs.Category{}
								category.Value = filter.Category
								category.Lang = "en"
								if xepgChannel.XCategory == "" {
									xepgChannel.XCategory = filter.Category
								}
							}
						}
					}

				}

			} else {
				// Loop through dummy channels and assign the filter info
				filters := []structs.FilterStruct{}
				for _, filter := range config.Settings.Filter {
					filter_json, _ := json.Marshal(filter)
					f := structs.FilterStruct{}
					err = json.Unmarshal(filter_json, &f)
					if err != nil {
						log.Println("XEPG:mapping:Error unmarshalling filter:", err)
						return
					}
					filters = append(filters, f)
				}
				for _, filter := range filters {
					if xepgChannel.GroupTitle == filter.Filter {
						category := &structs.Category{}
						category.Value = filter.Category
						category.Lang = "en"
						if xepgChannel.XCategory == "" {
							xepgChannel.XCategory = filter.Category
						}
					}
				}
			}
			if len(xepgChannel.XmltvFile) == 0 {
				xepgChannel.XmltvFile = "-"
			}

			if len(xepgChannel.XMapping) == 0 {
				xepgChannel.XMapping = "-"
			}

			config.Data.XEPG.Channels[xepg] = xepgChannel

		}

	}

	err = storage.SaveMapToJSONFile(config.System.File.XEPG, config.Data.XEPG.Channels)
	if err != nil {
		cli.ShowError(err, 000)
		return
	}

	return
}

// XEPG Mapping speichern
func SaveXEpgMapping(request structs.RequestStruct) (err error) {

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
