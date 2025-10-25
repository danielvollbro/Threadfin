package src

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"bufio"
	"threadfin/src/internal/channels"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/compression"
	"threadfin/src/internal/config"
	"threadfin/src/internal/imgcache"
	jsonserializer "threadfin/src/internal/json-serializer"
	"threadfin/src/internal/provider"
	"threadfin/src/internal/storage"
	"threadfin/src/internal/structs"
	"threadfin/src/internal/utilities"
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

				cleanupXEPG()
				err = createXMLTVFile()
				if err != nil {
					cli.ShowError(err, 000)
					return
				}

				createM3UFile()

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

						err = createXMLTVFile()
						if err != nil {
							cli.ShowError(err, 000)
							return
						}
						createM3UFile()

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

			cleanupXEPG()
			err = createXMLTVFile()
			if err != nil {
				cli.ShowError(err, 000)
				return
			}

			createM3UFile()

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

						err = createXMLTVFile()
						if err != nil {
							cli.ShowError(err, 000)
							return
						}
						createM3UFile()

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

			cleanupXEPG()

			// Exit maintenance before long file generation to keep UI responsive
			config.System.ScanInProgress = 0

			go func() {

				err = createXMLTVFile()
				if err != nil {
					cli.ShowError(err, 000)
					return
				}
				createM3UFile()
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
			var xmltv structs.XMLTV
			err = getLocalXMLTV(file, &xmltv)
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
				var xmltvMap = make(map[string]interface{}, len(xmltv.Channel)) // Pre-allocate

				for _, c := range xmltv.Channel {
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
				programData, _ := getProgramData(xepgChannel)
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
				programData, _ := getProgramData(xepgChannel)
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

			programData, _ := getProgramData(newChannel)

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

// XMLTV Datei erstellen
func createXMLTVFile() (err error) {

	// Image Cache
	// 4edd81ab7c368208cc6448b615051b37.jpg
	var imgc = config.Data.Cache.Images

	config.Data.Cache.ImagesFiles = []string{}
	config.Data.Cache.ImagesURLS = []string{}
	config.Data.Cache.ImagesCache = []string{}

	files, err := os.ReadDir(config.System.Folder.ImagesCache)
	if err == nil {

		for _, file := range files {

			if utilities.IndexOfString(file.Name(), config.Data.Cache.ImagesCache) == -1 {
				config.Data.Cache.ImagesCache = append(config.Data.Cache.ImagesCache, file.Name())
			}

		}

	}

	if len(config.Data.XMLTV.Files) == 0 && len(config.Data.Streams.Active) == 0 {
		config.Data.XEPG.Channels = make(map[string]interface{})
		return
	}

	cli.ShowInfo("XEPG:" + fmt.Sprintf("Create XMLTV file (%s)", config.System.File.XML))

	// Stream XML to disk to avoid huge memory usage
	xmlFile, err := os.Create(config.System.File.XML)
	if err != nil {
		return err
	}
	defer func() {
		err = xmlFile.Close()
	}()
	if err != nil {
		return err
	}

	// Use buffered writer for performance
	writer := bufio.NewWriterSize(xmlFile, 1<<20) // 1MB buffer
	defer func() {
		err = writer.Flush()
	}()
	if err != nil {
		return err
	}

	var xepgXML structs.XMLTV

	xepgXML.Generator = config.System.Name

	if config.System.Branch == "main" {
		xepgXML.Source = fmt.Sprintf("%s - %s", config.System.Name, config.System.Version)
	} else {
		xepgXML.Source = fmt.Sprintf("%s - %s.%s", config.System.Name, config.System.Version, config.System.Build)
	}

	var tmpProgram = &structs.XMLTV{}

	if _, err = writer.WriteString(xml.Header); err != nil {
		return err
	}
	if _, err = writer.WriteString("<tv>\n"); err != nil {
		return err
	}

	if _, err = fmt.Fprintf(writer, "  <generator>%s</generator>\n", xepgXML.Generator); err != nil {
		return err
	}
	if _, err = fmt.Fprintf(writer, "  <source>%s</source>\n", xepgXML.Source); err != nil {
		return err
	}

	type channelEntry struct {
		idx int
		ch  structs.XEPGChannelStruct
	}
	var entries []channelEntry

	for _, dxc := range config.Data.XEPG.Channels {
		var xepgChannel structs.XEPGChannelStruct
		err := json.Unmarshal([]byte(jsonserializer.MapToJSON(dxc)), &xepgChannel)
		if err == nil {
			entries = append(entries, channelEntry{idx: len(entries), ch: xepgChannel})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		chI := entries[i].ch.TvgChno
		chJ := entries[j].ch.TvgChno

		numI, errI := strconv.ParseFloat(chI, 64)
		numJ, errJ := strconv.ParseFloat(chJ, 64)

		if errI == nil && errJ == nil {
			return numI < numJ
		}

		if errI == nil && errJ != nil {
			return true
		}
		if errI != nil && errJ == nil {
			return false
		}

		return chI < chJ
	})

	for _, e := range entries {
		xepgChannel := e.ch
		if xepgChannel.TvgName == "" {
			xepgChannel.TvgName = xepgChannel.Name
		}
		if xepgChannel.XName == "" {
			xepgChannel.XName = xepgChannel.TvgName
		}

		if xepgChannel.XActive && !xepgChannel.XHideChannel {
			if (config.Settings.XepgReplaceChannelTitle && xepgChannel.XMapping == "PPV") || xepgChannel.XName != "" {
				channel := structs.Channel{
					ID: xepgChannel.XChannelID,
					Icon: structs.Icon{
						Src: imgc.Image.GetURL(xepgChannel.TvgLogo, config.Settings.HttpThreadfinDomain, config.Settings.Port, config.Settings.ForceHttps, config.Settings.HttpsPort, config.Settings.HttpsThreadfinDomain),
					},
					DisplayName: []structs.DisplayName{
						{Value: xepgChannel.XName},
					},
					Active: xepgChannel.XActive,
					Live:   xepgChannel.Live,
				}
				bytes, _ := xml.MarshalIndent(channel, "  ", "    ")
				if _, err = writer.Write(bytes); err != nil {
					return err
				}
				if _, err = writer.WriteString("\n"); err != nil {
					return err
				}
			}
		}
	}

	for _, e := range entries {
		xepgChannel := e.ch
		if xepgChannel.XActive && !xepgChannel.XHideChannel {
			*tmpProgram, err = getProgramData(xepgChannel)
			if err == nil {
				for _, p := range tmpProgram.Program {
					bytes, _ := xml.MarshalIndent(p, "  ", "    ")
					if _, err = writer.Write(bytes); err != nil {
						return err
					}
					if _, err = writer.WriteString("\n"); err != nil {
						return err
					}
				}
			} else {
				cli.ShowDebug("XEPG:"+fmt.Sprintf("Error: %s", err), 3)
			}
		}
	}

	// Close tv root
	if _, err = writer.WriteString("</tv>\n"); err != nil {
		return err
	}

	cli.ShowInfo("XEPG:" + fmt.Sprintf("Compress XMLTV file (%s)", config.System.Compressed.GZxml))
	if err = compression.CompressGZIPFile(config.System.File.XML, config.System.Compressed.GZxml); err != nil {
		return err
	}

	return
}

// Programmdaten erstellen (createXMLTVFile)
func getProgramData(xepgChannel structs.XEPGChannelStruct) (xepgXML structs.XMLTV, err error) {
	var xmltvFile = config.System.Folder.Data + xepgChannel.XmltvFile
	var channelID = xepgChannel.XMapping

	var xmltv structs.XMLTV

	if strings.Contains(xmltvFile, "Threadfin Dummy") {
		xmltv = createDummyProgram(xepgChannel)
	} else {
		if xepgChannel.XmltvFile != "" {
			err = getLocalXMLTV(xmltvFile, &xmltv)
			if err != nil {
				return
			}
		}
	}

	for _, xmltvProgram := range xmltv.Program {
		if xmltvProgram.Channel == channelID {
			var program = &structs.Program{}

			// Channel ID
			program.Channel = xepgChannel.XChannelID
			program.Start = xmltvProgram.Start
			program.Stop = xmltvProgram.Stop

			// Title
			if len(xmltvProgram.Title) > 0 {
				if !config.Settings.EnableNonAscii {
					xmltvProgram.Title[0].Value = strings.TrimSpace(strings.Map(func(r rune) rune {
						if r > unicode.MaxASCII {
							return -1
						}
						return r
					}, xmltvProgram.Title[0].Value))
				}
				program.Title = xmltvProgram.Title
			}

			filters := []structs.FilterStruct{}
			for _, filter := range config.Settings.Filter {
				filter_json, _ := json.Marshal(filter)
				f := structs.FilterStruct{}
				err = json.Unmarshal(filter_json, &f)
				if err != nil {
					log.Println("XEPG:getProgramData:Error unmarshalling filter:", err)
					return
				}
				filters = append(filters, f)
			}

			// Category (Kategorie)
			getCategory(program, xmltvProgram, xepgChannel, filters)

			// Sub-Title
			program.SubTitle = xmltvProgram.SubTitle

			// Description
			program.Desc = xmltvProgram.Desc

			// Credits : (Credits)
			program.Credits = xmltvProgram.Credits

			// Rating (Bewertung)
			program.Rating = xmltvProgram.Rating

			// StarRating (Bewertung / Kritiken)
			program.StarRating = xmltvProgram.StarRating

			// Country (Länder)
			program.Country = xmltvProgram.Country

			// Program icon (Poster / Cover)
			getPoster(program, xmltvProgram, xepgChannel, config.Settings.ForceHttps)

			// Language (Sprache)
			program.Language = xmltvProgram.Language

			// Episodes numbers (Episodennummern)
			getEpisodeNum(program, xmltvProgram, xepgChannel)

			// Video (Videoparameter)
			getVideo(program, xmltvProgram, xepgChannel)

			// Date (Datum)
			program.Date = xmltvProgram.Date

			// Previously shown (Wiederholung)
			program.PreviouslyShown = xmltvProgram.PreviouslyShown

			// New (Neu)
			program.New = xmltvProgram.New

			// Live
			program.Live = xmltvProgram.Live

			// Premiere
			program.Premiere = xmltvProgram.Premiere

			xepgXML.Program = append(xepgXML.Program, program)

		}

	}

	return
}

func createLiveProgram(xepgChannel structs.XEPGChannelStruct, channelId string) []*structs.Program {
	var programs []*structs.Program

	var currentTime = time.Now()
	localLocation := currentTime.Location() // Central Time (CT)

	startTime := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 0, 0, 0, currentTime.Nanosecond(), localLocation)
	stopTime := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 23, 59, 59, currentTime.Nanosecond(), localLocation)

	name := ""
	if xepgChannel.XName != "" {
		name = xepgChannel.XName
	} else {
		name = xepgChannel.TvgName
	}

	// Search for Datetime or Time
	// Datetime examples: '12/31-11:59 PM', '1.1 6:30 AM', '09/15-10:00PM', '7/4 12:00 PM', '3.21 3:45 AM', '6/30-8:00 AM', '4/15 3AM'
	// Time examples: '11:59 PM', '6:30 AM', '11:59PM', '1PM'
	re := regexp.MustCompile(`((\d{1,2}[./]\d{1,2})[-\s])*(\d{1,2}(:\d{2})*\s*(AM|PM)?(?:\s*(ET|CT|MT|PT|EST|CST|MST|PST))?)`)
	matches := re.FindStringSubmatch(name)
	if len(matches) > 0 {
		timePart := matches[len(matches)-2]
		if timePart == "" {
			timePart = matches[len(matches)-1]
		}

		timeString := strings.TrimSpace(timePart)
		timeString = strings.ReplaceAll(timeString, "  ", " ")

		// Handle timezone if present
		var location *time.Location
		if strings.Contains(timeString, "ET") || strings.Contains(timeString, "EST") {
			location, _ = time.LoadLocation("America/New_York")
		} else if strings.Contains(timeString, "CT") || strings.Contains(timeString, "CST") {
			location, _ = time.LoadLocation("America/Chicago")
		} else if strings.Contains(timeString, "MT") || strings.Contains(timeString, "MST") {
			location, _ = time.LoadLocation("America/Denver")
		} else if strings.Contains(timeString, "PT") || strings.Contains(timeString, "PST") {
			location, _ = time.LoadLocation("America/Los_Angeles")
		} else {
			location = currentTime.Location()
		}

		// Remove timezone from timeString
		timeString = strings.ReplaceAll(timeString, "ET", "")
		timeString = strings.ReplaceAll(timeString, "CT", "")
		timeString = strings.ReplaceAll(timeString, "MT", "")
		timeString = strings.ReplaceAll(timeString, "PT", "")
		timeString = strings.ReplaceAll(timeString, "EST", "")
		timeString = strings.ReplaceAll(timeString, "CST", "")
		timeString = strings.ReplaceAll(timeString, "MST", "")
		timeString = strings.ReplaceAll(timeString, "PST", "")
		timeString = strings.TrimSpace(timeString)

		// Handle different date formats
		var datePart string
		if len(matches) > 3 && matches[2] != "" {
			datePart = matches[2]
			// Convert slashes to dots for consistency
			datePart = strings.ReplaceAll(datePart, "/", ".")
		}

		// Build the full time string
		var fullTimeString string
		if datePart != "" {
			// If we have a date part, use it
			parts := strings.Split(datePart, ".")
			if len(parts) == 2 {
				month := parts[0]
				day := parts[1]
				fullTimeString = fmt.Sprintf("%d.%s.%s %s", currentTime.Year(), month, day, timeString)
			}
		} else {
			// If no date part, use current date
			fullTimeString = fmt.Sprintf("%d.%d.%d %s", currentTime.Year(), currentTime.Month(), currentTime.Day(), timeString)
		}

		// Determine layout based on time format
		var layout string
		if strings.Contains(timeString, ":") {
			if strings.Contains(timeString, "AM") || strings.Contains(timeString, "PM") {
				layout = "2006.1.2 3:04 PM"
			} else {
				layout = "2006.1.2 15:04"
			}
		} else {
			if strings.Contains(timeString, "AM") || strings.Contains(timeString, "PM") {
				layout = "2006.1.2 3PM"
			} else {
				layout = "2006.1.2 15"
			}
		}

		startTimeParsed, err := time.ParseInLocation(layout, fullTimeString, location)
		if err != nil {
			startTime = time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 6, 0, 0, 0, location)
		} else {
			localTime := startTimeParsed.In(localLocation)
			startTime = localTime
		}
	}

	// Add "CHANNEL OFFLINE" program for the time before the event
	beginningOfDay := time.Date(startTime.Year(), startTime.Month(), startTime.Day(), 0, 0, 0, 0, localLocation)

	// Handle non-ASCII characters in offline text
	var offlineText = "CHANNEL OFFLINE"
	if !config.Settings.EnableNonAscii {
		offlineText = strings.TrimSpace(strings.Map(func(r rune) rune {
			if r > unicode.MaxASCII {
				return -1
			}
			return r
		}, offlineText))
	}

	programBefore := &structs.Program{
		Channel: channelId,
		Start:   beginningOfDay.Format("20060102150405 -0700"),
		Stop:    startTime.Format("20060102150405 -0700"),
		Title:   []*structs.Title{{Lang: "en", Value: offlineText}},
		Desc:    []*structs.Desc{{Lang: "en", Value: offlineText}},
	}
	programs = append(programs, programBefore)

	// Add the main program
	mainProgram := &structs.Program{
		Channel: channelId,
		Start:   startTime.Format("20060102150405 -0700"),
		Stop:    stopTime.Format("20060102150405 -0700"),
	}

	if config.Settings.XepgReplaceChannelTitle && xepgChannel.XMapping == "PPV" {
		title := []*structs.Title{}
		title_parsed := fmt.Sprintf("%s %s", name, xepgChannel.XPpvExtra)

		// Handle non-ASCII characters in title
		if !config.Settings.EnableNonAscii {
			title_parsed = strings.TrimSpace(strings.Map(func(r rune) rune {
				if r > unicode.MaxASCII {
					return -1
				}
				return r
			}, title_parsed))
		}

		t := &structs.Title{Lang: "en", Value: title_parsed}
		title = append(title, t)
		mainProgram.Title = title

		desc := []*structs.Desc{}
		d := &structs.Desc{Lang: "en", Value: title_parsed}
		desc = append(desc, d)
		mainProgram.Desc = desc
	}
	programs = append(programs, mainProgram)

	// Add "CHANNEL OFFLINE" program for the time after the event
	midnightNextDayStart := time.Date(stopTime.Year(), stopTime.Month(), stopTime.Day()+1, 0, 0, 0, currentTime.Nanosecond(), localLocation)
	midnightNextDayStop := time.Date(stopTime.Year(), stopTime.Month(), stopTime.Day()+1, 23, 59, 59, currentTime.Nanosecond(), localLocation)
	programAfter := &structs.Program{
		Channel: channelId,
		Start:   midnightNextDayStart.Format("20060102150405 -0700"),
		Stop:    midnightNextDayStop.Format("20060102150405 -0700"),
		Title:   []*structs.Title{{Lang: "en", Value: offlineText}},
		Desc:    []*structs.Desc{{Lang: "en", Value: offlineText}},
	}
	programs = append(programs, programAfter)

	return programs
}

// Dummy Daten erstellen (createXMLTVFile)
func createDummyProgram(xepgChannel structs.XEPGChannelStruct) (dummyXMLTV structs.XMLTV) {
	if xepgChannel.XMapping == "PPV" {
		var channelID = xepgChannel.XMapping
		programs := createLiveProgram(xepgChannel, channelID)
		dummyXMLTV.Program = programs
		return
	}

	var imgc = config.Data.Cache.Images
	var currentTime = time.Now()
	var dateArray = strings.Fields(currentTime.String())
	var offset = " " + dateArray[2]
	var currentDay = currentTime.Format("20060102")
	var startTime, _ = time.Parse("20060102150405", currentDay+"000000")

	cli.ShowInfo("Create Dummy Guide:" + "Time offset" + offset + " - " + xepgChannel.XName)

	var dummyLength = 30 // Default to 30 minutes if parsing fails
	var err error
	var dl = strings.Split(xepgChannel.XMapping, "_")
	if dl[0] != "" {
		// Check if the first part is a valid integer
		if match, _ := regexp.MatchString(`^\d+$`, dl[0]); match {
			dummyLength, err = strconv.Atoi(dl[0])
			if err != nil {
				cli.ShowError(err, 000)
				// Continue with default value instead of returning
			}
		} else {
			// For non-numeric formats that aren't "PPV" (which is handled above),
			// use the default value
			cli.ShowInfo(fmt.Sprintf("Non-numeric format for XMapping: %s, using default duration of 30 minutes", xepgChannel.XMapping))
		}
	}

	for d := 0; d < 4; d++ {

		var epgStartTime = startTime.Add(time.Hour * time.Duration(d*24))

		for t := dummyLength; t <= 1440; t = t + dummyLength {

			var epgStopTime = epgStartTime.Add(time.Minute * time.Duration(dummyLength))

			var epg structs.Program
			poster := structs.Poster{}

			epg.Channel = xepgChannel.XMapping
			epg.Start = epgStartTime.Format("20060102150405") + offset
			epg.Stop = epgStopTime.Format("20060102150405") + offset

			// Create title with proper handling of non-ASCII characters
			var titleValue = xepgChannel.XName + " (" + epgStartTime.Weekday().String()[0:2] + ". " + epgStartTime.Format("15:04") + " - " + epgStopTime.Format("15:04") + ")"
			if !config.Settings.EnableNonAscii {
				titleValue = strings.TrimSpace(strings.Map(func(r rune) rune {
					if r > unicode.MaxASCII {
						return -1
					}
					return r
				}, titleValue))
			}
			epg.Title = append(epg.Title, &structs.Title{Value: titleValue, Lang: "en"})

			if len(xepgChannel.XDescription) == 0 {
				var descValue = "Threadfin: (" + strconv.Itoa(dummyLength) + " Minutes) " + epgStartTime.Weekday().String() + " " + epgStartTime.Format("15:04") + " - " + epgStopTime.Format("15:04")
				if !config.Settings.EnableNonAscii {
					descValue = strings.TrimSpace(strings.Map(func(r rune) rune {
						if r > unicode.MaxASCII {
							return -1
						}
						return r
					}, descValue))
				}
				epg.Desc = append(epg.Desc, &structs.Desc{Value: descValue, Lang: "en"})
			} else {
				var descValue = xepgChannel.XDescription
				if !config.Settings.EnableNonAscii {
					descValue = strings.TrimSpace(strings.Map(func(r rune) rune {
						if r > unicode.MaxASCII {
							return -1
						}
						return r
					}, descValue))
				}
				epg.Desc = append(epg.Desc, &structs.Desc{Value: descValue, Lang: "en"})
			}

			if config.Settings.XepgReplaceMissingImages {
				poster.Src = imgc.Image.GetURL(xepgChannel.TvgLogo, config.Settings.HttpThreadfinDomain, config.Settings.Port, config.Settings.ForceHttps, config.Settings.HttpsPort, config.Settings.HttpsThreadfinDomain)
				epg.Poster = append(epg.Poster, poster)
			}

			if xepgChannel.XCategory != "Movie" {
				epg.EpisodeNum = append(epg.EpisodeNum, &structs.EpisodeNum{Value: epgStartTime.Format("2006-01-02 15:04:05"), System: "original-air-date"})
			}

			epg.New = &structs.New{Value: ""}

			dummyXMLTV.Program = append(dummyXMLTV.Program, &epg)
			epgStartTime = epgStopTime

		}

	}

	return
}

// Kategorien erweitern (createXMLTVFile)
func getCategory(program *structs.Program, xmltvProgram *structs.Program, xepgChannel structs.XEPGChannelStruct, filters []structs.FilterStruct) {

	for _, i := range xmltvProgram.Category {

		category := &structs.Category{}
		category.Value = i.Value
		category.Lang = i.Lang
		program.Category = append(program.Category, category)

	}

	if len(xepgChannel.XCategory) > 0 {

		category := &structs.Category{}
		category.Value = strings.ToLower(xepgChannel.XCategory)
		category.Lang = "en"
		program.Category = append(program.Category, category)

	}
}

// Programm Poster Cover aus der XMLTV Datei laden
func getPoster(program *structs.Program, xmltvProgram *structs.Program, xepgChannel structs.XEPGChannelStruct, forceHttps bool) {

	var imgc = config.Data.Cache.Images

	for _, poster := range xmltvProgram.Poster {
		poster.Src = imgc.Image.GetURL(poster.Src, config.Settings.HttpThreadfinDomain, config.Settings.Port, config.Settings.ForceHttps, config.Settings.HttpsPort, config.Settings.HttpsThreadfinDomain)
		program.Poster = append(program.Poster, poster)
	}

	if config.Settings.XepgReplaceMissingImages {

		if len(xmltvProgram.Poster) == 0 {
			var poster structs.Poster
			poster.Src = imgc.Image.GetURL(xepgChannel.TvgLogo, config.Settings.HttpThreadfinDomain, config.Settings.Port, config.Settings.ForceHttps, config.Settings.HttpsPort, config.Settings.HttpsThreadfinDomain)
			program.Poster = append(program.Poster, poster)
		}

	}

}

// Episodensystem übernehmen, falls keins vorhanden ist und eine Kategorie im Mapping eingestellt wurden, wird eine Episode erstellt
func getEpisodeNum(program *structs.Program, xmltvProgram *structs.Program, xepgChannel structs.XEPGChannelStruct) {

	program.EpisodeNum = xmltvProgram.EpisodeNum

	if len(xepgChannel.XCategory) > 0 && xepgChannel.XCategory != "Movie" {

		if len(xmltvProgram.EpisodeNum) == 0 {

			var timeLayout = "20060102150405"

			t, err := time.Parse(timeLayout, strings.Split(xmltvProgram.Start, " ")[0])
			if err == nil {
				program.EpisodeNum = append(program.EpisodeNum, &structs.EpisodeNum{Value: t.Format("2006-01-02 15:04:05"), System: "original-air-date"})
			} else {
				cli.ShowError(err, 0)
			}

		}

	}
}

// Videoparameter erstellen (createXMLTVFile)
func getVideo(program *structs.Program, xmltvProgram *structs.Program, xepgChannel structs.XEPGChannelStruct) {

	var video structs.Video
	video.Present = xmltvProgram.Video.Present
	video.Colour = xmltvProgram.Video.Colour
	video.Aspect = xmltvProgram.Video.Aspect
	video.Quality = xmltvProgram.Video.Quality

	if len(xmltvProgram.Video.Quality) == 0 {

		if strings.Contains(strings.ToUpper(xepgChannel.XName), " HD") || strings.Contains(strings.ToUpper(xepgChannel.XName), " FHD") {
			video.Quality = "HDTV"
		}

		if strings.Contains(strings.ToUpper(xepgChannel.XName), " UHD") || strings.Contains(strings.ToUpper(xepgChannel.XName), " 4K") {
			video.Quality = "UHDTV"
		}

	}

	program.Video = video
}

// Lokale Provider XMLTV Datei laden
func getLocalXMLTV(file string, xmltv *structs.XMLTV) (err error) {

	if _, ok := config.Data.Cache.XMLTV[file]; !ok {

		// Cache initialisieren
		if len(config.Data.Cache.XMLTV) == 0 {
			config.Data.Cache.XMLTV = make(map[string]structs.XMLTV)
		}

		// Check file size to determine parsing strategy
		fileInfo, err := os.Stat(file)
		if err != nil {
			err = errors.New("local copy of the file no longer exists")
			return err
		}

		// For large files (>50MB), use streaming parser
		if fileInfo.Size() > 50*1024*1024 {
			cli.ShowInfo("XEPG:" + "Using streaming parser for large XMLTV file: " + file)
			err = parseXMLTVStream(file, xmltv)
		} else {
			// Use original method for smaller files
			content, err := storage.ReadByteFromFile(file)
			if err != nil {
				err = errors.New("local copy of the file no longer exists")
				return err
			}

			// XML Datei parsen
			err = xml.Unmarshal(content, &xmltv)
			if err != nil {
				return err
			}
		}

		if err != nil {
			return err
		}

		config.Data.Cache.XMLTV[file] = *xmltv

	} else {
		*xmltv = config.Data.Cache.XMLTV[file]
	}

	return
}

// parseXMLTVStream : Streaming XML parser for large XMLTV files
func parseXMLTVStream(file string, xmltv *structs.XMLTV) error {
	xmlFile, err := os.Open(file)
	if err != nil {
		return err
	}
	defer func() {
		err = xmlFile.Close()
	}()
	if err != nil {
		return err
	}

	decoder := xml.NewDecoder(xmlFile)

	xmltv.Channel = make([]*structs.Channel, 0)
	xmltv.Program = make([]*structs.Program, 0)

	var currentElement string
	var channelCount, programCount int

	for {
		token, err := decoder.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return err
		}

		switch se := token.(type) {
		case xml.StartElement:
			currentElement = se.Name.Local

			switch currentElement {
			case "tv":
				for _, attr := range se.Attr {
					switch attr.Name.Local {
					case "generator-info-name":
						xmltv.Generator = attr.Value
					case "source-info-name":
						xmltv.Source = attr.Value
					}
				}

			case "channel":
				var channel structs.Channel
				if err := decoder.DecodeElement(&channel, &se); err != nil {
					cli.ShowDebug("XMLTV Stream:Error parsing channel: "+err.Error(), 2)
					continue
				}
				xmltv.Channel = append(xmltv.Channel, &channel)
				channelCount++

				if channelCount%1000 == 0 {
					cli.ShowInfo(fmt.Sprintf("XMLTV Stream:Parsed %d channels", channelCount))
				}

			case "programme":
				var program structs.Program
				if err := decoder.DecodeElement(&program, &se); err != nil {
					cli.ShowDebug("XMLTV Stream:Error parsing program: "+err.Error(), 3)
					continue
				}
				xmltv.Program = append(xmltv.Program, &program)
				programCount++

				if programCount%10000 == 0 {
					cli.ShowInfo(fmt.Sprintf("XMLTV Stream:Parsed %d programs", programCount))
				}
			}
		}
	}

	cli.ShowInfo(fmt.Sprintf("XMLTV Stream:Completed - %d channels, %d programs", channelCount, programCount))
	return nil
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

// M3U Datei erstellen
func createM3UFile() {

	cli.ShowInfo("XEPG:" + fmt.Sprintf("Create M3U file (%s)", config.System.File.M3U))
	_, err := buildM3U([]string{})
	if err != nil {
		cli.ShowError(err, 000)
	}

	err = storage.SaveMapToJSONFile(config.System.File.URLS, config.Data.Cache.StreamingURLS)
	if err != nil {
		cli.ShowError(err, 000)
	}
}

// XEPG Datenbank bereinigen
func cleanupXEPG() {

	var sourceIDs []string

	for source := range config.Settings.Files.M3U {
		sourceIDs = append(sourceIDs, source)
	}

	for source := range config.Settings.Files.HDHR {
		sourceIDs = append(sourceIDs, source)
	}

	cli.ShowInfo("XEPG: Cleanup database")
	config.Data.XEPG.XEPGCount = 0

	for id, dxc := range config.Data.XEPG.Channels {

		var xepgChannel structs.XEPGChannelStruct
		err := json.Unmarshal([]byte(jsonserializer.MapToJSON(dxc)), &xepgChannel)
		if err == nil {

			if xepgChannel.TvgName == "" {
				xepgChannel.TvgName = xepgChannel.Name
			}

			// Create consistent channel hash using URL as primary identifier
			// Use TvgID when available, since names can change but IDs should remain stable
			var hashInput string
			if xepgChannel.TvgID != "" {
				// Use TvgID when available for stable identification
				hashInput = xepgChannel.URL + xepgChannel.TvgID + xepgChannel.FileM3UID
			} else {
				// Fall back to URL + FileM3UID only when TvgID is blank
				hashInput = xepgChannel.URL + xepgChannel.FileM3UID
			}
			hash := md5.Sum([]byte(hashInput))
			m3uChannelHash := hex.EncodeToString(hash[:])

			if utilities.IndexOfString(m3uChannelHash, config.Data.Cache.Streams.Active) == -1 {
				delete(config.Data.XEPG.Channels, id)
			} else {
				if xepgChannel.XActive && !xepgChannel.XHideChannel {
					config.Data.XEPG.XEPGCount++
				}
			}

			if utilities.IndexOfString(xepgChannel.FileM3UID, sourceIDs) == -1 {
				delete(config.Data.XEPG.Channels, id)
			}

		}

	}

	err := storage.SaveMapToJSONFile(config.System.File.XEPG, config.Data.XEPG.Channels)
	if err != nil {
		cli.ShowError(err, 000)
		return
	}

	cli.ShowInfo("XEPG Channels:" + fmt.Sprintf("%d", config.Data.XEPG.XEPGCount))

	if len(config.Data.Streams.Active) > 0 && config.Data.XEPG.XEPGCount == 0 {
		cli.ShowWarning(2005)
	}
}
