package stream

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"threadfin/src/internal/config"
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
