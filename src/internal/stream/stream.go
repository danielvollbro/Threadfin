package stream

import (
	"encoding/json"
	"errors"
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
