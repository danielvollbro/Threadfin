package system

import (
	"fmt"
	"threadfin/src/internal/config"
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
