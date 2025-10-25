package stream

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"threadfin/src/internal/config"
	"threadfin/src/internal/crypt"
	jsonserializer "threadfin/src/internal/json-serializer"
	"threadfin/src/internal/storage"
	"threadfin/src/internal/structs"
)

func GetStreamInfo(urlID string) (streamInfo structs.StreamInfo, err error) {
	if len(config.Data.Cache.StreamingURLS) == 0 {

		tmp, err := storage.LoadJSONFileToMap(config.System.File.URLS)
		if err != nil {
			return streamInfo, err
		}

		err = json.Unmarshal([]byte(jsonserializer.MapToJSON(tmp)), &config.Data.Cache.StreamingURLS)
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

func CreateID(stream map[int]structs.ThisStream, ip, userAgent string) (streamID int) {
	streamID = 0
	uniqueIdentifier := fmt.Sprintf("%s-%s", ip, userAgent)

	for i := 0; i <= len(stream); i++ {
		if _, ok := stream[i]; !ok {
			streamID = i
			break
		}
	}

	if _, ok := stream[streamID]; ok && stream[streamID].ClientID == uniqueIdentifier {
		// Return the same ID if the combination already exists
		return streamID
	}

	return
}

// Provider Streaming-URL zu Threadfin Streaming-URL konvertieren
func CreateURL(streamingType, playlistID, channelNumber, channelName, url string, backup_channel_1 *structs.BackupStream, backup_channel_2 *structs.BackupStream, backup_channel_3 *structs.BackupStream) (streamingURL string, err error) {

	var streamInfo structs.StreamInfo
	var serverProtocol string

	if len(config.Data.Cache.StreamingURLS) == 0 {
		config.Data.Cache.StreamingURLS = make(map[string]structs.StreamInfo)
	}

	var urlID = crypt.GetMD5(fmt.Sprintf("%s-%s", playlistID, url))

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
