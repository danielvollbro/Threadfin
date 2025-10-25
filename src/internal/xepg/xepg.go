package xepg

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/config"
	jsonserializer "threadfin/src/internal/json-serializer"
	"threadfin/src/internal/storage"
	"threadfin/src/internal/structs"
	"threadfin/src/internal/utilities"
)

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
