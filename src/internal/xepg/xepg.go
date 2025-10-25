package xepg

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"runtime"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/config"
	"threadfin/src/internal/hdhr"
	"threadfin/src/internal/imgcache"
	jsonserializer "threadfin/src/internal/json-serializer"
	"threadfin/src/internal/m3u"
	"threadfin/src/internal/storage"
	"threadfin/src/internal/structs"
	"threadfin/src/internal/utilities"
	"threadfin/src/internal/xmltv"
)

// XEPG Daten erstellen
func BuildXEPG(background bool) {
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

				Cleanup()
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

			Cleanup()
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

		_, err = hdhr.GetLineup()
		if err != nil {
			cli.ShowError(err, 000)
		}

		config.System.ScanInProgress = 0

	}

}

// Update XEPG data
func UpdateXEPG(background bool) {

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

			Cleanup()

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

func Cleanup() {

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
