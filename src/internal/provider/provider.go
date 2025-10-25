package provider

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/compression"
	"threadfin/src/internal/config"
	"threadfin/src/internal/http"
	jsonserializer "threadfin/src/internal/json-serializer"
	"threadfin/src/internal/m3u-parser"
	"threadfin/src/internal/settings"
	"threadfin/src/internal/storage"
	"threadfin/src/internal/structs"
	"time"
)

// fileType: Welcher Dateityp soll aktualisiert werden (m3u, hdhr, xml) | fileID: Update einer bestimmten Datei (Provider ID)
func GetData(fileType, fileID string) (err error) {

	var fileExtension, serverFileName string
	var body = make([]byte, 0)
	var dataMap = make(map[string]interface{})

	var saveDateFromProvider = func(fileSource, serverFileName, id string, body []byte) (err error) {

		var data = make(map[string]interface{})

		if value, ok := dataMap[id].(map[string]interface{}); ok {
			data = value
		} else {
			data["id.provider"] = id
			dataMap[id] = data
		}

		// Default keys für die Providerdaten
		var keys = []string{"name", "description", "type", "file." + config.System.AppName, "file.source", "tuner", "http_proxy.ip", "http_proxy.port", "last.update", "compatibility", "counter.error", "counter.download", "provider.availability"}

		for _, key := range keys {

			if _, ok := data[key]; !ok {

				switch key {

				case "name":
					data[key] = serverFileName

				case "description":
					data[key] = ""

				case "type":
					data[key] = fileType

				case "file." + config.System.AppName:
					data[key] = id + fileExtension

				case "file.source":
					data[key] = fileSource

				case "http_proxy.ip":
					data[key] = ""

				case "http_proxy.port":
					data[key] = ""

				case "last.update":
					data[key] = time.Now().Format("2006-01-02 15:04:05")

				case "tuner":
					if fileType == "m3u" || fileType == "hdhr" {
						if _, ok := data[key].(float64); !ok {
							data[key] = 1
						}
					}

				case "compatibility":
					data[key] = make(map[string]interface{})

				case "counter.download":
					data[key] = 0.0

				case "counter.error":
					data[key] = 0.0

				case "provider.availability":
					data[key] = 100
				}

			}

		}

		if _, ok := data["id.provider"]; !ok {
			data["id.provider"] = id
		}

		// Datei extrahieren
		body, err = compression.ExtractGZIP(body, fileSource)
		if err != nil {
			cli.ShowError(err, 000)
			return
		}

		// Daten überprüfen
		cli.ShowInfo("Check File:" + fileSource)

		switch fileType {

		case "m3u":
			newM3u, err := m3u.MakeInterfaceFromM3U(body)
			if err != nil {
				return err
			}

			var m3uContent strings.Builder
			m3uContent.WriteString("#EXTM3U\n")

			for _, channel := range newM3u {
				channelMap := channel.(map[string]string)

				extinf := fmt.Sprintf(`#EXTINF:-1 tvg-id="%s" tvg-name="%s" tvg-chno="%s" tvg-logo="%s" group-title="%s",%s`,
					channelMap["tvg-id"],
					channelMap["tvg-name"],
					channelMap["tvg-chno"],
					channelMap["tvg-logo"],
					channelMap["group-title"],
					channelMap["name"],
				)

				m3uContent.WriteString(extinf + "\n" + channelMap["url"] + "\n")
			}

			m3uBytes := []byte(m3uContent.String())
			body = m3uBytes

		case "hdhr":
			_, err = jsonserializer.JSONToInterface(string(body))

		case "xmltv":
			err = checkXMLCompatibility(id, body)

		}

		if err != nil {
			return
		}

		var filePath = config.System.Folder.Data + data["file."+config.System.AppName].(string)

		err = storage.WriteByteToFile(filePath, body)

		if err == nil {
			data["last.update"] = time.Now().Format("2006-01-02 15:04:05")
			data["counter.download"] = data["counter.download"].(float64) + 1
		}

		return

	}

	switch fileType {

	case "m3u":
		dataMap = config.Settings.Files.M3U
		fileExtension = ".m3u"

	case "hdhr":
		dataMap = config.Settings.Files.HDHR
		fileExtension = ".json"

	case "xmltv":
		dataMap = config.Settings.Files.XMLTV
		fileExtension = ".xml"

	}

	for dataID, d := range dataMap {

		var data = d.(map[string]interface{})
		var fileSource = data["file.source"].(string)
		var httpProxyIp = ""
		if data["http_proxy.ip"] != nil {
			httpProxyIp = data["http_proxy.ip"].(string)
		}
		var httpProxyPort = ""
		if data["http_proxy.port"] != nil {
			httpProxyPort = data["http_proxy.port"].(string)
		}
		var httpProxyUrl = ""
		if httpProxyIp != "" && httpProxyPort != "" {
			httpProxyUrl = fmt.Sprintf("http://%s:%s", httpProxyIp, httpProxyPort)
		}

		var newProvider = false

		if _, ok := data["new"]; ok {
			newProvider = true
			delete(data, "new")
		}

		// Wenn eine ID vorhanden ist und nicht mit der aus der Datanbank übereinstimmt, wird die Aktualisierung übersprungen (goto)
		if len(fileID) > 0 && !newProvider {
			if dataID != fileID {
				goto Done
			}
		}

		switch fileType {

		case "hdhr":

			// Laden vom HDHomeRun Tuner
			cli.ShowInfo("Tuner:" + fileSource)
			var tunerURL = "http://" + fileSource + "/lineup.json"
			serverFileName, body, err = http.DownloadFile(tunerURL, httpProxyUrl)

		default:

			if strings.Contains(fileSource, "http://") || strings.Contains(fileSource, "https://") {

				// Laden vom Remote Server
				cli.ShowInfo("Download:" + fileSource)
				serverFileName, body, err = http.DownloadFile(fileSource, httpProxyUrl)

			} else {

				// Laden einer lokalen Datei
				cli.ShowInfo("Open:" + fileSource)

				err = storage.CheckFile(fileSource)
				if err == nil {
					body, err = storage.ReadByteFromFile(fileSource)
					serverFileName = storage.GetFilenameFromPath(fileSource)
				}

			}

		}

		if err == nil {

			err = saveDateFromProvider(fileSource, serverFileName, dataID, body)
			if err == nil {
				cli.ShowInfo("Save File:" + fileSource + " [ID: " + dataID + "]")
			}

		}

		if err != nil {

			cli.ShowError(err, 000)
			var downloadErr = err

			if !newProvider {

				// Prüfen ob ältere Datei vorhanden ist
				var file = config.System.Folder.Data + dataID + fileExtension

				err = storage.CheckFile(file)
				if err == nil {

					if len(fileID) == 0 {
						cli.ShowWarning(1011)
					}
				}

				// Fehler Counter um 1 erhöhen
				if value, ok := dataMap[dataID].(map[string]interface{}); ok {
					data = value
					data["counter.error"] = data["counter.error"].(float64) + 1
					data["counter.download"] = data["counter.download"].(float64) + 1
				}
			} else {
				return downloadErr
			}
		}

		// Berechnen der Fehlerquote
		if !newProvider {
			if value, ok := dataMap[dataID].(map[string]interface{}); ok {
				data = value

				if data["counter.error"].(float64) == 0 {
					data["provider.availability"] = 100
				} else {
					data["provider.availability"] = int(data["counter.error"].(float64)*100/data["counter.download"].(float64)*-1 + 100)
				}
			}
		}

		switch fileType {

		case "m3u":
			config.Settings.Files.M3U = dataMap

		case "hdhr":
			config.Settings.Files.HDHR = dataMap

		case "xmltv":
			config.Settings.Files.XMLTV = dataMap
			delete(config.Data.Cache.XMLTV, config.System.Folder.Data+dataID+fileExtension)

		}

		err = settings.SaveSettings(config.Settings)
		if err != nil {
			return
		}
	Done:
	}

	return
}

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

// Provider Statistiken Kompatibilität aktualisieren
func SetCompatibility(id, fileType string, compatibility map[string]int) {
	var dataMap = make(map[string]interface{})

	switch fileType {
	case "m3u":
		dataMap = config.Settings.Files.M3U

	case "hdhr":
		dataMap = config.Settings.Files.HDHR

	case "xmltv":
		dataMap = config.Settings.Files.XMLTV
	}

	if data, ok := dataMap[id].(map[string]interface{}); ok {

		data["compatibility"] = compatibility

		switch fileType {
		case "m3u":
			config.Settings.Files.M3U = dataMap
		case "hdhr":
			config.Settings.Files.HDHR = dataMap
		case "xmltv":
			config.Settings.Files.XMLTV = dataMap
		}

		err := settings.SaveSettings(config.Settings)
		if err != nil {
			cli.ShowError(err, 0)
		}
	}

}

// Provider XMLTV Datei überprüfen
func checkXMLCompatibility(id string, body []byte) (err error) {

	var xmltv structs.XMLTV
	var compatibility = make(map[string]int)

	err = xml.Unmarshal(body, &xmltv)
	if err != nil {
		return
	}

	compatibility["xmltv.channels"] = len(xmltv.Channel)
	compatibility["xmltv.programs"] = len(xmltv.Program)

	SetCompatibility(id, "xmltv", compatibility)

	return
}
