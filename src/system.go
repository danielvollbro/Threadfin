package src

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/config"
	"threadfin/src/internal/structs"
	"time"
)

// Entwicklerinfos anzeigen
func showDevInfo() {
	if config.System.Dev {
		fmt.Print("\033[31m")
		fmt.Println("* * * * * D E V   M O D E * * * * *")
		fmt.Println("Version: ", config.System.Version)
		fmt.Println("Build:   ", config.System.Build)
		fmt.Println("* * * * * * * * * * * * * * * * * *")
		fmt.Print("\033[0m")
		fmt.Println()
	}
}

// Alle Systemordner erstellen
func createSystemFolders() (err error) {

	e := reflect.ValueOf(&config.System.Folder).Elem()

	for i := 0; i < e.NumField(); i++ {

		var folder = e.Field(i).Interface().(string)

		err = checkFolder(folder)

		if err != nil {
			return
		}

	}

	return
}

// Alle Systemdateien erstellen
func createSystemFiles() (err error) {
	var debug string
	for _, file := range config.SystemFiles {

		var filename = getPlatformFile(config.System.Folder.Config + file)

		err = checkFile(filename)
		if err != nil {
			// File does not exist, will be created now
			err = saveMapToJSONFile(filename, make(map[string]interface{}))
			if err != nil {
				return
			}

			debug = fmt.Sprintf("Create File:%s", filename)
			cli.ShowDebug(debug, 1)

		}

		switch file {

		case "authentication.json":
			config.System.File.Authentication = filename
		case "pms.json":
			config.System.File.PMS = filename
		case "settings.json":
			config.System.File.Settings = filename
		case "xepg.json":
			config.System.File.XEPG = filename
		case "urls.json":
			config.System.File.URLS = filename

		}

	}

	return
}

func updateUrlsJson() {

	getProviderData("m3u", "")
	getProviderData("hdhr", "")

	if config.Settings.EpgSource == "XEPG" {
		getProviderData("xmltv", "")
	}
	err := buildDatabaseDVR()
	if err != nil {
		cli.ShowError(err, 0)
		return
	}

	buildXEPG(false)
}

// Einstellungen laden und default Werte setzen (Threadfin)
func loadSettings() (settings structs.SettingsStruct, err error) {

	settingsMap, err := loadJSONFileToMap(config.System.File.Settings)
	if err != nil {
		return structs.SettingsStruct{}, err
	}

	// Deafult Werte setzten
	var defaults = make(map[string]interface{})
	var dataMap = make(map[string]interface{})

	dataMap["xmltv"] = make(map[string]interface{})
	dataMap["m3u"] = make(map[string]interface{})
	dataMap["hdhr"] = make(map[string]interface{})

	defaults["api"] = false
	defaults["authentication.api"] = false
	defaults["authentication.m3u"] = false
	defaults["authentication.pms"] = false
	defaults["authentication.web"] = false
	defaults["authentication.xml"] = false
	defaults["backup.keep"] = 10
	defaults["backup.path"] = config.System.Folder.Backup
	defaults["buffer"] = "-"
	defaults["buffer.size.kb"] = 1024
	defaults["buffer.timeout"] = 500
	defaults["cache.images"] = false
	defaults["epgSource"] = "PMS"
	defaults["ffmpeg.options"] = config.System.FFmpeg.DefaultOptions
	defaults["vlc.options"] = config.System.VLC.DefaultOptions
	defaults["files"] = dataMap
	defaults["files.update"] = true
	defaults["filter"] = make(map[string]interface{})
	defaults["git.branch"] = config.System.Branch
	defaults["language"] = "en"
	defaults["log.entries.ram"] = 500
	defaults["mapping.first.channel"] = 1000
	defaults["xepg.replace.missing.images"] = true
	defaults["xepg.replace.channel.title"] = false
	defaults["m3u8.adaptive.bandwidth.mbps"] = 10
	defaults["port"] = "34400"
	defaults["ssdp"] = true
	defaults["storeBufferInRAM"] = true
	defaults["forceHttps"] = false
	defaults["excludeStreamHttps"] = false
	defaults["httpsPort"] = 443
	defaults["httpsThreadfinDomain"] = ""
	defaults["httpThreadfinDomain"] = ""
	defaults["enableNonAscii"] = false
	defaults["epgCategories"] = "Kids:kids|News:news|Movie:movie|Series:series|Sports:sports"
	defaults["epgCategoriesColors"] = "kids:mediumpurple|news:tomato|movie:royalblue|series:gold|sports:yellowgreen"
	defaults["tuner"] = 1
	defaults["update"] = []string{"0000"}
	defaults["user.agent"] = config.System.Name
	defaults["uuid"] = createUUID()
	defaults["udpxy"] = ""
	defaults["version"] = config.System.DBVersion
	defaults["ThreadfinAutoUpdate"] = true
	if isRunningInContainer() {
		defaults["ThreadfinAutoUpdate"] = false
	}
	defaults["temp.path"] = config.System.Folder.Temp

	// Default Werte setzen
	for key, value := range defaults {
		if _, ok := settingsMap[key]; !ok {
			settingsMap[key] = value
		}
	}
	err = json.Unmarshal([]byte(mapToJSON(settingsMap)), &settings)
	if err != nil {
		return structs.SettingsStruct{}, err
	}

	// Einstellungen von den Flags übernehmen
	if len(config.System.Flag.Port) > 0 {
		settings.Port = config.System.Flag.Port
	}

	if len(config.System.Flag.Branch) > 0 {
		settings.Branch = config.System.Flag.Branch
		cli.ShowInfo(fmt.Sprintf("Git Branch:Switching Git Branch to -> %s", settings.Branch))
	}

	if len(settings.FFmpegPath) == 0 {
		settings.FFmpegPath = searchFileInOS("ffmpeg")
	}

	if len(settings.VLCPath) == 0 {
		settings.VLCPath = searchFileInOS("cvlc")
	}

	// Initialze virutal filesystem for the Buffer
	initBufferVFS()

	settings.Version = config.System.DBVersion

	err = saveSettings(settings)
	if err != nil {
		return structs.SettingsStruct{}, err
	}

	// Warung wenn FFmpeg nicht gefunden wurde
	if len(config.Settings.FFmpegPath) == 0 && config.Settings.Buffer == "ffmpeg" {
		cli.ShowWarning(2020)
	}

	if len(config.Settings.VLCPath) == 0 && config.Settings.Buffer == "vlc" {
		cli.ShowWarning(2021)
	}

	return settings, nil
}

// Einstellungen speichern (Threadfin)
func saveSettings(settings structs.SettingsStruct) (err error) {

	if settings.BackupKeep == 0 {
		settings.BackupKeep = 10
	}

	if len(settings.BackupPath) == 0 {
		settings.BackupPath = config.System.Folder.Backup
	}

	if settings.BufferTimeout < 0 {
		settings.BufferTimeout = 0
	}

	config.System.Folder.Temp = settings.TempPath + settings.UUID + string(os.PathSeparator)

	err = writeByteToFile(config.System.File.Settings, []byte(mapToJSON(settings)))
	if err != nil {
		return
	}

	config.Settings = settings

	if config.System.Dev {
		config.Settings.UUID = "2019-01-DEV-Threadfin!"
	}

	setDeviceID()

	return
}

// Zugriff über die Domain ermöglichen
func setGlobalDomain(domain string) {

	config.System.Domain = domain

	switch config.Settings.AuthenticationPMS {
	case true:
		config.System.Addresses.DVR = "username:password@" + config.System.Domain
	case false:
		config.System.Addresses.DVR = config.System.Domain
	}

	switch config.Settings.AuthenticationM3U {
	case true:
		config.System.Addresses.M3U = config.System.ServerProtocol.M3U + "://" + config.System.Domain + "/m3u/threadfin.m3u?username=xxx&password=yyy"
	case false:
		config.System.Addresses.M3U = config.System.ServerProtocol.M3U + "://" + config.System.Domain + "/m3u/threadfin.m3u"
	}

	switch config.Settings.AuthenticationXML {
	case true:
		config.System.Addresses.XML = config.System.ServerProtocol.XML + "://" + config.System.Domain + "/xmltv/threadfin.xml?username=xxx&password=yyy"
	case false:
		config.System.Addresses.XML = config.System.ServerProtocol.XML + "://" + config.System.Domain + "/xmltv/threadfin.xml"
	}

	if config.Settings.EpgSource != "XEPG" {
		log.Println("SOURCE: ", config.Settings.EpgSource)
		config.System.Addresses.M3U = cli.GetErrMsg(2106)
		config.System.Addresses.XML = cli.GetErrMsg(2106)
	}
}

// UUID generieren
func createUUID() (uuid string) {
	uuid = time.Now().Format("2006-01") + "-" + randomString(4) + "-" + randomString(6)
	return
}

// Eindeutige Geräte ID für Plex generieren
func setDeviceID() {
	var id = config.Settings.UUID

	switch config.Settings.Tuner {
	case 1:
		config.System.DeviceID = id

	default:
		config.System.DeviceID = fmt.Sprintf("%s:%d", id, config.Settings.Tuner)
	}
}

// Provider Streaming-URL zu Threadfin Streaming-URL konvertieren
func createStreamingURL(streamingType, playlistID, channelNumber, channelName, url string, backup_channel_1 *structs.BackupStream, backup_channel_2 *structs.BackupStream, backup_channel_3 *structs.BackupStream) (streamingURL string, err error) {

	var streamInfo structs.StreamInfo
	var serverProtocol string

	if len(config.Data.Cache.StreamingURLS) == 0 {
		config.Data.Cache.StreamingURLS = make(map[string]structs.StreamInfo)
	}

	var urlID = getMD5(fmt.Sprintf("%s-%s", playlistID, url))

	if s, ok := config.Data.Cache.StreamingURLS[urlID]; ok {
		streamInfo = s

	} else {
		streamInfo.URL = url
		streamInfo.BackupChannel1 = backup_channel_1
		streamInfo.BackupChannel2 = backup_channel_2
		streamInfo.BackupChannel3 = backup_channel_3
		streamInfo.Name = channelName
		streamInfo.PlaylistID = playlistID
		streamInfo.ChannelNumber = channelNumber
		streamInfo.URLid = urlID

		config.Data.Cache.StreamingURLS[urlID] = streamInfo

	}

	switch streamingType {

	case "DVR":
		serverProtocol = config.System.ServerProtocol.DVR

	case "M3U":
		serverProtocol = config.System.ServerProtocol.M3U

	}

	if config.Settings.ForceHttps {
		if config.Settings.HttpsThreadfinDomain != "" {
			serverProtocol = "https"
			config.System.Domain = config.Settings.HttpsThreadfinDomain
		}
	}

	streamingURL = fmt.Sprintf("%s://%s/stream/%s", serverProtocol, config.System.Domain, streamInfo.URLid)
	return
}

func getStreamInfo(urlID string) (streamInfo structs.StreamInfo, err error) {

	if len(config.Data.Cache.StreamingURLS) == 0 {

		tmp, err := loadJSONFileToMap(config.System.File.URLS)
		if err != nil {
			return streamInfo, err
		}

		err = json.Unmarshal([]byte(mapToJSON(tmp)), &config.Data.Cache.StreamingURLS)
		if err != nil {
			return streamInfo, err
		}

	}

	if s, ok := config.Data.Cache.StreamingURLS[urlID]; ok {
		s.URL = strings.Trim(s.URL, "\r\n")
		streamInfo = s
	} else {
		err = errors.New("streaming error")
	}

	return
}

func isRunningInContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err != nil {
		return false
	}
	return true
}
