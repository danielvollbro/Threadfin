package m3u

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/config"
	"threadfin/src/internal/hdhr"
	jsonserializer "threadfin/src/internal/json-serializer"
	"threadfin/src/internal/m3u-parser"
	"threadfin/src/internal/provider"
	"threadfin/src/internal/storage"
	"threadfin/src/internal/stream"
	"threadfin/src/internal/structs"
	"threadfin/src/internal/utilities"
)

func Build(groups []string) (m3u string, err error) {

	var imgc = config.Data.Cache.Images
	// Preserve every active channel as a distinct entry by iteration order, not keyed by number
	type channelEntry struct {
		idx int
		ch  structs.XEPGChannelStruct
	}
	var entries []channelEntry

	// Build a map of group -> expectedCount from Data.Playlist.M3U.Groups.Text (format: "Group (N)")
	expectedGroupCount := make(map[string]int)
	for _, label := range config.Data.Playlist.M3U.Groups.Text {
		// label example: "nfl (42)"
		open := strings.LastIndex(label, " (")
		close := strings.LastIndex(label, ")")
		if open > 0 && close > open+2 {
			name := label[:open]
			countStr := label[open+2 : close]
			if n, e := strconv.Atoi(countStr); e == nil {
				expectedGroupCount[name] = n
			}
		}
	}

	// Count deactivated channels per group
	deactivatedPerGroup := make(map[string]int)
	for _, dxc := range config.Data.XEPG.Channels {
		var ch structs.XEPGChannelStruct
		if err := json.Unmarshal([]byte(jsonserializer.MapToJSON(dxc)), &ch); err == nil {
			group := ch.XGroupTitle
			if ch.XCategory != "" {
				group = ch.XCategory
			}
			if group == "" {
				group = ch.GroupTitle
			}
			if !ch.XActive || ch.XHideChannel {
				deactivatedPerGroup[group] = deactivatedPerGroup[group] + 1
			}
		}
	}

	for _, dxc := range config.Data.XEPG.Channels {
		var xepgChannel structs.XEPGChannelStruct
		err := json.Unmarshal([]byte(jsonserializer.MapToJSON(dxc)), &xepgChannel)
		if err == nil {
			if xepgChannel.TvgName == "" {
				xepgChannel.TvgName = xepgChannel.Name
			}
			if xepgChannel.XActive && !xepgChannel.XHideChannel {
				if len(groups) > 0 {

					if utilities.IndexOfString(xepgChannel.XGroupTitle, groups) == -1 {
						goto Done
					}

				}
				entries = append(entries, channelEntry{idx: len(entries), ch: xepgChannel})
			}
		}

	Done:
	}

	// Prepare header
	var xmltvURL = fmt.Sprintf("%s://%s/xmltv/threadfin.xml", config.System.ServerProtocol.XML, config.System.Domain)
	if config.Settings.ForceHttps && config.Settings.HttpsThreadfinDomain != "" {
		xmltvURL = fmt.Sprintf("https://%s/xmltv/threadfin.xml", config.Settings.HttpsThreadfinDomain)
	}
	header := fmt.Sprintf(`#EXTM3U url-tvg="%s" x-tvg-url="%s"`+"\n", xmltvURL, xmltvURL)

	// If generating the full file, stream to disk to avoid huge in-memory strings
	var writer *bufio.Writer
	var file *os.File
	if len(groups) == 0 {
		filename := config.System.Folder.Data + "threadfin.m3u"
		file, err = os.Create(filename)
		if err != nil {
			return "", err
		}

		defer func() {
			err = file.Close()
		}()
		if err != nil {
			return "", err
		}

		writer = bufio.NewWriterSize(file, 1<<20) // 1MB buffer
		if _, err = writer.WriteString(header); err != nil {
			return "", err
		}
	} else {
		m3u = header
	}

	// Sort entries by tvg-chno numerically
	sort.Slice(entries, func(i, j int) bool {
		chI := entries[i].ch.TvgChno
		chJ := entries[j].ch.TvgChno

		// Try to parse as numbers for proper numeric sorting
		numI, errI := strconv.ParseFloat(chI, 64)
		numJ, errJ := strconv.ParseFloat(chJ, 64)

		// If both are numbers, sort numerically
		if errI == nil && errJ == nil {
			return numI < numJ
		}

		// If one is a number and other isn't, number comes first
		if errI == nil && errJ != nil {
			return true
		}
		if errI != nil && errJ == nil {
			return false
		}

		// If both are strings, sort alphabetically
		return chI < chJ
	})

	// Avoid duplicate exact stream URLs within the same group and cap per-group by expected minus deactivated
	seenURLInGroup := make(map[string]struct{})
	emittedGroupCount := make(map[string]int)
	for _, e := range entries {
		var channel = e.ch

		group := channel.XGroupTitle
		if channel.XCategory != "" {
			group = channel.XCategory
		}
		if group == "" {
			group = channel.GroupTitle
		}

		// Determine allowed active count = expected - deactivated
		if expected, ok := expectedGroupCount[group]; ok {
			allowed := expected - deactivatedPerGroup[group]
			if allowed < 0 {
				allowed = 0
			}
			if emittedGroupCount[group] >= allowed {
				continue
			}
		}

		// Disabling so not to rewrite stream to https domain when disable stream from https set
		if config.Settings.ForceHttps && config.Settings.HttpsThreadfinDomain != "" && !config.Settings.ExcludeStreamHttps {
			u, err := url.Parse(channel.URL)
			if err == nil {
				u.Scheme = "https"
				host_split := strings.Split(u.Host, ":")
				if len(host_split) > 0 {
					u.Host = host_split[0]
				}
				if u.RawQuery != "" {
					channel.URL = fmt.Sprintf("https://%s:%d%s?%s", u.Host, config.Settings.HttpsPort, u.Path, u.RawQuery)
				} else {
					channel.URL = fmt.Sprintf("https://%s:%d%s", u.Host, config.Settings.HttpsPort, u.Path)
				}
			}
		}

		logo := ""
		if channel.TvgLogo != "" {
			logo = imgc.Image.GetURL(channel.TvgLogo, config.Settings.HttpThreadfinDomain, config.Settings.Port, config.Settings.ForceHttps, config.Settings.HttpsPort, config.Settings.HttpsThreadfinDomain)
		}
		var parameter = fmt.Sprintf(`#EXTINF:0 channelID="%s" tvg-chno="%s" tvg-name="%s" tvg-id="%s" tvg-logo="%s" group-title="%s",%s`+"\n", channel.XEPG, channel.XChannelID, channel.XName, channel.XChannelID, logo, group, channel.XName)
		var stream, err = stream.CreateURL("M3U", channel.FileM3UID, channel.XChannelID, channel.XName, channel.URL, channel.BackupChannel1, channel.BackupChannel2, channel.BackupChannel3)
		if err == nil {
			key := group + "|" + stream
			if _, ok := seenURLInGroup[key]; ok {
				continue
			}
			seenURLInGroup[key] = struct{}{}
			if writer != nil {
				if _, err = writer.WriteString(parameter); err != nil {
					return "", err
				}
				if _, err = writer.WriteString(stream + "\n"); err != nil {
					return "", err
				}
			} else {
				m3u = m3u + parameter + stream + "\n"
			}
			emittedGroupCount[group] = emittedGroupCount[group] + 1
		}

	}

	if writer != nil {
		if err = writer.Flush(); err != nil {
			return "", err
		}
	}

	return
}

func CreateFile() {
	cli.ShowInfo("XEPG:" + fmt.Sprintf("Create M3U file (%s)", config.System.File.M3U))
	_, err := Build([]string{})
	if err != nil {
		cli.ShowError(err, 000)
	}

	err = storage.SaveMapToJSONFile(config.System.File.URLS, config.Data.Cache.StreamingURLS)
	if err != nil {
		cli.ShowError(err, 000)
	}
}

// Playlisten parsen
func ParsePlaylist(filename, fileType string) (channels []interface{}, err error) {

	content, err := storage.ReadByteFromFile(filename)
	var id = strings.TrimSuffix(storage.GetFilenameFromPath(filename), path.Ext(storage.GetFilenameFromPath(filename)))
	var playlistName = provider.GetProviderParameter(id, fileType, "name")

	if err == nil {

		switch fileType {
		case "m3u":
			channels, err = m3u.MakeInterfaceFromM3U(content)
		case "hdhr":
			channels, err = hdhr.MakeInteraceFromHDHR(content, playlistName, id)
		}

	}

	return
}
