package src

import (
	"log"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/config"
	"threadfin/src/internal/provider"
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
