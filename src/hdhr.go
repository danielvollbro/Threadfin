package src

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/config"
	jsonserializer "threadfin/src/internal/json-serializer"
	"threadfin/src/internal/structs"
)

func makeInteraceFromHDHR(content []byte, playlistName, id string) (channels []interface{}, err error) {

	var hdhrData []interface{}

	err = json.Unmarshal(content, &hdhrData)
	if err == nil {

		for _, d := range hdhrData {

			var channel = make(map[string]string)
			var data = d.(map[string]interface{})

			channel["group-title"] = playlistName
			channel["name"] = data["GuideName"].(string)
			channel["tvg-id"] = data["GuideName"].(string)
			channel["url"] = data["URL"].(string)
			channel["ID-"+id] = data["GuideNumber"].(string)
			channel["_uuid.key"] = "ID-" + id
			channel["_values"] = playlistName + " " + channel["name"]

			channels = append(channels, channel)

		}

	}

	return
}

func getCapability() (xmlContent []byte, err error) {

	var capability structs.Capability
	var buffer bytes.Buffer

	capability.Xmlns = "urn:schemas-upnp-org:device-1-0"
	capability.URLBase = config.System.ServerProtocol.WEB + "://" + config.System.Domain

	capability.SpecVersion.Major = 1
	capability.SpecVersion.Minor = 0

	capability.Device.DeviceType = "urn:schemas-upnp-org:device:MediaServer:1"
	capability.Device.FriendlyName = config.System.Name
	capability.Device.Manufacturer = "Silicondust"
	capability.Device.ModelName = "HDTC-2US"
	capability.Device.ModelNumber = "HDTC-2US"
	capability.Device.SerialNumber = ""
	capability.Device.UDN = "uuid:" + config.System.DeviceID

	output, err := xml.MarshalIndent(capability, " ", "  ")
	if err != nil {
		cli.ShowError(err, 1003)
	}

	buffer.Write([]byte(xml.Header))
	buffer.Write([]byte(output))
	xmlContent = buffer.Bytes()

	return
}

func getDiscover() (jsonContent []byte, err error) {

	var discover structs.Discover

	discover.BaseURL = config.System.ServerProtocol.WEB + "://" + config.System.Domain
	discover.DeviceAuth = config.System.AppName
	discover.DeviceID = config.System.DeviceID
	discover.FirmwareName = "bin_" + config.System.Version
	discover.FirmwareVersion = config.System.Version
	discover.FriendlyName = config.System.Name

	discover.LineupURL = fmt.Sprintf("%s://%s/lineup.json", config.System.ServerProtocol.DVR, config.System.Domain)
	discover.Manufacturer = "Golang"
	discover.ModelNumber = config.System.Version
	discover.TunerCount = config.Settings.Tuner

	jsonContent, err = json.MarshalIndent(discover, "", "  ")

	return
}

func getLineupStatus() (jsonContent []byte, err error) {

	var lineupStatus structs.LineupStatus

	lineupStatus.ScanInProgress = config.System.ScanInProgress
	lineupStatus.ScanPossible = 0
	lineupStatus.Source = "Cable"
	lineupStatus.SourceList = []string{"Cable"}

	jsonContent, err = json.MarshalIndent(lineupStatus, "", "  ")

	return
}

func getLineup() (jsonContent []byte, err error) {

	var lineup structs.Lineup

	switch config.Settings.EpgSource {

	case "PMS":
		for i, dsa := range config.Data.Streams.Active {

			var m3uChannel structs.M3UChannelStructXEPG

			err = json.Unmarshal([]byte(jsonserializer.MapToJSON(dsa)), &m3uChannel)
			if err != nil {
				return
			}

			var stream structs.LineupStream
			stream.GuideName = m3uChannel.Name
			switch len(m3uChannel.UUIDValue) {

			case 0:
				stream.GuideNumber = fmt.Sprintf("%d", i+1000)
				guideNumber, err := getGuideNumberPMS(stream.GuideName)
				if err != nil {
					cli.ShowError(err, 0)
				}

				stream.GuideNumber = guideNumber

			default:
				stream.GuideNumber = m3uChannel.UUIDValue

			}

			stream.URL, err = createStreamingURL("DVR", m3uChannel.FileM3UID, stream.GuideNumber, m3uChannel.Name, m3uChannel.URL, nil, nil, nil)
			if err == nil {
				lineup = append(lineup, stream)
			} else {
				cli.ShowError(err, 1202)
			}

		}

	case "XEPG":
		for _, dxc := range config.Data.XEPG.Channels {

			var xepgChannel structs.XEPGChannelStruct
			err = json.Unmarshal([]byte(jsonserializer.MapToJSON(dxc)), &xepgChannel)
			if err != nil {
				return
			}

			if xepgChannel.XActive && !xepgChannel.XHideChannel {
				var stream structs.LineupStream
				stream.GuideName = xepgChannel.XName
				stream.GuideNumber = xepgChannel.XChannelID
				stream.URL, err = createStreamingURL("DVR", xepgChannel.FileM3UID, xepgChannel.XChannelID, xepgChannel.XName, xepgChannel.URL, xepgChannel.BackupChannel1, xepgChannel.BackupChannel2, xepgChannel.BackupChannel3)
				if err == nil {
					lineup = append(lineup, stream)
				} else {
					cli.ShowError(err, 1202)
				}

			}

		}

	}

	jsonContent, err = json.MarshalIndent(lineup, "", "  ")
	if err != nil {
		return
	}

	config.Data.Cache.PMS = nil

	err = saveMapToJSONFile(config.System.File.URLS, config.Data.Cache.StreamingURLS)

	return
}

func getGuideNumberPMS(channelName string) (pmsID string, err error) {

	if len(config.Data.Cache.PMS) == 0 {

		config.Data.Cache.PMS = make(map[string]string)

		pms, err := loadJSONFileToMap(config.System.File.PMS)

		if err != nil {
			return "", err
		}

		for key, value := range pms {
			config.Data.Cache.PMS[key] = value.(string)
		}

	}

	var getNewID = func(channelName string) (id string) {

		var i int

	newID:

		var ids []string
		id = fmt.Sprintf("id-%d", i)

		for _, v := range config.Data.Cache.PMS {
			ids = append(ids, v)
		}

		if indexOfString(id, ids) != -1 {
			i++
			goto newID
		}

		return
	}

	if value, ok := config.Data.Cache.PMS[channelName]; ok {

		pmsID = value

	} else {

		pmsID = getNewID(channelName)
		config.Data.Cache.PMS[channelName] = pmsID
		err = saveMapToJSONFile(config.System.File.PMS, config.Data.Cache.PMS)
	}

	return
}
