package src

import (
	"fmt"
	"log"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/config"
	"threadfin/src/internal/crypt"
	"threadfin/src/internal/provider"
	"threadfin/src/internal/structs"
)

func updateUrlsJson() {

	err := provider.GetData("m3u", "")
	if err != nil {
		cli.ShowError(err, 0)
		return
	}

	err = provider.GetData("hdhr", "")
	if err != nil {
		cli.ShowError(err, 0)
		return
	}

	if config.Settings.EpgSource == "XEPG" {
		err = provider.GetData("xmltv", "")
		if err != nil {
			cli.ShowError(err, 0)
			return
		}
	}
	err = buildDatabaseDVR()
	if err != nil {
		cli.ShowError(err, 0)
		return
	}

	buildXEPG(false)
}

// Zugriff über die Domain ermöglichen
func setGlobalDomain(domain string) {

	config.System.Domain = domain

	switch config.Settings.AuthenticationPMS {
	case true:
		config.System.Addresses.DVR = "username:password@" + config.System.Domain
	case false:
		config.System.Addresses.DVR = config.System.Domain
	}

	switch config.Settings.AuthenticationM3U {
	case true:
		config.System.Addresses.M3U = config.System.ServerProtocol.M3U + "://" + config.System.Domain + "/m3u/threadfin.m3u?username=xxx&password=yyy"
	case false:
		config.System.Addresses.M3U = config.System.ServerProtocol.M3U + "://" + config.System.Domain + "/m3u/threadfin.m3u"
	}

	switch config.Settings.AuthenticationXML {
	case true:
		config.System.Addresses.XML = config.System.ServerProtocol.XML + "://" + config.System.Domain + "/xmltv/threadfin.xml?username=xxx&password=yyy"
	case false:
		config.System.Addresses.XML = config.System.ServerProtocol.XML + "://" + config.System.Domain + "/xmltv/threadfin.xml"
	}

	if config.Settings.EpgSource != "XEPG" {
		log.Println("SOURCE: ", config.Settings.EpgSource)
		config.System.Addresses.M3U = cli.GetErrMsg(2106)
		config.System.Addresses.XML = cli.GetErrMsg(2106)
	}
}

// Provider Streaming-URL zu Threadfin Streaming-URL konvertieren
func createStreamingURL(streamingType, playlistID, channelNumber, channelName, url string, backup_channel_1 *structs.BackupStream, backup_channel_2 *structs.BackupStream, backup_channel_3 *structs.BackupStream) (streamingURL string, err error) {

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
