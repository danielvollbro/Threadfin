package cli

import (
	"fmt"
	"log"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"threadfin/src/internal/config"
	"threadfin/src/internal/structs"
	"time"
)

func ShowInfo(str string) {
	config.InfoMutex.Lock()
	defer config.InfoMutex.Unlock()

	if config.System.Flag.Info {
		return
	}

	var max = 23
	var msg = strings.SplitN(str, ":", 2)
	var length = len(msg[0])
	var space string

	if len(msg) != 2 {
		return
	}

	for i := length; i < max; i++ {
		space = space + " "
	}

	msg[0] = msg[0] + ":" + space

	var logMsg = fmt.Sprintf("[%s] %s%s", config.System.Name, msg[0], msg[1])

	printLogOnScreen(logMsg, "info")

	logMsg = strings.ReplaceAll(logMsg, " ", "&nbsp;")
	config.WebScreenLog.Log = append(config.WebScreenLog.Log, time.Now().Format("2006-01-02 15:04:05")+" "+logMsg)
	logCleanUp()
}

func ShowDebug(str string, level int) {
	if config.System.Flag.Debug < level {
		return
	}

	var max = 23
	var msg = strings.SplitN(str, ":", 2)
	var length = len(msg[0])
	var space string
	var mutex = sync.RWMutex{}

	if len(msg) != 2 {
		return
	}

	for i := length; i < max; i++ {
		space = space + " "
	}
	msg[0] = msg[0] + ":" + space

	var logMsg = fmt.Sprintf("[DEBUG] %s%s", msg[0], msg[1])

	printLogOnScreen(logMsg, "debug")

	mutex.Lock()
	logMsg = strings.ReplaceAll(logMsg, " ", "&nbsp;")
	config.WebScreenLog.Log = append(config.WebScreenLog.Log, time.Now().Format("2006-01-02 15:04:05")+" "+logMsg)
	logCleanUp()
	mutex.Unlock()
}

func ShowHighlight(str string) {
	var max = 23
	var msg = strings.SplitN(str, ":", 2)
	var length = len(msg[0])
	var space string

	var notification structs.Notification
	notification.Type = "info"

	if len(msg) == 2 {
		for i := length; i < max; i++ {
			space = space + " "
		}

		msg[0] = msg[0] + ":" + space

		var logMsg = fmt.Sprintf("[%s] %s%s", config.System.Name, msg[0], msg[1])

		printLogOnScreen(logMsg, "highlight")

	} else {
		printLogOnScreen(str, "highlight")
	}

	notification.Type = "info"
	if len(msg) > 1 {
		notification.Message = msg[1]
	} else {
		notification.Message = str
	}

	addNotification(notification)
}

func ShowWarning(errCode int) {
	var errMsg = GetErrMsg(errCode)
	var logMsg = fmt.Sprintf("[%s] [WARNING] %s", config.System.Name, errMsg)
	var mutex = sync.RWMutex{}

	printLogOnScreen(logMsg, "warning")

	mutex.Lock()
	config.WebScreenLog.Log = append(config.WebScreenLog.Log, time.Now().Format("2006-01-02 15:04:05")+" "+logMsg)
	config.WebScreenLog.Warnings++
	mutex.Unlock()
}

// ShowError : Zeigt die Fehlermeldungen in der Konsole
func ShowError(err error, errCode int) {
	var mutex = sync.RWMutex{}

	var errMsg = GetErrMsg(errCode)
	var logMsg = fmt.Sprintf("[%s] [ERROR] %s (%s) - EC: %d", config.System.Name, err, errMsg, errCode)

	printLogOnScreen(logMsg, "error")

	mutex.Lock()
	config.WebScreenLog.Log = append(config.WebScreenLog.Log, time.Now().Format("2006-01-02 15:04:05")+" "+logMsg)
	config.WebScreenLog.Errors++
	mutex.Unlock()
}

func printLogOnScreen(logMsg string, logType string) {
	var color string

	switch logType {
	case "info":
		color = "\033[0m"

	case "debug":
		color = "\033[35m"

	case "highlight":
		color = "\033[32m"

	case "warning":
		color = "\033[33m"

	case "error":
		color = "\033[31m"
	}

	switch runtime.GOOS {
	case "windows":
		log.Println(logMsg)

	default:
		fmt.Print(color)
		log.Println(logMsg)
		fmt.Print("\033[0m")
	}
}

func logCleanUp() {
	config.LogMutex.Lock()
	defer config.LogMutex.Unlock()

	var logEntriesRAM = config.Settings.LogEntriesRAM
	var logs = config.WebScreenLog.Log

	config.WebScreenLog.Warnings = 0
	config.WebScreenLog.Errors = 0

	if len(logs) > logEntriesRAM {

		var tmp = make([]string, 0)
		for i := len(logs) - logEntriesRAM; i < logEntriesRAM; i++ {
			tmp = append(tmp, logs[i])
		}

		logs = tmp
	}

	for _, log := range logs {

		if strings.Contains(log, "WARNING") {
			config.WebScreenLog.Warnings++
		}

		if strings.Contains(log, "ERROR") {
			config.WebScreenLog.Errors++
		}

	}

	config.WebScreenLog.Log = logs
}

// Fehlercodes
func GetErrMsg(errCode int) (errMsg string) {
	switch errCode {

	case 0:
		return

	// Errors
	case 1001:
		errMsg = "Web server could not be started."
	case 1002:
		errMsg = "No local IP address found."
	case 1003:
		errMsg = "Invalid xml"
	case 1004:
		errMsg = "File not found"
	case 1005:
		errMsg = "Invalid M3U file, an extended M3U file is required."
	case 1006:
		errMsg = "No playlist!"
	case 1007:
		errMsg = "XEPG requires an XMLTV file."
	case 1010:
		errMsg = "Invalid file compression"
	case 1011:
		errMsg = fmt.Sprintf("Data is corrupt or unavailable, %s now uses an older version of this file", config.System.Name)
	case 1012:
		errMsg = "Invalid formatting of the time"
	case 1013:
		errMsg = fmt.Sprintf("Invalid settings file (settings.json), file must be at least version %s", config.System.Compatibility)
	case 1014:
		errMsg = "Invalid filter rule"

	case 1020:
		errMsg = "Data could not be saved, invalid keyword"

	// Datenbank Update
	case 1030:
		errMsg = fmt.Sprintf("Invalid settings file (%s)", config.System.File.Settings)
	case 1031:
		errMsg = "Database error. The database version of your settings is not compatible with this version."

	// M3U Parser
	case 1050:
		errMsg = "Invalid duration specification in the M3U8 playlist."

	case 1060:
		errMsg = "Invalid characters found in the tvg parameters, streams with invalid parameters were skipped."

	// Dateisystem
	case 1070:
		errMsg = "Folder could not be created."
	case 1071:
		errMsg = "File could not be created"
	case 1072:
		errMsg = "File not found"

	// Backup
	case 1090:
		errMsg = "Automatic backup failed"

	// Websockets
	case 1100:
		errMsg = "WebUI build error"
	case 1101:
		errMsg = "WebUI request error"
	case 1102:
		errMsg = "WebUI response error"

	// PMS Guide Numbers
	case 1200:
		errMsg = "Could not create file"

	// Stream URL Fehler
	case 1201:
		errMsg = "Plex stream error"
	case 1202:
		errMsg = "Steaming URL could not be found in any playlist"
	case 1203:
		errMsg = "Steaming URL could not be found in any playlist"
	case 1204:
		errMsg = "Streaming was stopped by third party transcoder (FFmpeg / VLC)"

	// Warnings
	case 2000:
		errMsg = fmt.Sprintf("Plex can not handle more than %d streams. If you do not use Plex, you can ignore this warning.", config.System.PlexChannelLimit)
	case 2001:
		errMsg = fmt.Sprintf("%s has loaded more than %d streams. Use the filter to reduce the number of streams.", config.System.Name, config.System.UnfilteredChannelLimit)
	case 2002:
		errMsg = "PMS can not play m3u8 streams"
	case 2003:
		errMsg = "PMS can not play streams over RTSP."
	case 2004:
		errMsg = "Buffer is disabled for this stream."
	case 2005:
		errMsg = "There are no channels mapped, use the mapping menu to assign EPG data to the channels."
	case 2010:
		errMsg = "No valid streaming URL"
	case 2020:
		errMsg = "FFmpeg binary was not found. Check the FFmpeg binary path in the Threadfin settings."
	case 2021:
		errMsg = "VLC binary was not found. Check the VLC path binary in the Threadfin settings."

	case 2098:
		errMsg = "Updates are disabled in the settings"
	case 2099:
		errMsg = "Updates have been disabled by the developer"

	// Tuner
	case 2105:
		errMsg = fmt.Sprintf("The number of tuners has changed, you have to delete " + config.System.Name + " in Plex / Emby HDHR and set it up again.")
	case 2106:
		errMsg = "This function is only available with XEPG as EPG source"

	case 2110:
		errMsg = "Don't run this as Root!"

	case 2300:
		errMsg = "No channel logo found in the XMLTV or M3U file."
	case 2301:
		errMsg = "XMLTV file no longer available, channel has been deactivated."
	case 2302:
		errMsg = "Channel ID in the XMLTV file has changed. Channel has been deactivated."

	// Benutzerauthentifizierung
	case 3000:
		errMsg = "Database for user authentication could not be initialized."
	case 3001:
		errMsg = "The user has no authorization to load the channels."

	// Buffer
	case 4000:
		errMsg = "Connection to streaming source was interrupted."
	case 4001:
		errMsg = "Too many errors connecting to the provider. Streaming is canceled."
	case 4002:
		errMsg = "New URL for the redirect to the streaming server is missing"
	case 4003:
		errMsg = "Server sends an incompatible content-type"
	case 4004:
		errMsg = "This error message comes from the provider"
	case 4005:
		errMsg = "Temporary buffer files could not be deleted"
	case 4006:
		errMsg = "Server connection timeout"
	case 4007:
		errMsg = "Old temporary buffer file could not be deleted"

	// Buffer (M3U8)
	case 4050:
		errMsg = "Invalid M3U8 file"
	case 4051:
		errMsg = "#EXTM3U header is missing"

	// Caching
	case 4100:
		errMsg = "Unknown content type for downloaded image"
	case 4101:
		errMsg = "Invalid URL, original URL is used for this image"

	// API
	case 5000:
		errMsg = "Invalid API command"

	// Update Server
	case 6001:
		errMsg = "Ivalid key"
	case 6002:
		errMsg = "Update failed"
	case 6003:
		errMsg = "Update server not available"
	case 6004:
		errMsg = "Threadfin update available"

	default:
		errMsg = fmt.Sprintf("Unknown error / warning (%d)", errCode)
	}

	return errMsg
}

func addNotification(notification structs.Notification) (err error) {
	var i int
	var t = time.Now().UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
	notification.Time = strconv.FormatInt(t, 10)
	notification.New = true

	if len(notification.Headline) == 0 {
		notification.Headline = strings.ToUpper(notification.Type)
	}

	if len(config.System.Notification) == 0 {
		config.System.Notification = make(map[string]structs.Notification)
	}

	config.System.Notification[notification.Time] = notification

	for key := range config.System.Notification {
		if i < len(config.System.Notification)-10 {
			delete(config.System.Notification, key)
		}

		i++
	}

	return
}
