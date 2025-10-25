package system

import (
	"fmt"
	"reflect"
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
