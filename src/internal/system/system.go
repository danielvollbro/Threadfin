package system

import (
	"fmt"
	"log"
	"reflect"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/config"
	"threadfin/src/internal/storage"
)

// Show developer info
func ShowDevInfo() {
	if !config.System.Dev {
		return
	}

	fmt.Print("\033[31m")
	fmt.Println("* * * * * D E V   M O D E * * * * *")
	fmt.Println("Version: ", config.System.Version)
	fmt.Println("Build:   ", config.System.Build)
	fmt.Println("* * * * * * * * * * * * * * * * * *")
	fmt.Print("\033[0m")
	fmt.Println()
}

func CreateSystemFolders() (err error) {
	e := reflect.ValueOf(&config.System.Folder).Elem()

	for i := 0; i < e.NumField(); i++ {

		var folder = e.Field(i).Interface().(string)

		err = storage.CheckFolder(folder)

		if err != nil {
			return
		}

	}

	return
}

func CreateSystemFiles() (err error) {
	var debug string
	for _, file := range config.SystemFiles {

		var filename = storage.GetPlatformFile(config.System.Folder.Config + file)

		err = storage.CheckFile(filename)
		if err != nil {
			// File does not exist, will be created now
			err = storage.SaveMapToJSONFile(filename, make(map[string]interface{}))
			if err != nil {
				return
			}

			debug = fmt.Sprintf("Create File:%s", filename)
			cli.ShowDebug(debug, 1)

		}

		switch file {

		case "authentication.json":
			config.System.File.Authentication = filename
		case "pms.json":
			config.System.File.PMS = filename
		case "settings.json":
			config.System.File.Settings = filename
		case "xepg.json":
			config.System.File.XEPG = filename
		case "urls.json":
			config.System.File.URLS = filename

		}

	}

	return
}

// Zugriff über die Domain ermöglichen
func SetGlobalDomain(domain string) {

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
