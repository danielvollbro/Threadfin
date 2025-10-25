package src

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"threadfin/src/internal/cli"
	"threadfin/src/internal/config"
	"threadfin/src/internal/dvr"
	"threadfin/src/internal/imgcache"
	jsonserializer "threadfin/src/internal/json-serializer"
	"threadfin/src/internal/m3u"
	systemSettings "threadfin/src/internal/settings"
	"threadfin/src/internal/storage"
	"threadfin/src/internal/structs"
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
