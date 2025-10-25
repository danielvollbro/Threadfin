package src

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"path"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"threadfin/src/internal/channels"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/config"
	"threadfin/src/internal/imgcache"
	jsonserializer "threadfin/src/internal/json-serializer"
	"threadfin/src/internal/m3u"
	"threadfin/src/internal/provider"
	"threadfin/src/internal/storage"
	"threadfin/src/internal/structs"
	"threadfin/src/internal/utilities"
	"threadfin/src/internal/xepg"
	"threadfin/src/internal/xmltv"
	_ "time/tzdata"
)

// XEPG Daten erstellen
func buildXEPG(background bool) {
	config.XepgMutex.Lock()
	defer func() {
		config.XepgMutex.Unlock()
	}()

	if config.System.ScanInProgress == 1 {
		return
	}

	config.System.ScanInProgress = 1
	// Enter maintenance during core steps

	// Clear streaming URL cache
	config.Data.Cache.StreamingURLS = make(map[string]structs.StreamInfo)
	err := storage.SaveMapToJSONFile(config.System.File.URLS, config.Data.Cache.StreamingURLS)
	if err != nil {
		cli.ShowError(err, 000)
		return
	}

	config.Data.Cache.Images, err = imgcache.New(config.System.Folder.ImagesCache, fmt.Sprintf("%s://%s/images/", config.System.ServerProtocol.WEB, config.System.Domain), config.Settings.CacheImages)
	if err != nil {
		cli.ShowError(err, 0)
	}

	if config.Settings.EpgSource == "XEPG" {

		switch background {

		case true:

			go func() {

				createXEPGMapping()
				err = createXEPGDatabase()
				if err != nil {
					cli.ShowError(err, 000)
					return
				}

				err = mapping()
				if err != nil {
					cli.ShowError(err, 000)
					return
				}

				xepg.Cleanup()
				err = xmltv.CreateFile()
				if err != nil {
					cli.ShowError(err, 000)
					return
				}

				m3u.CreateFile()

				cli.ShowInfo("XEPG: Ready to use")

				if config.Settings.CacheImages && config.System.ImageCachingInProgress == 0 {

					go func() {

						config.SystemMutex.Lock()
						config.System.ImageCachingInProgress = 1
						config.SystemMutex.Unlock()

						cli.ShowInfo(fmt.Sprintf("Image Caching:Images are cached (%d)", len(config.Data.Cache.Images.Queue)))

						config.Data.Cache.Images.Image.Caching()
						config.Data.Cache.Images.Image.Remove()
						cli.ShowInfo("Image Caching:Done")

						err = xmltv.CreateFile()
						if err != nil {
							cli.ShowError(err, 000)
							return
						}
						m3u.CreateFile()

						config.SystemMutex.Lock()
						config.System.ImageCachingInProgress = 0
						config.SystemMutex.Unlock()

					}()

				}

				// Core work is done; exit maintenance
				config.SystemMutex.Lock()
				config.System.ScanInProgress = 0
				config.SystemMutex.Unlock()

				// Cache löschen
				config.Data.Cache.XMLTV = make(map[string]structs.XMLTV)
				runtime.GC()

			}()

		case false:

			createXEPGMapping()
			err = createXEPGDatabase()
			if err != nil {
				cli.ShowError(err, 000)
				return
			}

			err = mapping()
			if err != nil {
				cli.ShowError(err, 000)
				return
			}

			xepg.Cleanup()
			err = xmltv.CreateFile()
			if err != nil {
				cli.ShowError(err, 000)
				return
			}

			m3u.CreateFile()

			// Exit maintenance before long file generation to keep UI responsive
			config.System.ScanInProgress = 0

			go func() {

				if config.Settings.CacheImages && config.System.ImageCachingInProgress == 0 {

					go func() {

						config.SystemMutex.Lock()
						config.System.ImageCachingInProgress = 1
						config.SystemMutex.Unlock()

						cli.ShowInfo(fmt.Sprintf("Image Caching:Images are cached (%d)", len(config.Data.Cache.Images.Queue)))

						config.Data.Cache.Images.Image.Caching()
						config.Data.Cache.Images.Image.Remove()
						cli.ShowInfo("Image Caching:Done")

						err = xmltv.CreateFile()
						if err != nil {
							cli.ShowError(err, 000)
							return
						}
						m3u.CreateFile()

						config.SystemMutex.Lock()
						config.System.ImageCachingInProgress = 0
						config.SystemMutex.Unlock()

					}()

				}

				cli.ShowInfo("XEPG: Ready to use")

				// Cache löschen
				config.Data.Cache.XMLTV = make(map[string]structs.XMLTV)
				runtime.GC()

			}()

		}

	} else {

		_, err = getLineup()
		if err != nil {
			cli.ShowError(err, 000)
		}

		config.System.ScanInProgress = 0

	}

}

// Update XEPG data
func updateXEPG(background bool) {

	if config.System.ScanInProgress == 1 {
		return
	}

	config.System.ScanInProgress = 1

	if config.Settings.EpgSource == "XEPG" {

		switch background {

		case false:

			err := createXEPGDatabase()
			if err != nil {
				cli.ShowError(err, 000)
				return
			}

			err = mapping()
			if err != nil {
				cli.ShowError(err, 000)
				return
			}

			xepg.Cleanup()

			// Exit maintenance before long file generation to keep UI responsive
			config.System.ScanInProgress = 0

			go func() {

				err = xmltv.CreateFile()
				if err != nil {
					cli.ShowError(err, 000)
					return
				}
				m3u.CreateFile()
				cli.ShowInfo("XEPG: Ready to use")

			}()

		case true:
			config.System.ScanInProgress = 0

		}

	} else {

		config.System.ScanInProgress = 0

	}

	// Cache löschen
	config.Data.Cache.XMLTV = make(map[string]structs.XMLTV)
}

// Mapping Menü für die XMLTV Dateien erstellen
func createXEPGMapping() {
	config.Data.XMLTV.Files = getLocalProviderFiles("xmltv")
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

// XEPG Datenbank erstellen / aktualisieren
func createXEPGDatabase() (err error) {

	var allChannelNumbers = make([]float64, 0, config.System.UnfilteredChannelLimit)
	config.Data.Cache.Streams.Active = make([]string, 0, config.System.UnfilteredChannelLimit)
	config.Data.XEPG.Channels = make(map[string]interface{}, config.System.UnfilteredChannelLimit)

	// Clear streaming URL cache
	config.Data.Cache.StreamingURLS = make(map[string]structs.StreamInfo)
	err = storage.SaveMapToJSONFile(config.System.File.URLS, config.Data.Cache.StreamingURLS)
	if err != nil {
		cli.ShowError(err, 000)
		return err
	}

	config.Data.Cache.Streams.Active = make([]string, 0, config.System.UnfilteredChannelLimit)
	config.Settings = structs.SettingsStruct{}
	config.Data.XEPG.Channels, err = storage.LoadJSONFileToMap(config.System.File.XEPG)
	if err != nil {
		cli.ShowError(err, 1004)
		return err
	}

	settings, err := storage.LoadJSONFileToMap(config.System.File.Settings)
	if err != nil || len(settings) == 0 {
		return
	}

	settings_json, err := json.Marshal(settings)
	if err != nil {
		cli.ShowError(err, 000)
		return
	}

	err = json.Unmarshal(settings_json, &config.Settings)
	if err != nil {
		cli.ShowError(err, 000)
		return
	}

	// Remove duplicate channels from existing XEPG database based on new hash logic
	channels.RemoveDuplicateChannels()

	// Get current M3U channels
	m3uChannels := make(map[string]structs.M3UChannelStructXEPG)
	for _, dsa := range config.Data.Streams.Active {
		var m3uChannel structs.M3UChannelStructXEPG
		err = json.Unmarshal([]byte(jsonserializer.MapToJSON(dsa)), &m3uChannel)
		if err == nil {
			// Use tvg-id as the key for matching channels
			key := m3uChannel.TvgID
			if key == "" {
				key = m3uChannel.TvgName
			}
			m3uChannels[key] = m3uChannel
		}
	}

	// Update URLs in XEPG database
	for id, dxc := range config.Data.XEPG.Channels {
		var xepgChannel structs.XEPGChannelStruct
		err = json.Unmarshal([]byte(jsonserializer.MapToJSON(dxc)), &xepgChannel)
		if err == nil {
			// Find matching M3U channel using tvg-id or tvg-name
			key := xepgChannel.TvgID
			if key == "" {
				key = xepgChannel.TvgName
			}
			if m3uChannel, ok := m3uChannels[key]; ok {
				// Always update URL if it's different
				if xepgChannel.URL != m3uChannel.URL {
					xepgChannel.URL = m3uChannel.URL
					config.Data.XEPG.Channels[id] = xepgChannel
				}
			}
		}
	}

	// Save updated XEPG database
	err = storage.SaveMapToJSONFile(config.System.File.XEPG, config.Data.XEPG.Channels)
	if err != nil {
		cli.ShowError(err, 000)
		return err
	}

	var createNewID = func() (xepg string) {

		var firstID = 0 //len(Data.XEPG.Channels)

	newXEPGID:

		if _, ok := config.Data.XEPG.Channels["x-ID."+strconv.FormatInt(int64(firstID), 10)]; ok {
			firstID++
			goto newXEPGID
		}

		xepg = "x-ID." + strconv.FormatInt(int64(firstID), 10)
		return
	}

	var getFreeChannelNumber = func(startingNumber float64) (xChannelID string) {

		sort.Float64s(allChannelNumbers)

		for {

			if utilities.IndexOfFloat64(startingNumber, allChannelNumbers) == -1 {
				xChannelID = fmt.Sprintf("%g", startingNumber)
				allChannelNumbers = append(allChannelNumbers, startingNumber)
				return
			}

			startingNumber++

		}
	}

	cli.ShowInfo("XEPG:" + "Update database")

	// Kanal mit fehlenden Kanalnummern löschen.  Delete channel with missing channel numbers
	for id, dxc := range config.Data.XEPG.Channels {

		var xepgChannel structs.XEPGChannelStruct
		err = json.Unmarshal([]byte(jsonserializer.MapToJSON(dxc)), &xepgChannel)
		if err != nil {
			return
		}

		if len(xepgChannel.XChannelID) == 0 {
			delete(config.Data.XEPG.Channels, id)
		}

		if xChannelID, err := strconv.ParseFloat(xepgChannel.XChannelID, 64); err == nil {
			allChannelNumbers = append(allChannelNumbers, xChannelID)
		}

	}

	// Make a map of the db channels based on their previously downloaded attributes -- filename, group, title, etc
	var xepgChannelsValuesMap = make(map[string]structs.XEPGChannelStruct, config.System.UnfilteredChannelLimit)
	for _, v := range config.Data.XEPG.Channels {
		var channel structs.XEPGChannelStruct
		err = json.Unmarshal([]byte(jsonserializer.MapToJSON(v)), &channel)
		if err != nil {
			return
		}

		if channel.TvgName == "" {
			channel.TvgName = channel.Name
		}

		// Create consistent channel hash using URL as primary identifier
		// Use TvgID when available, since names can change but IDs should remain stable
		var hashInput string
		if channel.TvgID != "" {
			// Use TvgID when available for stable identification
			hashInput = channel.URL + channel.TvgID + channel.FileM3UID
		} else {
			// Fall back to URL + FileM3UID only when TvgID is blank
			hashInput = channel.URL + channel.FileM3UID
		}
		hash := md5.Sum([]byte(hashInput))
		channelHash := hex.EncodeToString(hash[:])
		xepgChannelsValuesMap[channelHash] = channel
	}

	for _, dsa := range config.Data.Streams.Active {
		var channelExists = false  // Entscheidet ob ein Kanal neu zu Datenbank hinzugefügt werden soll.  Decides whether a channel should be added to the database
		var channelHasUUID = false // Überprüft, ob der Kanal (Stream) eindeutige ID's besitzt.  Checks whether the channel (stream) has unique IDs
		var currentXEPGID string   // Aktuelle Datenbank ID (XEPG). Wird verwendet, um den Kanal in der Datenbank mit dem Stream der M3u zu aktualisieren. Current database ID (XEPG) Used to update the channel in the database with the stream of the M3u

		var m3uChannel structs.M3UChannelStructXEPG

		err = json.Unmarshal([]byte(jsonserializer.MapToJSON(dsa)), &m3uChannel)
		if err != nil {
			return
		}

		if m3uChannel.TvgName == "" {
			m3uChannel.TvgName = m3uChannel.Name
		}

		// Try to find the channel based on matching all known values.  If that fails, then move to full channel scan
		// Create consistent channel hash using URL as primary identifier
		// Use TvgID when available, since names can change but IDs should remain stable
		var hashInput string
		if m3uChannel.TvgID != "" {
			// Use TvgID when available for stable identification
			hashInput = m3uChannel.URL + m3uChannel.TvgID + m3uChannel.FileM3UID
		} else {
			// Fall back to URL + FileM3UID only when TvgID is blank
			hashInput = m3uChannel.URL + m3uChannel.FileM3UID
		}
		hash := md5.Sum([]byte(hashInput))
		m3uChannelHash := hex.EncodeToString(hash[:])

		config.Data.Cache.Streams.Active = append(config.Data.Cache.Streams.Active, m3uChannelHash)

		if val, ok := xepgChannelsValuesMap[m3uChannelHash]; ok {
			channelExists = true
			currentXEPGID = val.XEPG
			if len(m3uChannel.UUIDValue) > 0 {
				channelHasUUID = true
			}
		} else {
			// XEPG Datenbank durchlaufen um nach dem Kanal zu suchen.  Run through the XEPG database to search for the channel (full scan)
			for _, dxc := range xepgChannelsValuesMap {
				if m3uChannel.FileM3UID == dxc.FileM3UID && !isInInactiveList(dxc.URL) {

					dxc.FileM3UID = m3uChannel.FileM3UID
					dxc.FileM3UName = m3uChannel.FileM3UName

					// Vergleichen des Streams anhand einer UUID in der M3U mit dem Kanal in der Databank.  Compare the stream using a UUID in the M3U with the channel in the database
					if len(dxc.UUIDValue) > 0 && len(m3uChannel.UUIDValue) > 0 {
						if dxc.UUIDValue == m3uChannel.UUIDValue {

							channelExists = true
							channelHasUUID = true
							currentXEPGID = dxc.XEPG
							break

						}
					}
				}

			}
		}

		switch channelExists {

		case true:
			// Bereits vorhandener Kanal
			var xepgChannel structs.XEPGChannelStruct
			err = json.Unmarshal([]byte(jsonserializer.MapToJSON(config.Data.XEPG.Channels[currentXEPGID])), &xepgChannel)
			if err != nil {
				return
			}

			// IMPORTANT: Skip updates for manually deactivated channels
			// If user has deactivated a channel, respect that choice during updates
			if !xepgChannel.XActive {
				cli.ShowInfo(fmt.Sprintf("XEPG:Skipping update for deactivated channel: %s (%s)", currentXEPGID, xepgChannel.XName))
				continue // Skip to next channel, don't update deactivated channels
			}

			// Update existing channel - since we found it via hash, it's the same logical channel
			if xepgChannel.TvgName == "" {
				xepgChannel.TvgName = xepgChannel.Name
			}

			// PRESERVE manual channel number assignments
			// Don't overwrite XChannelID/TvgChno for existing channels
			// User's manual channel number settings should be respected

			// Always update streaming URL
			xepgChannel.URL = m3uChannel.URL

			// Update Live Event status
			if m3uChannel.LiveEvent == "true" {
				xepgChannel.Live = true
			}

			// Update the ChannelUniqueID to new hash value
			xepgChannel.ChannelUniqueID = m3uChannelHash

			// Update channel name - for Live Events, allow name updates even without UUID
			if channelHasUUID {
				programData, _ := xmltv.GetData(xepgChannel)
				if xepgChannel.XUpdateChannelName || strings.Contains(xepgChannel.TvgID, "threadfin-") || (m3uChannel.LiveEvent == "true" && len(programData.Program) <= 3) {
					xepgChannel.XName = m3uChannel.Name
					xepgChannel.TvgName = m3uChannel.TvgName // Also update TvgName for Live Events
				}
			} else if m3uChannel.LiveEvent == "true" {
				// For Live Events without UUID, still allow name updates since they change frequently
				xepgChannel.XName = m3uChannel.Name
				xepgChannel.TvgName = m3uChannel.TvgName
			}

			// For Live Event channels, ensure they use Live Event EPG if they have insufficient program data
			if m3uChannel.LiveEvent == "true" && xepgChannel.Live {
				programData, _ := xmltv.GetData(xepgChannel)
				if len(programData.Program) <= 3 {
					cli.ShowInfo(fmt.Sprintf("XEPG: Updating Live Event channel to use Live EPG: %s", xepgChannel.XName))
					xepgChannel.XmltvFile = "Threadfin Dummy"
					xepgChannel.XMapping = "PPV"
				}
			}

			// Update channel logo
			if xepgChannel.XUpdateChannelIcon {
				var imgc = config.Data.Cache.Images
				xepgChannel.TvgLogo = imgc.Image.GetURL(m3uChannel.TvgLogo, config.Settings.HttpThreadfinDomain, config.Settings.Port, config.Settings.ForceHttps, config.Settings.HttpsPort, config.Settings.HttpsThreadfinDomain)
			}

			// PRESERVE XActive status - don't change user's activation choice
			// The XActive field is NOT updated here, preserving user's manual setting

			config.Data.XEPG.Channels[currentXEPGID] = xepgChannel

		case false:
			// Neuer Kanal
			var firstFreeNumber = config.Settings.MappingFirstChannel
			// Check channel start number from Group Filter
			filters := []structs.FilterStruct{}
			for _, filter := range config.Settings.Filter {
				filter_json, _ := json.Marshal(filter)
				f := structs.FilterStruct{}
				err = json.Unmarshal(filter_json, &f)
				if err != nil {
					log.Println("XEPG:createXEPGDatabase:Error unmarshalling filter:", err)
					return
				}
				filters = append(filters, f)
			}

			for _, filter := range filters {
				if m3uChannel.GroupTitle == filter.Filter {
					start_num, _ := strconv.ParseFloat(filter.StartingNumber, 64)
					firstFreeNumber = start_num
				}
			}

			var xepg = createNewID()
			var xChannelID string

			if m3uChannel.TvgChno == "" {
				xChannelID = getFreeChannelNumber(firstFreeNumber)
			} else {
				xChannelID = m3uChannel.TvgChno
			}

			var newChannel structs.XEPGChannelStruct
			newChannel.FileM3UID = m3uChannel.FileM3UID
			newChannel.FileM3UName = m3uChannel.FileM3UName
			newChannel.FileM3UPath = m3uChannel.FileM3UPath
			newChannel.Values = m3uChannel.Values
			newChannel.GroupTitle = m3uChannel.GroupTitle
			newChannel.Name = m3uChannel.Name
			newChannel.TvgID = m3uChannel.TvgID
			newChannel.TvgLogo = m3uChannel.TvgLogo
			newChannel.TvgName = m3uChannel.TvgName
			newChannel.URL = m3uChannel.URL
			newChannel.Live, _ = strconv.ParseBool(m3uChannel.LiveEvent)

			for file, xmltvChannels := range config.Data.XMLTV.Mapping {
				channelsMap, ok := xmltvChannels.(map[string]interface{})
				if !ok {
					continue
				}
				if channel, ok := channelsMap[m3uChannel.TvgID]; ok {
					filters := []structs.FilterStruct{}
					for _, filter := range config.Settings.Filter {
						filter_json, _ := json.Marshal(filter)
						f := structs.FilterStruct{}
						err = json.Unmarshal(filter_json, &f)
						if err != nil {
							log.Println("XEPG:createXEPGDatabase:Error unmarshalling filter:", err)
							return
						}
						filters = append(filters, f)
					}
					for _, filter := range filters {
						if newChannel.GroupTitle == filter.Filter {
							category := &structs.Category{}
							category.Value = filter.Category
							category.Lang = "en"
							newChannel.XCategory = filter.Category
						}
					}

					chmap, okk := channel.(map[string]interface{})
					if !okk {
						continue
					}

					if channelID, ok := chmap["id"].(string); ok {
						newChannel.XmltvFile = file
						newChannel.XMapping = channelID
						newChannel.XActive = true
						if newChannel.Live {
							cli.ShowInfo(fmt.Sprintf("XEPG:New live channel created (active): %s (%s)", newChannel.Name, newChannel.XGroupTitle))
						} else {
							cli.ShowInfo(fmt.Sprintf("XEPG:New channel created (active): %s (%s)", newChannel.Name, newChannel.XGroupTitle))
						}

						// Falls in der XMLTV Datei ein Logo existiert, wird dieses verwendet. Falls nicht, dann das Logo aus der M3U Datei
						/*if icon, ok := chmap["icon"].(string); ok {
							if len(icon) > 0 {
								newChannel.TvgLogo = icon
							}
						}*/

						break

					}

				}

			}

			programData, _ := xmltv.GetData(newChannel)

			if newChannel.Live && len(programData.Program) <= 3 {
				newChannel.XmltvFile = "Threadfin Dummy"
				newChannel.XMapping = "PPV"
				newChannel.XActive = true
				cli.ShowInfo(fmt.Sprintf("XEPG:New live channel created (active): %s (%s)", newChannel.Name, newChannel.XGroupTitle))
			}

			if len(m3uChannel.UUIDKey) > 0 {
				newChannel.UUIDKey = m3uChannel.UUIDKey
				newChannel.UUIDValue = m3uChannel.UUIDValue
			} else {
				newChannel.UUIDKey = ""
				newChannel.UUIDValue = ""
			}

			newChannel.XName = m3uChannel.Name
			newChannel.XGroupTitle = m3uChannel.GroupTitle
			newChannel.XEPG = xepg
			newChannel.TvgChno = xChannelID
			newChannel.XChannelID = xChannelID
			newChannel.ChannelUniqueID = m3uChannelHash
			config.Data.XEPG.Channels[xepg] = newChannel
			xepgChannelsValuesMap[m3uChannelHash] = newChannel

		}
	}

	cli.ShowInfo("XEPG:" + "Save DB file")

	err = storage.SaveMapToJSONFile(config.System.File.XEPG, config.Data.XEPG.Channels)
	if err != nil {
		cli.ShowError(err, 000)
		return
	}

	return
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

func isInInactiveList(channelURL string) bool {
	for _, channel := range config.Data.Streams.Inactive {
		// Type assert channel to map[string]interface{}
		chMap, ok := channel.(map[string]interface{})
		if !ok {
			continue
		}

		urlValue, exists := chMap["url"]
		if !exists {
			continue
		}

		urlStr, ok := urlValue.(string)
		if !ok {
			continue
		}

		if urlStr == channelURL {
			return true
		}
	}
	return false
}
