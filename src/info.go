package src

import (
	"fmt"
	"strings"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/config"
)

// ShowSystemInfo : Systeminformationen anzeigen
func ShowSystemInfo() {

	fmt.Print("Creating the information takes a moment...")
	err := buildDatabaseDVR()
	if err != nil {
		cli.ShowError(err, 0)
		return
	}

	buildXEPG(false)

	fmt.Println("OK")
	println()

	fmt.Println(fmt.Sprintf("Version:             %s %s.%s", config.System.Name, config.System.Version, config.System.Build))
	fmt.Println(fmt.Sprintf("Branch:              %s", config.System.Branch))
	fmt.Println(fmt.Sprintf("GitHub:              %s/%s | Git update = %t", config.System.GitHub.User, config.System.GitHub.Repo, config.System.GitHub.Update))
	fmt.Println(fmt.Sprintf("Folder (config):     %s", config.System.Folder.Config))

	fmt.Println(fmt.Sprintf("Streams:             %d / %d", len(config.Data.Streams.Active), len(config.Data.Streams.All)))
	fmt.Println(fmt.Sprintf("Filter:              %d", len(config.Data.Filter)))
	fmt.Println(fmt.Sprintf("XEPG Chanels:        %d", int(config.Data.XEPG.XEPGCount)))

	println()
	fmt.Println(fmt.Sprintf("IPv4 Addresses:"))

	for i, ipv4 := range config.System.IPAddressesV4 {

		switch count := i; {

		case count < 10:
			fmt.Println(fmt.Sprintf("  %d.                 %s", count, ipv4))
			break
		case count < 100:
			fmt.Println(fmt.Sprintf("  %d.                %s", count, ipv4))
			break

		}

	}

	println()
	fmt.Println(fmt.Sprintf("IPv6 Addresses:"))

	for i, ipv4 := range config.System.IPAddressesV6 {

		switch count := i; {

		case count < 10:
			fmt.Println(fmt.Sprintf("  %d.                 %s", count, ipv4))
			break
		case count < 100:
			fmt.Println(fmt.Sprintf("  %d.                %s", count, ipv4))
			break

		}

	}

	println("---")

	fmt.Println("Settings [General]")
	fmt.Println(fmt.Sprintf("Threadfin Update:        %t", config.Settings.ThreadfinAutoUpdate))
	fmt.Println(fmt.Sprintf("UUID:                %s", config.Settings.UUID))
	fmt.Println(fmt.Sprintf("Tuner (Plex / Emby): %d", config.Settings.Tuner))
	fmt.Println(fmt.Sprintf("EPG Source:          %s", config.Settings.EpgSource))

	println("---")

	fmt.Println("Settings [Files]")
	fmt.Println(fmt.Sprintf("Schedule:            %s", strings.Join(config.Settings.Update, ",")))
	fmt.Println(fmt.Sprintf("Files Update:        %t", config.Settings.FilesUpdate))
	fmt.Println(fmt.Sprintf("Folder (tmp):        %s", config.Settings.TempPath))
	fmt.Println(fmt.Sprintf("Image Chaching:      %t", config.Settings.CacheImages))
	fmt.Println(fmt.Sprintf("Replace EPG Image:   %t", config.Settings.XepgReplaceMissingImages))

	println("---")

	fmt.Println("Settings [Streaming]")
	fmt.Println(fmt.Sprintf("Buffer:              %s", config.Settings.Buffer))
	fmt.Println(fmt.Sprintf("UDPxy:               %s", config.Settings.UDPxy))
	fmt.Println(fmt.Sprintf("Buffer Size:         %d KB", config.Settings.BufferSize))
	fmt.Println(fmt.Sprintf("Timeout:             %d ms", int(config.Settings.BufferTimeout)))
	fmt.Println(fmt.Sprintf("User Agent:          %s", config.Settings.UserAgent))

	println("---")

	fmt.Println("Settings [Backup]")
	fmt.Println(fmt.Sprintf("Folder (backup):     %s", config.Settings.BackupPath))
	fmt.Println(fmt.Sprintf("Backup Keep:         %d", config.Settings.BackupKeep))

}
