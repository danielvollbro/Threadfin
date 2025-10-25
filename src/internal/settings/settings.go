package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"threadfin/internal/buffer"
	"threadfin/internal/cli"
	"threadfin/internal/config"
	jsonserializer "threadfin/internal/json-serializer"
	"threadfin/internal/plex"
	"threadfin/internal/storage"
	"threadfin/internal/structs"
	"threadfin/internal/utilities"
)

// Einstellungen speichern (Threadfin)
func SaveSettings(settings structs.SettingsStruct) (err error) {

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

	err = storage.WriteByteToFile(config.System.File.Settings, []byte(jsonserializer.MapToJSON(settings)))
	if err != nil {
		return
	}

	config.Settings = settings

	if config.System.Dev {
		config.Settings.UUID = "2019-01-DEV-Threadfin!"
	}

	plex.SetDeviceID()

	return
}

// Load settings and set default values ​​(Threadfin)
func Load() (settings structs.SettingsStruct, err error) {
	settingsMap, err := storage.LoadJSONFileToMap(config.System.File.Settings)
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
	defaults["uuid"] = utilities.CreateUUID()
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
	err = json.Unmarshal([]byte(jsonserializer.MapToJSON(settingsMap)), &settings)
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
		settings.FFmpegPath = storage.SearchFileInOS("ffmpeg")
	}

	if len(settings.VLCPath) == 0 {
		settings.VLCPath = storage.SearchFileInOS("cvlc")
	}

	// Initialze virutal filesystem for the Buffer
	buffer.InitVFS()

	settings.Version = config.System.DBVersion

	err = SaveSettings(settings)
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

func isRunningInContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err != nil {
		return false
	}
	return true
}
