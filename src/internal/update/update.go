package update

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"reflect"
	"threadfin/internal/cli"
	"threadfin/internal/config"
	"threadfin/internal/storage"
	"threadfin/internal/structs"
	up2date "threadfin/internal/up2date/client"

	"github.com/hashicorp/go-version"
)

// BinaryUpdate : Binary Update Prozess. Git Branch master und beta wird von GitHub geladen.
func BinaryUpdate() (err error) {

	if !config.System.GitHub.Update {
		cli.ShowWarning(2099)
		return
	}

	if !config.Settings.ThreadfinAutoUpdate {
		cli.ShowWarning(2098)
		return
	}

	var debug string

	var updater = &up2date.Updater
	updater.Name = config.System.Update.Name
	updater.Branch = config.System.Branch

	up2date.Init()

	log.Println("BRANCH: ", config.System.Branch)
	switch config.System.Branch {

	// Update von GitHub
	case "Main", "Beta":
		var releaseInfo = fmt.Sprintf("%s/releases", config.System.Update.Github)
		var latest string
		var body []byte

		var git []*structs.GithubReleaseInfo

		resp, err := http.Get(releaseInfo)
		if err != nil {
			cli.ShowError(err, 6003)
			return nil
		}

		body, _ = io.ReadAll(resp.Body)

		err = json.Unmarshal(body, &git)
		if err != nil {
			return err
		}

		// Get latest prerelease tag name
		if config.System.Branch == "Beta" {
			for _, release := range git {
				if release.Prerelease {
					latest = release.TagName
					updater.Response.Version = release.TagName
					break
				}
			}
		}

		// Latest main tag name
		if config.System.Branch == "Main" {
			for _, release := range git {
				if !release.Prerelease {
					updater.Response.Version = release.TagName
					latest = release.TagName
					log.Println("TAG LATEST: ", release.TagName)
					break
				}
			}
		}

		var File = fmt.Sprintf("%s/releases/download/%s/%s_%s_%s", config.System.Update.Git, latest, "Threadfin", config.System.OS, config.System.ARCH)

		updater.Response.Status = true
		updater.Response.UpdateBIN = File

		log.Println("FILE: ", updater.Response.UpdateBIN)

	// Update vom eigenen Server
	default:

		updater.URL = config.Settings.UpdateURL

		if len(updater.URL) == 0 {
			cli.ShowInfo(fmt.Sprintf("Update URL:No server URL specified, update will not be performed. Branch: %s", config.System.Branch))
			return
		}

		cli.ShowInfo("Update URL:" + updater.URL)
		fmt.Println("-----------------")

		// Versionsinformationen vom Server laden
		err = up2date.GetVersion()
		if err != nil {

			debug = err.Error()
			cli.ShowDebug(debug, 1)

			return nil
		}

		if len(updater.Response.Reason) > 0 {

			err = fmt.Errorf("update Server: %s", updater.Response.Reason)
			cli.ShowError(err, 6002)

			return nil
		}

	}

	var currentVersion = config.System.Version + "." + config.System.Build
	current_version, _ := version.NewVersion(currentVersion)
	response_version, _ := version.NewVersion(updater.Response.Version)
	// Versionsnummer überprüfen
	if response_version.GreaterThan(current_version) && updater.Response.Status {
		if config.Settings.ThreadfinAutoUpdate {
			// Update durchführen
			var fileType, url string

			cli.ShowInfo(fmt.Sprintf("Update Available:Version: %s", updater.Response.Version))

			switch config.System.Branch {

			// Update von GitHub
			case "master", "beta":
				cli.ShowInfo("Update Server:GitHub")

			// Update vom eigenen Server
			default:
				cli.ShowInfo(fmt.Sprintf("Update Server:%s", config.Settings.UpdateURL))

			}

			cli.ShowInfo(fmt.Sprintf("Start Update:Branch: %s", updater.Branch))

			// Neue Version als BIN Datei herunterladen
			if len(updater.Response.UpdateBIN) > 0 {
				url = updater.Response.UpdateBIN
				fileType = "bin"
			}

			// Neue Version als ZIP Datei herunterladen
			if len(updater.Response.UpdateZIP) > 0 {
				url = updater.Response.UpdateZIP
				fileType = "zip"
			}

			if len(url) > 0 {

				err = up2date.DoUpdate(fileType, updater.Response.Filename)
				if err != nil {
					cli.ShowError(err, 6002)
				}

			}

		} else {
			// Hinweis ausgeben
			cli.ShowWarning(6004)
		}

	}

	return nil
}

func ConditionalUpdateChanges() (err error) {

checkVersion:
	settingsMap, err := storage.LoadJSONFileToMap(config.System.File.Settings)
	if err != nil || len(settingsMap) == 0 {
		return
	}

	if settingsVersion, ok := settingsMap["version"].(string); ok {

		if settingsVersion > config.System.DBVersion {
			cli.ShowInfo("Settings DB Version:" + settingsVersion)
			cli.ShowInfo("System DB Version:" + config.System.DBVersion)
			err = errors.New(cli.GetErrMsg(1031))
			return
		}

		// Letzte Kompatible Version (1.4.4)
		if settingsVersion < config.System.Compatibility {
			err = errors.New(cli.GetErrMsg(1013))
			return
		}

		switch settingsVersion {

		case "1.4.4":
			// UUID Wert in xepg.json setzen
			err = setValueForUUID()
			if err != nil {
				return
			}

			// Neuer Filter (WebUI). Alte Filtereinstellungen werden konvertiert
			if oldFilter, ok := settingsMap["filter"].([]interface{}); ok {
				var newFilterMap = convertToNewFilter(oldFilter)
				settingsMap["filter"] = newFilterMap

				settingsMap["version"] = "2.0.0"

				err = storage.SaveMapToJSONFile(config.System.File.Settings, settingsMap)
				if err != nil {
					return
				}

				goto checkVersion

			} else {
				err = errors.New(cli.GetErrMsg(1030))
				return
			}

		case "2.0.0":

			if oldBuffer, ok := settingsMap["buffer"].(bool); ok {

				var newBuffer string
				switch oldBuffer {
				case true:
					newBuffer = "threadfin"
				case false:
					newBuffer = "-"
				}

				settingsMap["buffer"] = newBuffer

				settingsMap["version"] = "2.1.0"

				err = storage.SaveMapToJSONFile(config.System.File.Settings, settingsMap)
				if err != nil {
					return
				}

				goto checkVersion

			} else {
				err = errors.New(cli.GetErrMsg(1030))
				return
			}

		case "2.1.0":
			// Falls es in einem späteren Update Änderungen an der Datenbank gibt, geht es hier weiter

			break
		}

	} else {
		// settings.json ist zu alt (älter als Version 1.4.4)
		err = errors.New(cli.GetErrMsg(1013))
	}

	return
}

func convertToNewFilter(oldFilter []interface{}) (newFilterMap map[int]interface{}) {

	newFilterMap = make(map[int]interface{})

	switch reflect.TypeOf(oldFilter).Kind() {

	case reflect.Slice:
		s := reflect.ValueOf(oldFilter)

		for i := 0; i < s.Len(); i++ {

			var newFilter structs.FilterStruct
			newFilter.Active = true
			newFilter.Name = fmt.Sprintf("Custom filter %d", i+1)
			newFilter.Filter = s.Index(i).Interface().(string)
			newFilter.Type = "custom-filter"
			newFilter.CaseSensitive = false

			newFilterMap[i] = newFilter

		}

	}

	return
}

func setValueForUUID() (err error) {

	xepg, _ := storage.LoadJSONFileToMap(config.System.File.XEPG)

	for _, c := range xepg {

		var xepgChannel = c.(map[string]interface{})

		if uuidKey, ok := xepgChannel["_uuid.key"].(string); ok {

			if value, ok := xepgChannel[uuidKey].(string); ok {

				if len(value) > 0 {
					xepgChannel["_uuid.value"] = value
				}

			}

		}

	}

	err = storage.SaveMapToJSONFile(config.System.File.XEPG, xepg)

	return
}
