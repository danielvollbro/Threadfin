package system

import (
	"fmt"
	"strings"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/config"
	"threadfin/src/internal/dvr"
	"threadfin/src/internal/xepg"
)

// ShowSystemInfo : Systeminformationen anzeigen
func ShowSystemInfo() {

	fmt.Print("Creating the information takes a moment...")
	err := dvr.BuildDatabase()
	if err != nil {
		cli.ShowError(err, 0)
		return
	}

	xepg.BuildXEPG(false)

	fmt.Println("OK")
	println()

	fmt.Printf("Version:             %s %s.%s\n", config.System.Name, config.System.Version, config.System.Build)
	fmt.Printf("Branch:              %s\n", config.System.Branch)
	fmt.Printf("GitHub:              %s/%s | Git update = %t\n", config.System.GitHub.User, config.System.GitHub.Repo, config.System.GitHub.Update)
	fmt.Printf("Folder (config):     %s\n", config.System.Folder.Config)

	fmt.Printf("Streams:             %d / %d\n", len(config.Data.Streams.Active), len(config.Data.Streams.All))
	fmt.Printf("Filter:              %d\n", len(config.Data.Filter))
	fmt.Printf("XEPG Chanels:        %d\n", int(config.Data.XEPG.XEPGCount))

	println()
	fmt.Println("IPv4 Addresses:")

	for i, ipv4 := range config.System.IPAddressesV4 {

		switch count := i; {

		case count < 10:
			fmt.Printf("  %d.                 %s\n", count, ipv4)
		case count < 100:
			fmt.Printf("  %d.                %s\n", count, ipv4)

		}

	}

	println()
	fmt.Println("IPv6 Addresses:")

	for i, ipv4 := range config.System.IPAddressesV6 {

		switch count := i; {

		case count < 10:
			fmt.Printf("  %d.                 %s\n", count, ipv4)
		case count < 100:
			fmt.Printf("  %d.                %s\n", count, ipv4)
		}

	}

	println("---")

	fmt.Println("Settings [General]")
	fmt.Printf("Threadfin Update:        %t\n", config.Settings.ThreadfinAutoUpdate)
	fmt.Printf("UUID:                %s\n", config.Settings.UUID)
	fmt.Printf("Tuner (Plex / Emby): %d\n", config.Settings.Tuner)
	fmt.Printf("EPG Source:          %s\n", config.Settings.EpgSource)

	println("---")

	fmt.Println("Settings [Files]")
	fmt.Printf("Schedule:            %s\n", strings.Join(config.Settings.Update, ","))
	fmt.Printf("Files Update:        %t\n", config.Settings.FilesUpdate)
	fmt.Printf("Folder (tmp):        %s\n", config.Settings.TempPath)
	fmt.Printf("Image Chaching:      %t\n", config.Settings.CacheImages)
	fmt.Printf("Replace EPG Image:   %t\n", config.Settings.XepgReplaceMissingImages)

	println("---")

	fmt.Println("Settings [Streaming]")
	fmt.Printf("Buffer:              %s\n", config.Settings.Buffer)
	fmt.Printf("UDPxy:               %s\n", config.Settings.UDPxy)
	fmt.Printf("Buffer Size:         %d KB\n", config.Settings.BufferSize)
	fmt.Printf("Timeout:             %d ms\n", int(config.Settings.BufferTimeout))
	fmt.Printf("User Agent:          %s\n", config.Settings.UserAgent)

	println("---")

	fmt.Println("Settings [Backup]")
	fmt.Printf("Folder (backup):     %s\n", config.Settings.BackupPath)
	fmt.Printf("Backup Keep:         %d\n", config.Settings.BackupKeep)

}
