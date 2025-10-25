package system

import (
	"fmt"
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
