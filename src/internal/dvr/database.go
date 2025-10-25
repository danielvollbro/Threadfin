package dvr

import (
	"errors"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/config"
	"threadfin/src/internal/m3u"
	"threadfin/src/internal/provider"
	"threadfin/src/internal/storage"
)

// Datenbank für das DVR System erstellen
func BuildDatabase() (err error) {

	config.System.ScanInProgress = 1

	config.Data.Streams.All = make([]interface{}, 0, config.System.UnfilteredChannelLimit)
	config.Data.Streams.Active = make([]interface{}, 0, config.System.UnfilteredChannelLimit)
	config.Data.Streams.Inactive = make([]interface{}, 0, config.System.UnfilteredChannelLimit)
	config.Data.Playlist.M3U.Groups.Text = []string{}
	config.Data.Playlist.M3U.Groups.Value = []string{}
	config.Data.StreamPreviewUI.Active = []string{}
	config.Data.StreamPreviewUI.Inactive = []string{}

	var availableFileTypes = []string{"m3u", "hdhr"}

	var tmpGroupsM3U = make(map[string]int64)

	err = createFilterRules()
	if err != nil {
		return
	}

	for _, fileType := range availableFileTypes {

		var playlistFile = provider.GetLocalFiles(fileType)

		for n, i := range playlistFile {

			var channels []interface{}
			var groupTitle, tvgID, uuid = 0, 0, 0
			var keys = []string{"group-title", "tvg-id", "uuid"}
			var compatibility = make(map[string]int)

			var id = strings.TrimSuffix(storage.GetFilenameFromPath(i), path.Ext(storage.GetFilenameFromPath(i)))
			var playlistName = provider.GetProviderParameter(id, fileType, "name")

			switch fileType {

			case "m3u":
				channels, err = m3u.ParsePlaylist(i, fileType)
			case "hdhr":
				channels, err = m3u.ParsePlaylist(i, fileType)

			}

			if err != nil {
				cli.ShowError(err, 1005)
				err = errors.New(playlistName + ": Local copy of the file no longer exists")
				cli.ShowError(err, 0)
				playlistFile = append(playlistFile[:n], playlistFile[n+1:]...)
			}

			// Streams analysieren
			for _, stream := range channels {
				var s = stream.(map[string]string)
				s["_file.m3u.path"] = i
				s["_file.m3u.name"] = playlistName
				s["_file.m3u.id"] = id

				// Kompatibilität berechnen
				for _, key := range keys {

					switch key {
					case "uuid":
						if value, ok := s["_uuid.key"]; ok {
							if len(value) > 0 {
								uuid++
							}
						}

					case "group-title":
						if value, ok := s[key]; ok {
							if len(value) > 0 {
								tmpGroupsM3U[value]++
								groupTitle++
							}
						}

					case "tvg-id":
						if value, ok := s[key]; ok {
							if len(value) > 0 {
								tvgID++
							}
						}

					}

				}

				config.Data.Streams.All = append(config.Data.Streams.All, stream)

				// Neuer Filter ab Version 1.3.0
				var preview string
				var status bool

				if config.Settings.IgnoreFilters {
					status = true
				} else {
					var liveEvent bool
					status, liveEvent = m3u.FilterThisStream(stream)
					s["liveEvent"] = strconv.FormatBool(liveEvent)
				}

				if name, ok := s["name"]; ok {
					var group string

					if v, ok := s["group-title"]; ok {
						group = v
					}

					preview = fmt.Sprintf("%s [%s]", name, group)

				}

				switch status {

				case true:
					config.Data.StreamPreviewUI.Active = append(config.Data.StreamPreviewUI.Active, preview)
					config.Data.Streams.Active = append(config.Data.Streams.Active, stream)

				case false:
					config.Data.StreamPreviewUI.Inactive = append(config.Data.StreamPreviewUI.Inactive, preview)
					config.Data.Streams.Inactive = append(config.Data.Streams.Inactive, stream)

				}

			}

			if tvgID == 0 {
				compatibility["tvg.id"] = 0
			} else {
				compatibility["tvg.id"] = int(tvgID * 100 / len(channels))
			}

			if groupTitle == 0 {
				compatibility["group.title"] = 0
			} else {
				compatibility["group.title"] = int(groupTitle * 100 / len(channels))
			}

			if uuid == 0 {
				compatibility["stream.id"] = 0
			} else {
				compatibility["stream.id"] = int(uuid * 100 / len(channels))
			}

			compatibility["streams"] = len(channels)

			provider.SetCompatibility(id, fileType, compatibility)

		}

	}

	for group, count := range tmpGroupsM3U {
		var text = fmt.Sprintf("%s (%d)", group, count)
		config.Data.Playlist.M3U.Groups.Text = append(config.Data.Playlist.M3U.Groups.Text, text)
		config.Data.Playlist.M3U.Groups.Value = append(config.Data.Playlist.M3U.Groups.Value, group)
	}

	sort.Strings(config.Data.Playlist.M3U.Groups.Text)
	sort.Strings(config.Data.Playlist.M3U.Groups.Value)

	if len(config.Data.Streams.Active) == 0 && len(config.Data.Streams.All) <= config.System.UnfilteredChannelLimit && len(config.Settings.Filter) == 0 {
		config.Data.Streams.Active = config.Data.Streams.All
		config.Data.Streams.Inactive = make([]interface{}, 0)

		config.Data.StreamPreviewUI.Active = config.Data.StreamPreviewUI.Inactive
		config.Data.StreamPreviewUI.Inactive = []string{}

	}

	if len(config.Data.Streams.Active) > config.System.PlexChannelLimit {
		cli.ShowWarning(2000)
	}

	if len(config.Settings.Filter) == 0 && len(config.Data.Streams.All) > config.System.UnfilteredChannelLimit {
		cli.ShowWarning(2001)
	}

	config.System.ScanInProgress = 0
	cli.ShowInfo(fmt.Sprintf("All streams:%d", len(config.Data.Streams.All)))
	cli.ShowInfo(fmt.Sprintf("Active streams:%d", len(config.Data.Streams.Active)))
	cli.ShowInfo(fmt.Sprintf("Filter:%d", len(config.Data.Filter)))

	sort.Strings(config.Data.StreamPreviewUI.Active)
	sort.Strings(config.Data.StreamPreviewUI.Inactive)

	return
}
