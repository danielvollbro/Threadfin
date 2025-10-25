package provider

import (
	"strconv"
	"threadfin/src/internal/config"
)

// Output provider parameters based on the key
func GetProviderParameter(id, fileType, key string) (s string) {
	var dataMap = make(map[string]any)

	switch fileType {
	case "m3u":
		dataMap = config.Settings.Files.M3U

	case "hdhr":
		dataMap = config.Settings.Files.HDHR

	case "xmltv":
		dataMap = config.Settings.Files.XMLTV
	}

	if data, ok := dataMap[id].(map[string]any); ok {

		if v, ok := data[key].(string); ok {
			s = v
		}

		if v, ok := data[key].(float64); ok {
			s = strconv.Itoa(int(v))
		}

	}

	return
}
