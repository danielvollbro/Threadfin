package src

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"threadfin/src/internal/authentication"
	"threadfin/src/internal/buffer"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/client"
	"threadfin/src/internal/config"
	jsonserializer "threadfin/src/internal/json-serializer"
	"threadfin/src/internal/media"
	"threadfin/src/internal/playlist"
	"threadfin/src/internal/storage"
	"threadfin/src/internal/stream"
	"threadfin/src/internal/structs"
	"threadfin/src/internal/utilities"
	"threadfin/src/web"

	"github.com/gorilla/websocket"
)

// StartWebserver : Startet den Webserver
func StartWebserver() (err error) {
	config.SystemMutex.Lock()
	port := config.Settings.Port
	ipAddress := config.System.IPAddress
	if config.Settings.BindIpAddress != "" {
		ipAddress = config.Settings.BindIpAddress
	}
	config.SystemMutex.Unlock()

	http.HandleFunc("/", Index)
	http.HandleFunc("/stream/", Stream)
	http.HandleFunc("/xmltv/", Threadfin)
	http.HandleFunc("/m3u/", Threadfin)
	http.HandleFunc("/data/", WS)
	http.HandleFunc("/web/", Web)
	http.HandleFunc("/download/", Download)
	http.HandleFunc("/api/", API)
	http.HandleFunc("/images/", Images)
	http.HandleFunc("/data_images/", DataImages)
	http.HandleFunc("/ppv/enable", enablePPV)
	http.HandleFunc("/ppv/disable", disablePPV)
	http.HandleFunc("/auto/", Auto)

	config.SystemMutex.Lock()
	ips := len(config.System.IPAddressesV4) + len(config.System.IPAddressesV6) - 1
	switch ips {
	case 0:
		cli.ShowHighlight(fmt.Sprintf("Web Interface:%s://%s:%s/web/", config.System.ServerProtocol.WEB, ipAddress, config.Settings.Port))
	case 1:
		cli.ShowHighlight(fmt.Sprintf("Web Interface:%s://%s:%s/web/ | Threadfin is also available via the other %d IP.", config.System.ServerProtocol.WEB, ipAddress, config.Settings.Port, ips))
	default:
		cli.ShowHighlight(fmt.Sprintf("Web Interface:%s://%s:%s/web/ | Threadfin is also available via the other %d IP's.", config.System.ServerProtocol.WEB, ipAddress, config.Settings.Port, len(config.System.IPAddressesV4)+len(config.System.IPAddressesV6)-1))
	}
	config.SystemMutex.Unlock()

	if err = http.ListenAndServe(ipAddress+":"+port, nil); err != nil {
		cli.ShowError(err, 1001)
		return
	}

	return
}

// Index : Web Server /
func Index(w http.ResponseWriter, r *http.Request) {
	var err error
	var response []byte
	var path = r.URL.Path

	config.SystemMutex.Lock()
	if config.Settings.HttpThreadfinDomain != "" {
		setGlobalDomain(getBaseUrl(config.Settings.HttpThreadfinDomain, config.Settings.Port))
	} else {
		setGlobalDomain(r.Host)
	}
	config.SystemMutex.Unlock()

	switch path {
	case "/discover.json":
		response, err = getDiscover()
		w.Header().Set("Content-Type", "application/json")
	case "/lineup_status.json":
		response, err = getLineupStatus()
		w.Header().Set("Content-Type", "application/json")
	case "/lineup.json":
		config.SystemMutex.Lock()
		if config.Settings.AuthenticationPMS {
			config.SystemMutex.Unlock()
			_, err := basicAuth(r, "authentication.pms")
			if err != nil {
				cli.ShowError(err, 000)
				web.HttpStatusError(w, 403)
				return
			}
		} else {
			config.SystemMutex.Unlock()
		}
		response, err = getLineup()
		w.Header().Set("Content-Type", "application/json")
	case "/device.xml", "/capability":
		response, err = getCapability()
		w.Header().Set("Content-Type", "application/xml")
	default:
		response, err = getCapability()
		w.Header().Set("Content-Type", "application/xml")
	}

	if err == nil {
		w.WriteHeader(200)
		_, err = w.Write(response)
		if err != nil {
			cli.ShowError(err, 000)
		}
		return
	}

	web.HttpStatusError(w, 500)
}

// Stream : Web Server /stream/
func Stream(w http.ResponseWriter, r *http.Request) {
	var path = strings.Replace(r.RequestURI, "/stream/", "", 1)
	streamInfo, err := stream.GetStreamInfo(path)
	if err != nil {
		cli.ShowError(err, 1203)
		web.HttpStatusError(w, 404)
		return
	}

	// If an UDPxy host is set, and the stream URL is multicast (i.e. starts with 'udp://@'),
	// then streamInfo.URL needs to be rewritten to point to UDPxy.
	if config.Settings.UDPxy != "" && strings.HasPrefix(streamInfo.URL, "udp://@") {
		streamInfo.URL = fmt.Sprintf("http://%s/udp/%s/", config.Settings.UDPxy, strings.TrimPrefix(streamInfo.URL, "udp://@"))
	}

	config.SystemMutex.Lock()
	forceHttps := config.Settings.ForceHttps
	noStreamHttps := config.Settings.ExcludeStreamHttps
	config.SystemMutex.Unlock()

	// Dont Change Source M3Us to use HTTPs when forceHttps set and Exclude Streams from https
	if forceHttps && !noStreamHttps {
		u, err := url.Parse(streamInfo.URL)
		if err == nil {
			u.Scheme = "https"
			hostSplit := strings.Split(u.Host, ":")
			if len(hostSplit) > 0 {
				u.Host = hostSplit[0]
			}
			streamInfo.URL = fmt.Sprintf("https://%s:%d%s?%s", u.Host, config.Settings.HttpsPort, u.Path, u.RawQuery)
		}
	}

	if r.Method == "HEAD" {
		client := &http.Client{}
		req, err := http.NewRequest("HEAD", streamInfo.URL, nil)
		if err != nil {
			cli.ShowError(err, 1501)
			web.HttpStatusError(w, 405)
			return
		}
		resp, err := client.Do(req)
		if err != nil {
			cli.ShowError(err, 1502)
			web.HttpStatusError(w, 405)
			return
		}

		defer func() {
			err = resp.Body.Close()
		}()
		if err != nil {
			cli.ShowError(err, 1503)
			web.HttpStatusError(w, 405)
			return
		}

		// Copy headers from the source HEAD response to the outgoing response
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		return
	}

	var playListBuffer string
	config.SystemMutex.Lock()
	playListInterface := config.Settings.Files.M3U[streamInfo.PlaylistID]
	if playListInterface == nil {
		playListInterface = config.Settings.Files.HDHR[streamInfo.PlaylistID]
	}

	if playListMap, ok := playListInterface.(map[string]interface{}); ok {
		if bufferValue, exists := playListMap["buffer"]; exists && bufferValue != nil {
			if buffer, ok := bufferValue.(string); ok {
				playListBuffer = buffer
			}
		}
	}
	config.SystemMutex.Unlock()

	switch playListBuffer {
	case "-":
		cli.ShowInfo(fmt.Sprintf("Buffer:false [%s]", playListBuffer))
	case "threadfin":
		if strings.Contains(streamInfo.URL, "rtsp://") || strings.Contains(streamInfo.URL, "rtp://") {
			err = errors.New("RTSP and RTP streams are not supported")
			cli.ShowError(err, 2004)
			cli.ShowInfo("Streaming URL:" + streamInfo.URL)
			http.Redirect(w, r, streamInfo.URL, http.StatusFound)
			return
		}
		cli.ShowInfo(fmt.Sprintf("Buffer:true [%s]", playListBuffer))
	default:
		cli.ShowInfo(fmt.Sprintf("Buffer:true [%s]", playListBuffer))
	}

	cli.ShowInfo(fmt.Sprintf("Channel Name:%s", streamInfo.Name))
	cli.ShowInfo(fmt.Sprintf("Client User-Agent:%s", r.Header.Get("User-Agent")))

	switch playListBuffer {
	case "-":
		cli.ShowInfo("Streaming URL:" + streamInfo.URL)
		w.Header().Set("Access-Control-Allow-Origin", "*")
		http.Redirect(w, r, streamInfo.URL, http.StatusFound)
		cli.ShowInfo("Streaming Info:URL was passed to the client.")
		cli.ShowInfo("Streaming Info:Threadfin is no longer involved, the client connects directly to the streaming server.")
	default:
		stream.Buffering(streamInfo.PlaylistID, streamInfo.URL, streamInfo.BackupChannel1, streamInfo.BackupChannel2, streamInfo.BackupChannel3, streamInfo.Name, w, r)
	}
}

// Auto : HDHR routing (wird derzeit nicht benutzt)
func Auto(w http.ResponseWriter, r *http.Request) {
	var channelID = strings.Replace(r.RequestURI, "/auto/v", "", 1)
	fmt.Println(channelID)
}

// Threadfin : Web Server /xmltv/ und /m3u/
func Threadfin(w http.ResponseWriter, r *http.Request) {

	var requestType, groupTitle, file, content, contentType string
	var err error
	var path = strings.TrimPrefix(r.URL.Path, "/")
	var groups = []string{}

	config.SystemMutex.Lock()
	if config.Settings.HttpThreadfinDomain != "" {
		setGlobalDomain(getBaseUrl(config.Settings.HttpThreadfinDomain, config.Settings.Port))
	} else {
		setGlobalDomain(r.Host)
	}
	config.SystemMutex.Unlock()

	// XMLTV Datei
	if strings.Contains(path, "xmltv/") {

		requestType = "xml"

		err = urlAuth(r, requestType)
		if err != nil {
			cli.ShowError(err, 000)
			web.HttpStatusError(w, 403)
			return
		}

		config.SystemMutex.Lock()
		file = config.System.Folder.Data + storage.GetFilenameFromPath(path)
		config.SystemMutex.Unlock()

		content, err = storage.ReadStringFromFile(file)
		if err != nil {
			web.HttpStatusError(w, 404)
			return
		}

	}

	// M3U Datei
	if strings.Contains(path, "m3u/") {

		requestType = "m3u"

		err = urlAuth(r, requestType)
		if err != nil {
			cli.ShowError(err, 000)
			web.HttpStatusError(w, 403)
			return
		}

		groupTitle = r.URL.Query().Get("group-title")

		config.SystemMutex.Lock()
		m3uFilePath := config.System.Folder.Data + "threadfin.m3u"
		config.SystemMutex.Unlock()

		queries := r.URL.Query()
		// Check if the m3u file exists
		if len(queries) == 0 {
			if _, err := os.Stat(m3uFilePath); err == nil {
				log.Println("Serving existing m3u file")
				http.ServeFile(w, r, m3uFilePath)
				return
			}
		}

		log.Println("M3U file does not exist, building new one")

		config.SystemMutex.Lock()
		if !config.System.Dev {
			// false: Dateiname wird im Header gesetzt
			// true: M3U wird direkt im Browser angezeigt
			w.Header().Set("Content-Disposition", "attachment; filename="+storage.GetFilenameFromPath(path))
		}
		config.SystemMutex.Unlock()

		if len(groupTitle) > 0 {
			groups = strings.Split(groupTitle, ",")
		}

		content, err = buildM3U(groups)
		if err != nil {
			cli.ShowError(err, 000)
		}

	}

	contentType = http.DetectContentType([]byte(content))
	if strings.Contains(strings.ToLower(contentType), "xml") {
		contentType = "application/xml; charset=utf-8"
	}

	w.Header().Set("Content-Type", contentType)

	if err == nil {
		_, err = w.Write([]byte(content))
		if err != nil {
			cli.ShowError(err, 000)
		}
	}
}

// Images : Image Cache /images/
func Images(w http.ResponseWriter, r *http.Request) {

	var path = strings.TrimPrefix(r.URL.Path, "/")
	config.SystemMutex.Lock()
	filePath := config.System.Folder.ImagesCache + storage.GetFilenameFromPath(path)
	config.SystemMutex.Unlock()

	content, err := storage.ReadByteFromFile(filePath)
	if err != nil {
		web.HttpStatusError(w, 404)
		return
	}

	w.Header().Add("Content-Type", getContentType(filePath))
	w.Header().Add("Content-Length", fmt.Sprintf("%d", len(content)))
	w.WriteHeader(200)
	_, err = w.Write(content)
	if err != nil {
		cli.ShowError(err, 000)
	}
}

// DataImages : Image Pfad für Logos / Bilder die hochgeladen wurden /data_images/
func DataImages(w http.ResponseWriter, r *http.Request) {

	var path = strings.TrimPrefix(r.URL.Path, "/")
	config.SystemMutex.Lock()
	filePath := config.System.Folder.ImagesUpload + storage.GetFilenameFromPath(path)
	config.SystemMutex.Unlock()

	content, err := storage.ReadByteFromFile(filePath)
	if err != nil {
		web.HttpStatusError(w, 404)
		return
	}

	w.Header().Add("Content-Type", getContentType(filePath))
	w.Header().Add("Content-Length", fmt.Sprintf("%d", len(content)))

	w.WriteHeader(200)
	_, err = w.Write(content)
	if err != nil {
		cli.ShowError(err, 000)
	}
}

// WS : Web Sockets /ws/
func WS(w http.ResponseWriter, r *http.Request) {

	var request structs.RequestStruct
	var response structs.ResponseStruct
	response.Status = true

	var newToken string

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			// Implement any custom origin validation logic here, if needed.
			return true
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		cli.ShowError(err, 0)
		http.Error(w, "Could not open websocket connection", http.StatusBadRequest)
		return
	}

	config.SystemMutex.Lock()
	if config.Settings.HttpThreadfinDomain != "" {
		setGlobalDomain(getBaseUrl(config.Settings.HttpThreadfinDomain, config.Settings.Port))
	} else {
		setGlobalDomain(r.Host)
	}
	config.SystemMutex.Unlock()

	for {

		err = conn.ReadJSON(&request)

		if err != nil {
			return
		}

		config.SystemMutex.Lock()
		if !config.System.ConfigurationWizard {

			switch config.Settings.AuthenticationWEB {

			// Token Authentication
			case true:

				var token string
				tokens, ok := r.URL.Query()["Token"]

				if !ok || len(tokens[0]) < 1 {
					token = "-"
				} else {
					token = tokens[0]
				}

				newToken, err = tokenAuthentication(token)
				if err != nil {
					response.Status = false
					response.Reload = true
					response.Error = err.Error()
					request.Cmd = "-"

					if err = conn.WriteJSON(response); err != nil {
						cli.ShowError(err, 1102)
					}

					config.SystemMutex.Unlock()
					return
				}

				response.Token = newToken
				response.Users, _ = authentication.GetAllUserData()

			}

		}
		config.SystemMutex.Unlock()

		switch request.Cmd {
		case "updateLog":
			response = setDefaultResponseData(response, false)
			if err = conn.WriteJSON(response); err != nil {
				cli.ShowError(err, 1022)
			} else {
				return
			}
			return

		// Data write commands
		case "saveSettings":
			var authenticationUpdate = config.Settings.AuthenticationWEB
			response.Settings, err = updateServerSettings(request)
			if err == nil {
				response.OpenMenu = strconv.Itoa(utilities.IndexOfString("settings", config.System.WEB.Menu))
				response.Reload = config.Settings.AuthenticationWEB && !authenticationUpdate

				buffer.InitVFS()
			}

		case "saveFilesM3U":
			// Reset cache for urls.json
			var filename = storage.GetPlatformFile(config.System.Folder.Config + "urls.json")
			err = storage.SaveMapToJSONFile(filename, make(map[string]structs.StreamInfo))
			if err != nil {
				cli.ShowError(err, 000)
				return
			}
			config.Data.Cache.StreamingURLS = make(map[string]structs.StreamInfo)

			err = saveFiles(request, "m3u")
			if err == nil {
				response.OpenMenu = strconv.Itoa(utilities.IndexOfString("playlist", config.System.WEB.Menu))
			}
			updateUrlsJson()

		case "updateFileM3U":
			// Reset cache for urls.json
			var filename = storage.GetPlatformFile(config.System.Folder.Config + "urls.json")
			err = storage.SaveMapToJSONFile(filename, make(map[string]structs.StreamInfo))
			if err != nil {
				cli.ShowError(err, 000)
				return
			}
			config.Data.Cache.StreamingURLS = make(map[string]structs.StreamInfo)

			err = updateFile(request, "m3u")
			if err == nil {
				updateUrlsJson()
				response.OpenMenu = strconv.Itoa(utilities.IndexOfString("playlist", config.System.WEB.Menu))
			}

		case "saveFilesHDHR":
			err = saveFiles(request, "hdhr")
			if err == nil {
				response.OpenMenu = strconv.Itoa(utilities.IndexOfString("playlist", config.System.WEB.Menu))
			}

		case "updateFileHDHR":
			err = updateFile(request, "hdhr")
			if err == nil {
				response.OpenMenu = strconv.Itoa(utilities.IndexOfString("playlist", config.System.WEB.Menu))
			}

		case "saveFilesXMLTV":
			err = saveFiles(request, "xmltv")
			if err == nil {
				response.OpenMenu = strconv.Itoa(utilities.IndexOfString("xmltv", config.System.WEB.Menu))
			}

		case "updateFileXMLTV":
			err = updateFile(request, "xmltv")
			if err == nil {
				response.OpenMenu = strconv.Itoa(utilities.IndexOfString("xmltv", config.System.WEB.Menu))
			}

		case "saveFilter":
			response.Settings, err = saveFilter(request)
			if err == nil {
				response.OpenMenu = strconv.Itoa(utilities.IndexOfString("filter", config.System.WEB.Menu))
			}

		case "saveEpgMapping":
			err = saveXEpgMapping(request)

		case "saveUserData":
			err = saveUserData(request)
			if err == nil {
				response.OpenMenu = strconv.Itoa(utilities.IndexOfString("users", config.System.WEB.Menu))
			}

		case "saveNewUser":
			err = saveNewUser(request)
			if err == nil {
				response.OpenMenu = strconv.Itoa(utilities.IndexOfString("users", config.System.WEB.Menu))
			}

		case "resetLogs":
			config.WebScreenLog.Log = make([]string, 0)
			config.WebScreenLog.Errors = 0
			config.WebScreenLog.Warnings = 0
			response.OpenMenu = strconv.Itoa(utilities.IndexOfString("log", config.System.WEB.Menu))

		case "ThreadfinBackup":
			file, errNew := ThreadfinBackup()
			err = errNew
			if err == nil {
				response.OpenLink = fmt.Sprintf("%s://%s/download/%s", config.System.ServerProtocol.WEB, config.System.Domain, file)
			}

		case "ThreadfinRestore":
			config.WebScreenLog.Log = make([]string, 0)
			config.WebScreenLog.Errors = 0
			config.WebScreenLog.Warnings = 0

			if len(request.Base64) > 0 {
				newWebURL, err := ThreadfinRestoreFromWeb(request.Base64)
				if err != nil {
					cli.ShowError(err, 000)
					response.Alert = err.Error()
				}

				if err == nil {
					if len(newWebURL) > 0 {
						response.Alert = "Backup was successfully restored.\nThe port of the sTeVe URL has changed, you have to restart Threadfin.\nAfter a restart, Threadfin can be reached again at the following URL:\n" + newWebURL
					} else {
						response.Alert = "Backup was successfully restored."
						response.Reload = true
					}
					cli.ShowInfo("Threadfin:" + "Backup successfully restored.")
				}
			}

		case "uploadLogo":
			if len(request.Base64) > 0 {
				response.LogoURL, err = media.UploadLogo(request.Base64, request.Filename)
				if err == nil {
					if err = conn.WriteJSON(response); err != nil {
						cli.ShowError(err, 1022)
					} else {
						return
					}
				}
			}

		case "saveWizard":
			nextStep, errNew := saveWizard(request)
			err = errNew
			if err == nil {
				if nextStep == 10 {
					config.System.ConfigurationWizard = false
					response.Reload = true
				} else {
					response.Wizard = nextStep
				}
			}

		case "probeChannel":
			resolution, frameRate, audioChannels, _ := probeChannel(request)
			response.ProbeInfo = structs.ProbeInfoStruct{Resolution: resolution, FrameRate: frameRate, AudioChannel: audioChannels}

		default:
			fmt.Println("+ + + + + + + + + + +", request.Cmd)
		}

		if err != nil {
			response.Status = false
			response.Error = err.Error()
			response.Settings = config.Settings
		}

		response = setDefaultResponseData(response, true)
		if config.System.ConfigurationWizard {
			response.ConfigurationWizard = config.System.ConfigurationWizard
		}

		if err = conn.WriteJSON(response); err != nil {
			cli.ShowError(err, 1022)
		} else {
			break
		}

	}
}

// Web : Web Server /web/
func Web(w http.ResponseWriter, r *http.Request) {

	var lang = make(map[string]any)
	var err error

	var requestFile = strings.ReplaceAll(r.URL.Path, "/web", "html")
	var content, contentType, file string

	var language structs.LanguageUI

	config.SystemMutex.Lock()
	if config.Settings.HttpThreadfinDomain != "" {
		setGlobalDomain(getBaseUrl(config.Settings.HttpThreadfinDomain, config.Settings.Port))
	} else {
		setGlobalDomain(r.Host)
	}
	config.SystemMutex.Unlock()

	config.SystemMutex.Lock()
	if config.System.Dev {
		lang, err = storage.LoadJSONFileToMap(fmt.Sprintf("html/lang/%s.json", config.Settings.Language))
		config.SystemMutex.Unlock()
		if err != nil {
			cli.ShowError(err, 000)
		}
	} else {
		config.SystemMutex.Unlock()
		var languageFile = "html/lang/en.json"

		if value, ok := web.WebUI[languageFile].(string); ok {
			content = web.GetHTMLString(value)
			lang = jsonserializer.JSONToMap(content)
		}
	}

	err = json.Unmarshal([]byte(jsonserializer.MapToJSON(lang)), &language)
	if err != nil {
		cli.ShowError(err, 000)
		return
	}

	if storage.GetFilenameFromPath(requestFile) == "html" {

		config.SystemMutex.Lock()
		if config.System.ConfigurationWizard {
			file = requestFile + "configuration.html"
			config.Settings.AuthenticationWEB = false
		} else {
			file = requestFile + "index.html"
		}

		if config.System.ScanInProgress == 1 {
			file = requestFile + "maintenance.html"
		}
		authenticationWebEnabled := config.Settings.AuthenticationWEB
		config.SystemMutex.Unlock()

		if authenticationWebEnabled {
			var username, password, confirm string
			switch r.Method {
			case "POST":
				var allUserData, _ = authentication.GetAllUserData()

				username = r.FormValue("username")
				password = r.FormValue("password")

				if len(allUserData) == 0 {
					confirm = r.FormValue("confirm")
				}

				// Erster Benutzer wird angelegt (Passwortbestätigung ist vorhanden)
				if len(confirm) > 0 {

					var token, err = createFirstUserForAuthentication(username, password)
					if err != nil {
						web.HttpStatusError(w, 429)
						return
					}
					// Redirect, damit die Daten aus dem Browser gelöscht werden.
					w = authentication.SetCookieToken(w, token)
					http.Redirect(w, r, "/web", http.StatusMovedPermanently)
					return

				}

				// Benutzername und Passwort vorhanden, wird jetzt überprüft
				if len(username) > 0 && len(password) > 0 {

					var token, err = authentication.UserAuthentication(username, password)
					if err != nil {
						file = requestFile + "login.html"
						lang["authenticationErr"] = language.Login.Failed
						break
					}

					w = authentication.SetCookieToken(w, token)
					http.Redirect(w, r, "/web", http.StatusMovedPermanently) // Redirect, damit die Daten aus dem Browser gelöscht werden.

				} else {
					w = authentication.SetCookieToken(w, "-")
					http.Redirect(w, r, "/web", http.StatusMovedPermanently) // Redirect, damit die Daten aus dem Browser gelöscht werden.
				}

				return

			case "GET":
				lang["authenticationErr"] = ""
				_, token, err := authentication.CheckTheValidityOfTheTokenFromHTTPHeader(w, r)

				if err != nil {
					file = requestFile + "login.html"
					break
				}

				err = checkAuthorizationLevel(token, "authentication.web")
				if err != nil {
					file = requestFile + "login.html"
					break
				}

			}

			allUserData, err := authentication.GetAllUserData()
			if err != nil {
				cli.ShowError(err, 000)
				web.HttpStatusError(w, 403)
				return
			}

			config.SystemMutex.Lock()
			if len(allUserData) == 0 && config.Settings.AuthenticationWEB {
				file = requestFile + "create-first-user.html"
			}
			config.SystemMutex.Unlock()
		}

		requestFile = file

		if _, ok := web.WebUI[requestFile]; ok {
			if contentType == "text/plain" {
				w.Header().Set("Content-Disposition", "attachment; filename="+storage.GetFilenameFromPath(requestFile))
			}

		} else {
			web.HttpStatusError(w, 404)
			return
		}

	}

	if value, ok := web.WebUI[requestFile].(string); ok {

		content = web.GetHTMLString(value)
		contentType = getContentType(requestFile)

		if contentType == "text/plain" {
			w.Header().Set("Content-Disposition", "attachment; filename="+storage.GetFilenameFromPath(requestFile))
		}

	} else {
		web.HttpStatusError(w, 404)
		return
	}

	contentType = getContentType(requestFile)

	config.SystemMutex.Lock()
	if config.System.Dev {
		// Lokale Webserver Dateien werden geladen, nur für die Entwicklung
		content, _ = storage.ReadStringFromFile(requestFile)
	}
	config.SystemMutex.Unlock()

	w.Header().Add("Content-Type", contentType)
	w.WriteHeader(200)

	if contentType == "text/html" || contentType == "application/javascript" {
		content = parseTemplate(content, lang)
	}

	_, err = w.Write([]byte(content))
	if err != nil {
		cli.ShowError(err, 000)
	}
}

// API : API request /api/
func API(w http.ResponseWriter, r *http.Request) {

	/*
			API Bedingungen (ohne Authentifizierung):
			- API muss in den Einstellungen aktiviert sein

			Beispiel API Request mit curl
			Status:
			curl -X POST -H "Content-Type: application/json" -d '{"cmd":"status"}' http://localhost:34400/api/

			- - - - -

			API Bedingungen (mit Authentifizierung):
			- API muss in den Einstellungen aktiviert sein
			- API muss bei den Authentifizierungseinstellungen aktiviert sein
			- Benutzer muss die Berechtigung API haben

			Nach jeder API Anfrage wird ein Token generiert, dieser ist einmal in 60 Minuten gültig.
			In jeder Antwort ist ein neuer Token enthalten

			Beispiel API Request mit curl
			Login:
			curl -X POST -H "Content-Type: application/json" -d '{"cmd":"login","username":"plex","password":"123"}' http://localhost:34400/api/

			Antwort:
			{
		  	"status": true,
		  	"token": "U0T-NTSaigh-RlbkqERsHvUpgvaaY2dyRGuwIIvv"
			}

			Status mit Verwendung eines Tokens:
			curl -X POST -H "Content-Type: application/json" -d '{"cmd":"status","token":"U0T-NTSaigh-RlbkqERsHvUpgvaaY2dyRGuwIIvv"}' http://localhost:4400/api/

			Antwort:
			{
			  "epg.source": "XEPG",
			  "status": true,
			  "streams.active": 7,
			  "streams.all": 63,
			  "streams.xepg": 2,
			  "token": "mXiG1NE1MrTXDtyh7PxRHK5z8iPI_LzxsQmY-LFn",
			  "url.dvr": "localhost:34400",
			  "url.m3u": "http://localhost:34400/m3u/threadfin.m3u",
			  "url.xepg": "http://localhost:34400/xmltv/threadfin.xml",
			  "version.api": "1.1.0",
			  "version.threadfin": "1.3.0"
			}
	*/

	if config.Settings.HttpThreadfinDomain != "" {
		setGlobalDomain(getBaseUrl(config.Settings.HttpThreadfinDomain, config.Settings.Port))
	} else {
		setGlobalDomain(r.Host)
	}
	var request structs.APIRequestStruct
	var response structs.APIResponseStruct

	var responseAPIError = func(err error) {

		var response structs.APIResponseStruct

		response.Status = false
		response.Error = err.Error()
		_, err = w.Write([]byte(jsonserializer.MapToJSON(response)))
		if err != nil {
			cli.ShowError(err, 000)
		}
	}

	response.Status = true

	if config.Settings.API {
		web.HttpStatusError(w, 423)
		return
	}

	if r.Method == "GET" {
		web.HttpStatusError(w, 404)
		return
	}

	b, err := io.ReadAll(r.Body)
	if err != nil {
		web.HttpStatusError(w, 400)
		return
	}

	defer func() {
		err = r.Body.Close()
	}()
	if err != nil {
		web.HttpStatusError(w, 400)
		return
	}

	err = json.Unmarshal(b, &request)
	if err != nil {
		web.HttpStatusError(w, 400)
		return
	}

	w.Header().Set("content-type", "application/json")

	if config.Settings.AuthenticationAPI {
		var token string
		switch len(request.Token) {
		case 0:
			if request.Cmd == "login" {
				token, err = authentication.UserAuthentication(request.Username, request.Password)
				if err != nil {
					responseAPIError(err)
					return
				}

			} else {
				err = errors.New("login incorrect")
				if err != nil {
					responseAPIError(err)
					return
				}

			}

		default:
			token, err = tokenAuthentication(request.Token)
			fmt.Println(err)
			if err != nil {
				responseAPIError(err)
				return
			}

		}
		err = checkAuthorizationLevel(token, "authentication.api")
		if err != nil {
			responseAPIError(err)
			return
		}

		response.Token = token

	}

	switch request.Cmd {
	case "login": // Muss nichts übergeben werden

	case "status":

		response.VersionThreadfin = config.System.Version
		response.VersionAPI = config.System.APIVersion
		response.StreamsActive = int64(len(config.Data.Streams.Active))
		response.StreamsAll = int64(len(config.Data.Streams.All))
		response.StreamsXepg = int64(config.Data.XEPG.XEPGCount)
		response.EpgSource = config.Settings.EpgSource
		response.URLDvr = config.System.Domain
		response.URLM3U = config.System.ServerProtocol.M3U + "://" + config.System.Domain + "/m3u/threadfin.m3u"
		response.URLXepg = config.System.ServerProtocol.XML + "://" + config.System.Domain + "/xmltv/threadfin.xml"

	case "update.m3u":
		err = getProviderData("m3u", "")
		if err != nil {
			break
		}

		err = buildDatabaseDVR()
		if err != nil {
			break
		}

		buildXEPG(false)

	case "update.hdhr":

		err = getProviderData("hdhr", "")
		if err != nil {
			break
		}

		err = buildDatabaseDVR()
		if err != nil {
			break
		}

		buildXEPG(false)

	case "update.xmltv":
		err = getProviderData("xmltv", "")
		if err != nil {
			break
		}

		buildXEPG(false)

	case "update.xepg":
		buildXEPG(false)

	default:
		err = errors.New(cli.GetErrMsg(5000))

	}

	if err != nil {
		responseAPIError(err)
	}

	_, err = w.Write([]byte(jsonserializer.MapToJSON(response)))
	if err != nil {
		cli.ShowError(err, 000)
	}
}

// Download : Datei Download
func Download(w http.ResponseWriter, r *http.Request) {

	var path = r.URL.Path
	var file = config.System.Folder.Temp + storage.GetFilenameFromPath(path)
	w.Header().Set("Content-Disposition", "attachment; filename="+storage.GetFilenameFromPath(file))

	content, err := storage.ReadStringFromFile(file)
	if err != nil {
		w.WriteHeader(404)
		return
	}

	err = os.RemoveAll(config.System.Folder.Temp + storage.GetFilenameFromPath(path))
	if err != nil {
		cli.ShowError(err, 000)
	}

	_, err = w.Write([]byte(content))
	if err != nil {
		cli.ShowError(err, 000)
	}
}

func setDefaultResponseData(response structs.ResponseStruct, data bool) (defaults structs.ResponseStruct) {

	defaults = response

	// Total connections for all playlists
	totalPlaylistCount := 0
	if len(config.Settings.Files.M3U) > 0 {
		for _, value := range config.Settings.Files.M3U {

			// Assert that value is a map[string]interface{}
			nestedMap, ok := value.(map[string]interface{})
			if !ok {
				fmt.Printf("Error asserting nested value as map: %v\n", value)
				continue
			}

			// Get the tuner count
			if tuner, exists := nestedMap["tuner"]; exists {
				switch v := tuner.(type) {
				case float64:
					totalPlaylistCount += int(v)
				case int:
					totalPlaylistCount += v
				default:
				}
			}
		}
	}

	// Folgende Daten immer an den Client übergeben
	defaults.ClientInfo.ARCH = config.System.ARCH
	defaults.ClientInfo.EpgSource = config.Settings.EpgSource
	defaults.ClientInfo.DVR = config.System.Addresses.DVR
	defaults.ClientInfo.M3U = config.System.Addresses.M3U
	defaults.ClientInfo.XML = config.System.Addresses.XML
	defaults.ClientInfo.OS = config.System.OS
	defaults.ClientInfo.Streams = fmt.Sprintf("%d / %d", len(config.Data.Streams.Active), len(config.Data.Streams.All))
	defaults.ClientInfo.UUID = config.Settings.UUID
	defaults.ClientInfo.Errors = config.WebScreenLog.Errors
	defaults.ClientInfo.Warnings = config.WebScreenLog.Warnings
	defaults.ClientInfo.ActiveClients = client.GetActiveCount()
	defaults.ClientInfo.ActivePlaylist = playlist.GetActiveCount()
	defaults.ClientInfo.TotalClients = config.Settings.Tuner
	defaults.ClientInfo.TotalPlaylist = totalPlaylistCount
	defaults.Notification = config.System.Notification
	defaults.Log = config.WebScreenLog

	switch config.System.Branch {

	case "master":
		defaults.ClientInfo.Version = config.System.Version

	default:
		defaults.ClientInfo.Version = fmt.Sprintf("%s (%s)", config.System.Version, config.System.Build)
		defaults.ClientInfo.Branch = config.System.Branch

	}

	if data {
		defaults.Users, _ = authentication.GetAllUserData()

		if config.Settings.EpgSource == "XEPG" {

			defaults.ClientInfo.XEPGCount = config.Data.XEPG.XEPGCount

			var XEPG = make(map[string]interface{})

			if len(config.Data.Streams.Active) > 0 {

				XEPG["epgMapping"] = config.Data.XEPG.Channels
				XEPG["xmltvMap"] = config.Data.XMLTV.Mapping

			} else {

				XEPG["epgMapping"] = make(map[string]interface{})
				XEPG["xmltvMap"] = make(map[string]interface{})

			}

			defaults.XEPG = XEPG

		}

		defaults.Settings = config.Settings

		defaults.Data.Playlist.M3U.Groups.Text = config.Data.Playlist.M3U.Groups.Text
		defaults.Data.Playlist.M3U.Groups.Value = config.Data.Playlist.M3U.Groups.Value
		defaults.Data.StreamPreviewUI.Active = config.Data.StreamPreviewUI.Active
		defaults.Data.StreamPreviewUI.Inactive = config.Data.StreamPreviewUI.Inactive

	}

	return
}

func enablePPV(w http.ResponseWriter, r *http.Request) {
	xepg, err := storage.LoadJSONFileToMap(config.System.File.XEPG)
	if err != nil {
		var response structs.APIResponseStruct

		response.Status = false
		response.Error = err.Error()
		_, err = w.Write([]byte(jsonserializer.MapToJSON(response)))
		if err != nil {
			cli.ShowError(err, 000)
		}
		return
	}

	for _, c := range xepg {

		var xepgChannel = c.(map[string]interface{})

		if xepgChannel["x-mapping"] == "PPV" {
			xepgChannel["x-active"] = true
		}
	}

	err = storage.SaveMapToJSONFile(config.System.File.XEPG, xepg)
	if err != nil {
		var response structs.APIResponseStruct

		response.Status = false
		response.Error = err.Error()
		_, err = w.Write([]byte(jsonserializer.MapToJSON(response)))
		if err != nil {
			cli.ShowError(err, 000)
		}
		w.WriteHeader(405)
		return
	}
	buildXEPG(false)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
}

func disablePPV(w http.ResponseWriter, r *http.Request) {
	xepg, err := storage.LoadJSONFileToMap(config.System.File.XEPG)
	if err != nil {
		var response structs.APIResponseStruct

		response.Status = false
		response.Error = err.Error()
		_, err = w.Write([]byte(jsonserializer.MapToJSON(response)))
		if err != nil {
			cli.ShowError(err, 000)
		}
	}

	for _, c := range xepg {

		var xepgChannel = c.(map[string]interface{})

		if xepgChannel["x-mapping"] == "PPV" && xepgChannel["x-active"] == true {
			xepgChannel["x-active"] = false
		}
	}

	err = storage.SaveMapToJSONFile(config.System.File.XEPG, xepg)
	if err != nil {
		var response structs.APIResponseStruct

		response.Status = false
		response.Error = err.Error()
		_, err = w.Write([]byte(jsonserializer.MapToJSON(response)))
		if err != nil {
			cli.ShowError(err, 000)
		}
	}
	buildXEPG(false)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
}

func getContentType(filename string) (contentType string) {

	mimeTypes := map[string]string{
		".html": "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".svg":  "image/svg+xml",
		".ico":  "image/x-icon",
		".webp": "image/webp",
		".mp4":  "video/mp4",
		".webm": "video/webm",
		".ogg":  "video/ogg",
		".mp3":  "audio/mp3",
		".wav":  "audio/wav",
	}

	// Extract the file extension and normalize it to lowercase
	ext := strings.ToLower(path.Ext(filename))
	if contentType, exists := mimeTypes[ext]; exists {
		return contentType
	}
	return "text/plain"
}
