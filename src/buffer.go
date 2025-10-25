package src

/*
  Render tuner-limit image as video [ffmpeg]
  -loop 1 -i stream-limit.jpg -c:v libx264 -t 1 -pix_fmt yuv420p -vf scale=1920:1080  stream-limit.ts
*/

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/config"
	"threadfin/src/internal/storage"
	"threadfin/src/internal/structs"
	"threadfin/src/web"
	"time"

	"github.com/avfs/avfs/vfs/memfs"
)

func getActiveClientCount() (count int) {
	count = 0
	cleanUpStaleClients() // Ensure stale clients are removed first

	config.BufferInformation.Range(func(key, value interface{}) bool {
		playlist, ok := value.(structs.Playlist)
		if !ok {
			fmt.Printf("Invalid type assertion for playlist: %v\n", value)
			return true
		}

		for clientID, client := range playlist.Clients {
			if client.Connection < 0 {
				fmt.Printf("Client ID %d has negative connections: %d. Resetting to 0.\n", clientID, client.Connection)
				client.Connection = 0
				playlist.Clients[clientID] = client
				config.BufferInformation.Store(key, playlist)
			}
			if client.Connection > 1 {
				fmt.Printf("Client ID %d has suspiciously high connections: %d. Resetting to 1.\n", clientID, client.Connection)
				client.Connection = 1
				playlist.Clients[clientID] = client
				config.BufferInformation.Store(key, playlist)
			}
			count += client.Connection
		}

		fmt.Printf("Playlist %s has %d active clients\n", playlist.PlaylistID, len(playlist.Clients))
		return true
	})

	return count
}

func getActivePlaylistCount() (count int) {
	count = 0
	config.BufferInformation.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

func cleanUpStaleClients() {
	config.BufferInformation.Range(func(key, value interface{}) bool {
		playlist, ok := value.(structs.Playlist)
		if !ok {
			fmt.Printf("Invalid type assertion for playlist: %v\n", value)
			return true
		}

		for clientID, client := range playlist.Clients {
			if client.Connection <= 0 {
				fmt.Printf("Removing stale client ID %d from playlist %s\n", clientID, playlist.PlaylistID)
				delete(playlist.Clients, clientID)
			}
		}
		config.BufferInformation.Store(key, playlist)
		return true
	})
}

func getClientIP(r *http.Request) string {
	// Check the X-Forwarded-For header first
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// X-Forwarded-For may contain multiple IP addresses; return the first one
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check the X-Real-IP header next
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fallback to RemoteAddr
	ip := r.RemoteAddr
	if strings.Contains(ip, ":") {
		// Remove port if present
		ip = strings.Split(ip, ":")[0]
	}

	return ip
}

func createStreamID(stream map[int]structs.ThisStream, ip, userAgent string) (streamID int) {
	streamID = 0
	uniqueIdentifier := fmt.Sprintf("%s-%s", ip, userAgent)

	for i := 0; i <= len(stream); i++ {
		if _, ok := stream[i]; !ok {
			streamID = i
			break
		}
	}

	if _, ok := stream[streamID]; ok && stream[streamID].ClientID == uniqueIdentifier {
		// Return the same ID if the combination already exists
		return streamID
	}

	return
}

func bufferingStream(playlistID string, streamingURL string, backupStream1 *structs.BackupStream, backupStream2 *structs.BackupStream, backupStream3 *structs.BackupStream, channelName string, w http.ResponseWriter, r *http.Request) {

	time.Sleep(time.Duration(config.Settings.BufferTimeout) * time.Millisecond)

	var playlist structs.Playlist
	var client structs.ThisClient
	var stream structs.ThisStream
	var streaming = false
	var streamID int
	var debug string
	var timeOut = 0
	var newStream = true

	//w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Connection", "close")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Check whether the playlist is already in use
	config.Lock.Lock()
	if p, ok := config.BufferInformation.Load(playlistID); !ok {
		config.Lock.Unlock() // Unlock early if not found
		var playlistType string

		// Playlist is not yet in use, create default values for the playlist
		playlist.Folder = config.System.Folder.Temp + playlistID + string(os.PathSeparator)
		playlist.PlaylistID = playlistID
		playlist.Streams = make(map[int]structs.ThisStream)
		playlist.Clients = make(map[int]structs.ThisClient)

		err := checkVFSFolder(playlist.Folder, config.BufferVFS)
		if err != nil {
			cli.ShowError(err, 000)
			httpStatusError(w, 404)
			return
		}

		switch playlist.PlaylistID[0:1] {

		case "M":
			playlistType = "m3u"

		case "H":
			playlistType = "hdhr"

		}

		var playListBuffer string
		config.SystemMutex.Lock()
		playListInterface := config.Settings.Files.M3U[playlistID]
		if playListInterface == nil {
			playListInterface = config.Settings.Files.HDHR[playlistID]
		}
		if playListMap, ok := playListInterface.(map[string]interface{}); ok {
			if buffer, ok := playListMap["buffer"].(string); ok {
				playListBuffer = buffer
			} else {
				playListBuffer = "-"
			}
		}
		config.SystemMutex.Unlock()

		playlist.Buffer = playListBuffer

		playlist.Tuner = getTuner(playlistID, playlistType)

		playlist.PlaylistName = getProviderParameter(playlist.PlaylistID, playlistType, "name")

		playlist.HttpProxyIP = getProviderParameter(playlist.PlaylistID, playlistType, "http_proxy.ip")
		playlist.HttpProxyPort = getProviderParameter(playlist.PlaylistID, playlistType, "http_proxy.port")

		playlist.HttpUserOrigin = getProviderParameter(playlist.PlaylistID, playlistType, "http_headers.origin")
		playlist.HttpUserReferer = getProviderParameter(playlist.PlaylistID, playlistType, "http_headers.referer")

		// Create default values for the stream
		streamID = createStreamID(playlist.Streams, getClientIP(r), r.UserAgent())

		client.Connection += 1

		stream.URL = streamingURL
		stream.BackupChannel1 = backupStream1
		stream.BackupChannel2 = backupStream2
		stream.BackupChannel3 = backupStream3
		stream.ChannelName = channelName
		stream.Status = false

		playlist.Streams[streamID] = stream
		playlist.Clients[streamID] = client

		config.Lock.Lock()
		config.BufferInformation.Store(playlistID, playlist)
		config.Lock.Unlock()

	} else {
		playlist = p.(structs.Playlist)
		config.Lock.Unlock()

		// Playlist is already used for streaming
		// Check if the URL is already streaming from another client.
		for id := range playlist.Streams {

			stream = playlist.Streams[id]
			client = playlist.Clients[id]

			stream.BackupChannel1 = backupStream1
			stream.BackupChannel2 = backupStream2
			stream.BackupChannel3 = backupStream3
			stream.ChannelName = channelName
			stream.Status = false

			if streamingURL == stream.URL {

				streamID = id
				newStream = false
				client.Connection += 1

				playlist.Clients[streamID] = client

				config.Lock.Lock()
				config.BufferInformation.Store(playlistID, playlist)
				config.Lock.Unlock()

				debug = fmt.Sprintf("Restream Status:Playlist: %s - Channel: %s - Connections: %d", playlist.PlaylistName, stream.ChannelName, client.Connection)

				cli.ShowDebug(debug, 1)

				if c, ok := config.BufferClients.Load(playlistID + stream.MD5); ok {

					var clients = c.(structs.ClientConnection)
					clients.Connection = client.Connection

					cli.ShowInfo(fmt.Sprintf("Streaming Status:Channel: %s (Clients: %d)", stream.ChannelName, clients.Connection))

					config.BufferClients.Store(playlistID+stream.MD5, clients)

				}

				break
			}

		}

		// New stream for an already active playlist
		if newStream {

			// Check if the playlist allows another stream (Tuner)
			if len(playlist.Streams) >= playlist.Tuner {
				// If there are backup URLs, use them
				if backupStream1 != nil {
					bufferingStream(backupStream1.PlaylistID, backupStream1.URL, nil, backupStream2, backupStream3, channelName, w, r)
				} else if backupStream2 != nil {
					bufferingStream(backupStream2.PlaylistID, backupStream2.URL, nil, nil, backupStream3, channelName, w, r)
				} else if backupStream3 != nil {
					bufferingStream(backupStream3.PlaylistID, backupStream3.URL, nil, nil, nil, channelName, w, r)
				}

				cli.ShowInfo(fmt.Sprintf("Streaming Status:Playlist: %s - No new connections available. Tuner = %d", playlist.PlaylistName, playlist.Tuner))

				if value, ok := web.WebUI["html/video/stream-limit.ts"]; ok {

					content := GetHTMLString(value.(string))

					w.WriteHeader(200)
					w.Header().Set("Content-type", "video/mpeg")
					w.Header().Set("Content-Length:", "0")

					for i := 1; i < 60; i++ {
						_ = i
						_, err := w.Write([]byte(content))
						if err != nil {
							cli.ShowError(err, 0)
							return
						}

						time.Sleep(time.Duration(500) * time.Millisecond)
					}

					return
				}

				return
			}

			// Playlist allows another stream (Tuner limit not yet reached)
			// Create default values for the stream
			stream = structs.ThisStream{}
			client = structs.ThisClient{}

			streamID = createStreamID(playlist.Streams, getClientIP(r), r.UserAgent())

			client.Connection = 1
			stream.URL = streamingURL
			stream.ChannelName = channelName
			stream.Status = false
			stream.BackupChannel1 = backupStream1
			stream.BackupChannel2 = backupStream2
			stream.BackupChannel3 = backupStream3

			playlist.Streams[streamID] = stream
			playlist.Clients[streamID] = client

			config.Lock.Lock()
			config.BufferInformation.Store(playlistID, playlist)
			config.Lock.Unlock()

		}

	}

	// Check whether the stream is already being played by another client
	if !playlist.Streams[streamID].Status && newStream {

		// New buffer is needed
		stream = playlist.Streams[streamID]
		stream.MD5 = getMD5(streamingURL)
		stream.Folder = playlist.Folder + stream.MD5 + string(os.PathSeparator)
		stream.PlaylistID = playlistID
		stream.PlaylistName = playlist.PlaylistName
		stream.BackupChannel1 = backupStream1
		stream.BackupChannel2 = backupStream2
		stream.BackupChannel3 = backupStream3

		playlist.Streams[streamID] = stream

		config.Lock.Lock()
		config.BufferInformation.Store(playlistID, playlist)
		config.Lock.Unlock()

		switch playlist.Buffer {

		case "ffmpeg", "vlc":
			go thirdPartyBuffer(streamID, playlistID, false, 0)

		default:
			break

		}

		cli.ShowInfo(fmt.Sprintf("Streaming Status 1:Playlist: %s - Tuner: %d / %d", playlist.PlaylistName, len(playlist.Streams), playlist.Tuner))

		var clients structs.ClientConnection
		clients.Connection = 1
		config.BufferClients.Store(playlistID+stream.MD5, clients)

	}

	w.WriteHeader(200)

	for { //Loop 1: Wait until the first segment has been downloaded through the buffer

		if p, ok := config.BufferInformation.Load(playlistID); ok {

			var playlist = p.(structs.Playlist)

			if stream, ok := playlist.Streams[streamID]; ok {

				if !stream.Status {

					timeOut++

					time.Sleep(time.Duration(100) * time.Millisecond)

					if c, ok := config.BufferClients.Load(playlistID + stream.MD5); ok {

						var clients = c.(structs.ClientConnection)

						if clients.Error != nil || (timeOut > 200 && (playlist.Streams[streamID].BackupChannel1 == nil && playlist.Streams[streamID].BackupChannel2 == nil && playlist.Streams[streamID].BackupChannel3 == nil)) {
							killClientConnection(streamID, stream.PlaylistID, false)
							return
						}

					}

					continue
				}

				var oldSegments []string

				for { // Loop 2: Temporary files are present, data can be sent to the client

					// Monitor HTTP client connection

					ctx := r.Context()
					if ok {

						select {

						case <-ctx.Done():
							killClientConnection(streamID, playlistID, false)
							return

						default:
							if c, ok := config.BufferClients.Load(playlistID + stream.MD5); ok {

								var clients = c.(structs.ClientConnection)
								if clients.Error != nil {
									cli.ShowError(clients.Error, 0)
									killClientConnection(streamID, playlistID, false)
									return
								}

							} else {

								return

							}

						}

					}

					if _, err := config.BufferVFS.Stat(stream.Folder); fsIsNotExistErr(err) {
						killClientConnection(streamID, playlistID, false)
						return
					}

					var tmpFiles = getBufTmpFiles(&stream)
					//fmt.Println("Buffer Loop:", stream.Connection)

					for _, f := range tmpFiles {

						if _, err := config.BufferVFS.Stat(stream.Folder); fsIsNotExistErr(err) {
							killClientConnection(streamID, playlistID, false)
							return
						}

						oldSegments = append(oldSegments, f)

						var fileName = stream.Folder + f

						file, err := config.BufferVFS.Open(fileName)
						if err != nil {
							debug = fmt.Sprintf("Buffer Open (%s)", fileName)
							cli.ShowDebug(debug, 2)
							return
						}
						defer func() {
							err = file.Close()
						}()

						if err == nil {
							l, err := file.Stat()
							if err == nil {

								debug = fmt.Sprintf("Buffer Status:Send to client (%s)", fileName)
								cli.ShowDebug(debug, 2)

								var buffer = make([]byte, int(l.Size()))
								_, err = file.Read(buffer)

								if err == nil {

									_, err = file.Seek(0, 0)
									if err != nil {
										cli.ShowError(err, 0)
										killClientConnection(streamID, playlistID, false)
										return
									}

									if !streaming {

										contentType := http.DetectContentType(buffer)
										_ = contentType
										w.Header().Set("Content-type", contentType)
										w.Header().Set("Content-Length", "0")
										w.Header().Set("Connection", "close")

									}

									/*
									   // HDHR Header
									   w.Header().Set("Cache-Control", "no-cache")
									   w.Header().Set("Pragma", "no-cache")
									   w.Header().Set("transferMode.dlna.org", "Streaming")
									*/

									_, err := w.Write(buffer)

									if err != nil {
										err = file.Close()
										if err != nil {
											cli.ShowError(err, 0)
										}
										killClientConnection(streamID, playlistID, false)
										return
									}

									err = file.Close()
									if err != nil {
										cli.ShowError(err, 0)
										killClientConnection(streamID, playlistID, false)
										return
									}
									streaming = true

								}

								err = file.Close()
								if err != nil {
									cli.ShowError(err, 0)
									killClientConnection(streamID, playlistID, false)
									return
								}
							}

							var n = indexOfString(f, oldSegments)

							if n > 20 {

								var fileToRemove = stream.Folder + oldSegments[0]
								if err = config.BufferVFS.RemoveAll(storage.GetPlatformFile(fileToRemove)); err != nil {
									cli.ShowError(err, 4007)
								}
								oldSegments = append(oldSegments[:0], oldSegments[0+1:]...)

							}

						}

						err = file.Close()
						if err != nil {
							cli.ShowError(err, 0)
							killClientConnection(streamID, playlistID, false)
							return
						}
					}

					if len(tmpFiles) == 0 {
						time.Sleep(time.Duration(100) * time.Millisecond)
					}

				} // End Loop 2

			} else {

				// Stream not found
				cli.ShowDebug("Streaming Status:Stream not found. Killing Connection", 3)
				killClientConnection(streamID, stream.PlaylistID, false)
				cli.ShowInfo(fmt.Sprintf("Streaming Status:Playlist: %s - Tuner: %d / %d", playlist.PlaylistName, len(playlist.Streams), playlist.Tuner))
				return

			}

		} // End BufferInformation

	} // End Loop 1

}

func getBufTmpFiles(stream *structs.ThisStream) (tmpFiles []string) {

	var tmpFolder = stream.Folder
	var fileIDs []float64

	if _, err := config.BufferVFS.Stat(tmpFolder); !fsIsNotExistErr(err) {

		files, err := config.BufferVFS.ReadDir(getPlatformPath(tmpFolder))
		if err != nil {
			cli.ShowError(err, 000)
			return
		}

		if len(files) > 2 {

			for _, file := range files {

				var fileID = strings.ReplaceAll(file.Name(), ".ts", "")
				var f, err = strconv.ParseFloat(fileID, 64)

				if err == nil {
					fileIDs = append(fileIDs, f)
				}

			}

			sort.Float64s(fileIDs)
			fileIDs = fileIDs[:len(fileIDs)-1]

			for _, file := range fileIDs {

				var fileName = fmt.Sprintf("%d.ts", int64(file))

				if indexOfString(fileName, stream.OldSegments) == -1 {
					tmpFiles = append(tmpFiles, fileName)
					stream.OldSegments = append(stream.OldSegments, fileName)
				}

			}

		}

	}

	return
}

func killClientConnection(streamID int, playlistID string, force bool) {
	config.Lock.Lock()
	defer config.Lock.Unlock()

	if p, ok := config.BufferInformation.Load(playlistID); ok {
		var playlist = p.(structs.Playlist)

		if force {
			delete(playlist.Streams, streamID)
			if len(playlist.Streams) == 0 {
				config.BufferInformation.Delete(playlistID)
			} else {
				config.BufferInformation.Store(playlistID, playlist)
			}
			cli.ShowInfo(fmt.Sprintf("Streaming Status: Playlist: %s - Tuner: %d / %d", playlist.PlaylistName, len(playlist.Streams), playlist.Tuner))
			return
		}

		if stream, ok := playlist.Streams[streamID]; ok {
			client := playlist.Clients[streamID]

			if c, ok := config.BufferClients.Load(playlistID + stream.MD5); ok {
				var clients = c.(structs.ClientConnection)
				clients.Connection--
				client.Connection--

				// Ensure client connections cannot go below zero
				if client.Connection < 0 {
					client.Connection = 0
				}
				if clients.Connection < 0 {
					clients.Connection = 0
				}

				playlist.Clients[streamID] = client
				config.BufferClients.Store(playlistID+stream.MD5, clients)

				cli.ShowInfo(fmt.Sprintf("Streaming Status: Channel: %s (Clients: %d)", stream.ChannelName, clients.Connection))

				if clients.Connection <= 0 {
					config.BufferClients.Delete(playlistID + stream.MD5)
					delete(playlist.Streams, streamID)
					delete(playlist.Clients, streamID)

					if len(playlist.Streams) == 0 {
						config.BufferInformation.Delete(playlistID)
					} else {
						config.BufferInformation.Store(playlistID, playlist)
					}
				} else {
					config.BufferInformation.Store(playlistID, playlist)
				}

				if len(playlist.Streams) > 0 {
					cli.ShowInfo(fmt.Sprintf("Streaming Status: Playlist: %s - Tuner: %d / %d", playlist.PlaylistName, len(playlist.Streams), playlist.Tuner))
				}
			}
		}
	}
}

func clientConnection(stream structs.ThisStream) (status bool) {

	status = true
	config.Lock.Lock()
	defer config.Lock.Unlock()

	if _, ok := config.BufferClients.Load(stream.PlaylistID + stream.MD5); !ok {

		var debug = fmt.Sprintf("Streaming Status:Remove temporary files (%s)", stream.Folder)
		cli.ShowDebug(debug, 1)

		status = false

		debug = fmt.Sprintf("Remove tmp folder:%s", stream.Folder)
		cli.ShowDebug(debug, 1)

		if err := config.BufferVFS.RemoveAll(stream.Folder); err != nil {
			cli.ShowError(err, 4005)
		}

		if p, ok := config.BufferInformation.Load(stream.PlaylistID); !ok {

			cli.ShowInfo(fmt.Sprintf("Streaming Status:Channel: %s - No client is using this channel anymore. Streaming Server connection has ended", stream.ChannelName))

			if p != nil {
				var playlist = p.(structs.Playlist)

				cli.ShowInfo(fmt.Sprintf("Streaming Status:Playlist: %s - Tuner: %d / %d", playlist.PlaylistName, len(playlist.Streams), playlist.Tuner))

				if len(playlist.Streams) <= 0 {
					config.BufferInformation.Delete(stream.PlaylistID)
				}
			}

		}

		status = false

	}

	return
}

// Buffer with FFMPEG
func thirdPartyBuffer(streamID int, playlistID string, useBackup bool, backupNumber int) {

	if p, ok := config.BufferInformation.Load(playlistID); ok {

		var playlist = p.(structs.Playlist)
		var debug, path, options, bufferType string
		var tmpSegment = 1
		var bufferSize = config.Settings.BufferSize * 1024
		var stream = playlist.Streams[streamID]
		var buf bytes.Buffer
		var fileSize = 0
		var streamStatus = make(chan bool)

		var tmpFolder = playlist.Streams[streamID].Folder
		var url = playlist.Streams[streamID].URL
		if useBackup {
			if backupNumber >= 1 && backupNumber <= 3 {
				switch backupNumber {
				case 1:
					if stream.BackupChannel1 != nil {
						url = stream.BackupChannel1.URL
						cli.ShowHighlight("START OF BACKUP 1 STREAM")
						cli.ShowInfo("Backup Channel 1 URL: " + url)
					}
				case 2:
					if stream.BackupChannel2 != nil {
						url = stream.BackupChannel2.URL
						cli.ShowHighlight("START OF BACKUP 2 STREAM")
						cli.ShowInfo("Backup Channel 2 URL: " + url)
					}
				case 3:
					if stream.BackupChannel3 != nil {
						url = stream.BackupChannel3.URL
						cli.ShowHighlight("START OF BACKUP 3 STREAM")
						cli.ShowInfo("Backup Channel 3 URL: " + url)
					}
				}
			}
		}

		stream.Status = false

		bufferType = strings.ToUpper(playlist.Buffer)

		switch playlist.Buffer {

		case "ffmpeg":

			if config.Settings.FFmpegForceHttp {
				url = strings.ReplaceAll(url, "https://", "http://")
				cli.ShowInfo("Forcing URL to HTTP for FFMPEG: " + url)
			}

			path = config.Settings.FFmpegPath
			options = config.Settings.FFmpegOptions

		case "vlc":
			path = config.Settings.VLCPath
			options = config.Settings.VLCOptions

		default:
			return
		}

		var addErrorToStream = func(err error) {
			if !useBackup || (useBackup && backupNumber >= 0 && backupNumber <= 3) {
				backupNumber = backupNumber + 1
				if stream.BackupChannel1 != nil || stream.BackupChannel2 != nil || stream.BackupChannel3 != nil {
					thirdPartyBuffer(streamID, playlistID, true, backupNumber)
				}
				return
			}

			var stream = playlist.Streams[streamID]

			if c, ok := config.BufferClients.Load(playlistID + stream.MD5); ok {

				var clients = c.(structs.ClientConnection)
				clients.Error = err
				config.BufferClients.Store(playlistID+stream.MD5, clients)

			}

		}

		if err := config.BufferVFS.RemoveAll(getPlatformPath(tmpFolder)); err != nil {
			cli.ShowError(err, 4005)
		}

		err := checkVFSFolder(tmpFolder, config.BufferVFS)
		if err != nil {
			cli.ShowError(err, 0)
			killClientConnection(streamID, playlistID, false)
			addErrorToStream(err)
			return
		}

		err = checkFile(path)
		if err != nil {
			cli.ShowError(err, 0)
			killClientConnection(streamID, playlistID, false)
			addErrorToStream(err)
			return
		}

		cli.ShowInfo(fmt.Sprintf("%s path:%s", bufferType, path))
		cli.ShowInfo("Streaming URL:" + url)

		var tmpFile = fmt.Sprintf("%s%d.ts", tmpFolder, tmpSegment)

		f, err := config.BufferVFS.Create(tmpFile)
		if err != nil {
			cli.ShowError(err, 0)
			killClientConnection(streamID, playlistID, false)
			addErrorToStream(err)
			return
		}

		defer func() {
			err = f.Close()
		}()
		if err != nil {
			cli.ShowError(err, 0)
			killClientConnection(streamID, playlistID, false)
			addErrorToStream(err)
			return
		}

		// Set User-Agent
		var args []string

		for i, a := range strings.Split(options, " ") {

			switch bufferType {
			case "FFMPEG":
				a = strings.ReplaceAll(a, "[URL]", url)
				if i == 0 {
					if len(config.Settings.UserAgent) != 0 {
						args = []string{"-user_agent", config.Settings.UserAgent}
					}

					if playlist.HttpProxyIP != "" && playlist.HttpProxyPort != "" {
						args = append(args, "-http_proxy", fmt.Sprintf("http://%s:%s", playlist.HttpProxyIP, playlist.HttpProxyPort))
					}

					var headers string
					if len(playlist.HttpUserReferer) != 0 {
						headers += fmt.Sprintf("Referer: %s\r\n", playlist.HttpUserReferer)
					}
					if len(playlist.HttpUserOrigin) != 0 {
						headers += fmt.Sprintf("Origin: %s\r\n", playlist.HttpUserOrigin)
					}
					if headers != "" {
						args = append(args, "-headers", headers)
					}
				}

				args = append(args, a)

			case "VLC":
				if a == "[URL]" {
					a = strings.ReplaceAll(a, "[URL]", url)
					args = append(args, a)

					if len(config.Settings.UserAgent) != 0 {
						args = append(args, fmt.Sprintf(":http-user-agent=%s", config.Settings.UserAgent))
					}

					if len(playlist.HttpUserReferer) != 0 {
						args = append(args, fmt.Sprintf(":http-referrer=%s", playlist.HttpUserReferer))
					}

					if playlist.HttpProxyIP != "" && playlist.HttpProxyPort != "" {
						args = append(args, fmt.Sprintf(":http-proxy=%s:%s", playlist.HttpProxyIP, playlist.HttpProxyPort))
					}

				} else {
					args = append(args, a)
				}

			}

		}

		var cmd = exec.Command(path, args...)
		// Set this explicitly to avoid issues with VLC
		cmd.Env = append(os.Environ(), "DISPLAY=:0")

		debug = fmt.Sprintf("BUFFER DEBUG: %s:%s %s", bufferType, path, args)
		cli.ShowDebug(debug, 1)

		// Byte data from the process
		stdOut, err := cmd.StdoutPipe()
		if err != nil {
			cli.ShowError(err, 0)
			killClientConnection(streamID, playlistID, false)
			addErrorToStream(err)
			return
		}

		// Log data from the process
		logOut, err := cmd.StderrPipe()
		if err != nil {
			cli.ShowError(err, 0)
			killClientConnection(streamID, playlistID, false)
			addErrorToStream(err)
			return
		}

		if len(buf.Bytes()) == 0 && !stream.Status {
			cli.ShowInfo(bufferType + ":Processing data")
		}

		err = cmd.Start()
		if err != nil {
			cli.ShowError(err, 0)
			killClientConnection(streamID, playlistID, false)
			addErrorToStream(err)
			return
		}

		defer func() {
			err = cmd.Wait()
		}()
		if err != nil {
			cli.ShowError(err, 0)
			killClientConnection(streamID, playlistID, false)
			addErrorToStream(err)
			return
		}

		go func() {

			// Display log data from the process in debug mode 1.
			scanner := bufio.NewScanner(logOut)
			scanner.Split(bufio.ScanLines)

			for scanner.Scan() {

				debug = fmt.Sprintf("%s log:%s", bufferType, strings.TrimSpace(scanner.Text()))

				select {
				case <-streamStatus:
					cli.ShowDebug(debug, 1)
				default:
					cli.ShowInfo(debug)
				}

				time.Sleep(time.Duration(10) * time.Millisecond)

			}

		}()

		f, err = config.BufferVFS.OpenFile(tmpFile, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			panic(err)
		}
		defer func() {
			err = f.Close()
		}()
		if err != nil {
			cli.ShowError(err, 0)
			killClientConnection(streamID, playlistID, false)
			addErrorToStream(err)
			return
		}

		buffer := make([]byte, 1024*4)

		reader := bufio.NewReader(stdOut)

		t := make(chan int)

		go func() {

			var timeout = 0
			for {
				time.Sleep(time.Duration(1000) * time.Millisecond)
				timeout++

				select {
				case <-t:
					return
				default:
					// Check if the channel is closed before sending
					select {
					case t <- timeout:
					default:
					}
				}

			}

		}()

		for {

			select {
			case timeout := <-t:
				if timeout >= 20 && tmpSegment == 1 {
					err = cmd.Process.Kill()
					if err != nil {
						cli.ShowError(err, 0)
					}
					err = errors.New("Timeout")
					cli.ShowError(err, 4006)
					killClientConnection(streamID, playlistID, false)
					addErrorToStream(err)
					err = cmd.Wait()
					if err != nil {
						cli.ShowError(err, 0)
					}
					err = f.Close()
					if err != nil {
						cli.ShowError(err, 0)
					}
					return
				}

			default:

			}

			if fileSize == 0 && !stream.Status {
				cli.ShowInfo("Streaming Status:Receive data from " + bufferType)
			}

			if !clientConnection(stream) {
				err = cmd.Process.Kill()
				if err != nil {
					cli.ShowError(err, 0)
				}

				err = f.Close()
				if err != nil {
					cli.ShowError(err, 0)
				}

				err = cmd.Wait()
				if err != nil {
					cli.ShowError(err, 0)
				}
				return
			}

			n, err := reader.Read(buffer)
			if err == io.EOF {
				break
			}

			fileSize = fileSize + len(buffer[:n])

			if _, err := f.Write(buffer[:n]); err != nil {
				cli.ShowError(err, 0)
				err = cmd.Process.Kill()
				if err != nil {
					cli.ShowError(err, 0)
				}
				killClientConnection(streamID, playlistID, false)
				addErrorToStream(err)
				err = cmd.Wait()
				if err != nil {
					cli.ShowError(err, 0)
				}

				return
			}

			if fileSize >= bufferSize/2 {

				if tmpSegment == 1 && !stream.Status {
					close(t)
					close(streamStatus)
					cli.ShowInfo(fmt.Sprintf("Streaming Status:Buffering data from %s", bufferType))
				}

				err = f.Close()
				if err != nil {
					cli.ShowError(err, 0)
					return
				}

				tmpSegment++

				if !stream.Status {
					config.Lock.Lock()
					stream.Status = true
					playlist.Streams[streamID] = stream
					config.BufferInformation.Store(playlistID, playlist)
					config.Lock.Unlock()
				}

				tmpFile = fmt.Sprintf("%s%d.ts", tmpFolder, tmpSegment)

				fileSize = 0

				var errCreate, errOpen error
				_, errCreate = config.BufferVFS.Create(tmpFile)
				f, errOpen = config.BufferVFS.OpenFile(tmpFile, os.O_APPEND|os.O_WRONLY, 0600)
				if errCreate != nil || errOpen != nil {
					cli.ShowError(err, 0)
					err = cmd.Process.Kill()
					if err != nil {
						cli.ShowError(err, 0)
					}

					killClientConnection(streamID, playlistID, false)
					addErrorToStream(err)
					err = cmd.Wait()
					if err != nil {
						cli.ShowError(err, 0)
					}
					return
				}

			}

		}

		err = cmd.Process.Kill()
		if err != nil {
			cli.ShowError(err, 0)
		}

		err = cmd.Wait()
		if err != nil {
			cli.ShowError(err, 0)
		}

		err = errors.New(bufferType + " error")
		addErrorToStream(err)
		cli.ShowError(err, 1204)

		time.Sleep(time.Duration(500) * time.Millisecond)
		clientConnection(stream)

		return

	}

}

func getTuner(id, playlistType string) (tuner int) {

	var playListBuffer string
	config.SystemMutex.Lock()
	playListInterface := config.Settings.Files.M3U[id]
	if playListInterface == nil {
		playListInterface = config.Settings.Files.HDHR[id]
	}
	if playListMap, ok := playListInterface.(map[string]interface{}); ok {
		if buffer, ok := playListMap["buffer"].(string); ok {
			playListBuffer = buffer
		} else {
			playListBuffer = "-"
		}
	}
	config.SystemMutex.Unlock()

	switch playListBuffer {

	case "-":
		tuner = config.Settings.Tuner

	case "threadfin", "ffmpeg", "vlc":

		i, err := strconv.Atoi(getProviderParameter(id, playlistType, "tuner"))
		if err == nil {
			tuner = i
		} else {
			cli.ShowError(err, 0)
			tuner = 1
		}

	}

	return
}

func initBufferVFS() {
	config.BufferVFS = memfs.New()
}
