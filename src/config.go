package src

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/config"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Init : Systeminitialisierung
func Init() (err error) {

	var debug string

	// System Einstellungen
	config.System.AppName = strings.ToLower(config.System.Name)
	config.System.ARCH = runtime.GOARCH
	config.System.OS = runtime.GOOS
	config.System.ServerProtocol.API = "http"
	config.System.ServerProtocol.DVR = "http"
	config.System.ServerProtocol.M3U = "http"
	config.System.ServerProtocol.WEB = "http"
	config.System.ServerProtocol.XML = "http"
	config.System.PlexChannelLimit = 480
	config.System.UnfilteredChannelLimit = 480
	config.System.Compatibility = "0.1.0"

	// FFmpeg Default Einstellungen
	config.System.FFmpeg.DefaultOptions = "-hide_banner -loglevel error -analyzeduration 1000000 -probesize 1000000 -i [URL] -map 0:v -map 0:a:0 -c:v copy -c:a aac -b:a 192k -ac 2 -c:s copy -f mpegts -fflags +genpts -movflags +faststart -copyts pipe:1"
	config.System.VLC.DefaultOptions = "-I dummy [URL] --sout #std{mux=ts,access=file,dst=-}"

	// Default Logeinträge, wird später von denen aus der settings.json überschrieben. Muss gemacht werden, damit die ersten Einträge auch im Log (webUI aangezeigt werden)
	config.Settings.LogEntriesRAM = 500

	// Variablen für den Update Prozess
	//System.Update.Git = "https://github.com/Threadfin/Threadfin/blob"
	config.System.Update.Git = fmt.Sprintf("https://github.com/%s/%s", config.System.GitHub.User, config.System.GitHub.Repo)
	config.System.Update.Github = fmt.Sprintf("https://api.github.com/repos/%s/%s", config.System.GitHub.User, config.System.GitHub.Repo)
	config.System.Update.Name = "Threadfin"

	// Ordnerpfade festlegen
	var tempFolder = os.TempDir() + string(os.PathSeparator) + config.System.AppName + string(os.PathSeparator)
	tempFolder = getPlatformPath(strings.ReplaceAll(tempFolder, "//", "/"))

	if len(config.System.Folder.Config) == 0 {
		config.System.Folder.Config = GetUserHomeDirectory() + string(os.PathSeparator) + "." + config.System.AppName + string(os.PathSeparator)
	} else {
		config.System.Folder.Config = strings.TrimRight(config.System.Folder.Config, string(os.PathSeparator)) + string(os.PathSeparator)
	}

	config.System.Folder.Config = getPlatformPath(config.System.Folder.Config)

	config.System.Folder.Backup = config.System.Folder.Config + "backup" + string(os.PathSeparator)
	config.System.Folder.Data = config.System.Folder.Config + "data" + string(os.PathSeparator)
	config.System.Folder.Cache = config.System.Folder.Config + "cache" + string(os.PathSeparator)
	config.System.Folder.ImagesCache = config.System.Folder.Cache + "images" + string(os.PathSeparator)
	config.System.Folder.ImagesUpload = config.System.Folder.Data + "images" + string(os.PathSeparator)
	config.System.Folder.Temp = tempFolder

	// Dev Info
	showDevInfo()

	// System Ordner erstellen
	err = createSystemFolders()
	if err != nil {
		cli.ShowError(err, 1070)
		return
	}

	if len(config.System.Flag.Restore) > 0 {
		// Einstellungen werden über CLI wiederhergestellt. Weitere Initialisierung ist nicht notwendig.
		return
	}

	config.System.File.XML = getPlatformFile(fmt.Sprintf("%s%s.xml", config.System.Folder.Data, config.System.AppName))
	config.System.File.M3U = getPlatformFile(fmt.Sprintf("%s%s.m3u", config.System.Folder.Data, config.System.AppName))

	config.System.Compressed.GZxml = getPlatformFile(fmt.Sprintf("%s%s.xml.gz", config.System.Folder.Data, config.System.AppName))

	err = activatedSystemAuthentication()
	if err != nil {
		return
	}

	err = resolveHostIP()
	if err != nil {
		cli.ShowError(err, 1002)
	}

	// Menü für das Webinterface
	config.System.WEB.Menu = []string{"playlist", "xmltv", "filter", "mapping", "users", "settings", "log", "logout"}

	fmt.Println("For help run: " + getPlatformFile(os.Args[0]) + " -h")
	fmt.Println()

	// Überprüfen ob Threadfin als root läuft
	if os.Geteuid() == 0 {
		cli.ShowWarning(2110)
	}

	if config.System.Flag.Debug > 0 {
		debug = fmt.Sprintf("Debug Level:%d", config.System.Flag.Debug)
		cli.ShowDebug(debug, 1)
	}

	cli.ShowInfo(fmt.Sprintf("Version:%s Build: %s", config.System.Version, config.System.Build))
	cli.ShowInfo(fmt.Sprintf("Database Version:%s", config.System.DBVersion))
	cli.ShowInfo(fmt.Sprintf("System IP Addresses:IPv4: %d | IPv6: %d", len(config.System.IPAddressesV4), len(config.System.IPAddressesV6)))
	cli.ShowInfo("Hostname:" + config.System.Hostname)
	cli.ShowInfo(fmt.Sprintf("System Folder:%s", getPlatformPath(config.System.Folder.Config)))

	// Systemdateien erstellen (Falls nicht vorhanden)
	err = createSystemFiles()
	if err != nil {
		cli.ShowError(err, 1071)
		return
	}

	err = conditionalUpdateChanges()
	if err != nil {
		return
	}

	// Einstellungen laden (settings.json)
	cli.ShowInfo(fmt.Sprintf("Load Settings:%s", config.System.File.Settings))

	_, err = loadSettings()
	if err != nil {
		cli.ShowError(err, 0)
		return
	}

	// Separaten tmp Ordner für jede Instanz
	//System.Folder.Temp = System.Folder.Temp + Settings.UUID + string(os.PathSeparator)
	cli.ShowInfo(fmt.Sprintf("Temporary Folder:%s", getPlatformPath(config.System.Folder.Temp)))

	err = checkFolder(config.System.Folder.Temp)
	if err != nil {
		return
	}

	err = removeChildItems(getPlatformPath(config.System.Folder.Temp))
	if err != nil {
		return
	}

	// Branch festlegen
	config.System.Branch = cases.Title(language.English).String(config.Settings.Branch)

	if config.System.Dev {
		config.System.Branch = cases.Title(language.English).String("development")
	}

	if len(config.System.Branch) == 0 {
		config.System.Branch = cases.Title(language.English).String("main")
	}

	cli.ShowInfo(fmt.Sprintf("GitHub:https://github.com/%s", config.System.GitHub.User))
	cli.ShowInfo(fmt.Sprintf("Git Branch:%s [%s]", config.System.Branch, config.System.GitHub.User))

	// Set base URI
	if config.Settings.HttpThreadfinDomain != "" {
		setGlobalDomain(getBaseUrl(config.Settings.HttpThreadfinDomain, config.Settings.Port))
	} else {
		setGlobalDomain(fmt.Sprintf("%s:%s", config.System.IPAddress, config.Settings.Port))
	}

	config.System.URLBase = fmt.Sprintf("%s://%s:%s", config.System.ServerProtocol.WEB, config.System.IPAddress, config.Settings.Port)

	// HTML Dateien erstellen, mit dev == true werden die lokalen HTML Dateien verwendet
	if config.System.Dev {

		HTMLInit("webUI", "src", "html"+string(os.PathSeparator), "src"+string(os.PathSeparator)+"webUI.go")
		err = BuildGoFile()
		if err != nil {
			return
		}

	}

	// DLNA Server starten
	if config.Settings.SSDP {
		err = SSDP()
		if err != nil {
			return
		}
	}

	// HTML Datein laden
	loadHTMLMap()

	return
}

// StartSystem : System wird gestartet
func StartSystem(updateProviderFiles bool) (err error) {

	setDeviceID()

	if config.System.ScanInProgress == 1 {
		return
	}

	// Systeminformationen in der Konsole ausgeben
	cli.ShowInfo(fmt.Sprintf("UUID:%s", config.Settings.UUID))
	cli.ShowInfo(fmt.Sprintf("Tuner (Plex / Emby):%d", config.Settings.Tuner))
	cli.ShowInfo(fmt.Sprintf("EPG Source:%s", config.Settings.EpgSource))
	cli.ShowInfo(fmt.Sprintf("Plex Channel Limit:%d", config.System.PlexChannelLimit))
	cli.ShowInfo(fmt.Sprintf("Unfiltered Chan. Limit:%d", config.System.UnfilteredChannelLimit))

	// Providerdaten aktualisieren
	if len(config.Settings.Files.M3U) > 0 && config.Settings.FilesUpdate || updateProviderFiles {

		err = ThreadfinAutoBackup()
		if err != nil {
			cli.ShowError(err, 1090)
		}

		err = getProviderData("m3u", "")
		if err != nil {
			cli.ShowError(err, 0)
			return
		}

		err = getProviderData("hdhr", "")
		if err != nil {
			cli.ShowError(err, 0)
			return
		}

		if config.Settings.EpgSource == "XEPG" {
			err = getProviderData("xmltv", "")
			if err != nil {
				cli.ShowError(err, 0)
				return
			}
		}

	}

	err = buildDatabaseDVR()
	if err != nil {
		cli.ShowError(err, 0)
		return
	}

	buildXEPG(true)

	return
}
