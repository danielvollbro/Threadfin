package stream

import (
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"threadfin/src/internal/buffer"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/client"
	"threadfin/src/internal/config"
	"threadfin/src/internal/crypt"
	"threadfin/src/internal/provider"
	"threadfin/src/internal/storage"
	"threadfin/src/internal/structs"
	"threadfin/src/internal/tuner"
	"threadfin/src/internal/utilities"
	"threadfin/src/web"
	"time"
)

func Buffering(playlistID string, streamingURL string, backupStream1 *structs.BackupStream, backupStream2 *structs.BackupStream, backupStream3 *structs.BackupStream, channelName string, w http.ResponseWriter, r *http.Request) {
	time.Sleep(time.Duration(config.Settings.BufferTimeout) * time.Millisecond)

	var playlist structs.Playlist
	var currentClient structs.ThisClient
	var currentStream structs.ThisStream
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

		err := storage.CheckVFSFolder(playlist.Folder, config.BufferVFS)
		if err != nil {
			cli.ShowError(err, 000)
			web.HttpStatusError(w, 404)
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

		playlist.Tuner = tuner.Get(playlistID, playlistType)

		playlist.PlaylistName = provider.GetProviderParameter(playlist.PlaylistID, playlistType, "name")

		playlist.HttpProxyIP = provider.GetProviderParameter(playlist.PlaylistID, playlistType, "http_proxy.ip")
		playlist.HttpProxyPort = provider.GetProviderParameter(playlist.PlaylistID, playlistType, "http_proxy.port")

		playlist.HttpUserOrigin = provider.GetProviderParameter(playlist.PlaylistID, playlistType, "http_headers.origin")
		playlist.HttpUserReferer = provider.GetProviderParameter(playlist.PlaylistID, playlistType, "http_headers.referer")

		// Create default values for the stream
		streamID = CreateID(playlist.Streams, client.GetIP(r), r.UserAgent())

		currentClient.Connection += 1

		currentStream.URL = streamingURL
		currentStream.BackupChannel1 = backupStream1
		currentStream.BackupChannel2 = backupStream2
		currentStream.BackupChannel3 = backupStream3
		currentStream.ChannelName = channelName
		currentStream.Status = false

		playlist.Streams[streamID] = currentStream
		playlist.Clients[streamID] = currentClient

		config.Lock.Lock()
		config.BufferInformation.Store(playlistID, playlist)
		config.Lock.Unlock()

	} else {
		playlist = p.(structs.Playlist)
		config.Lock.Unlock()

		// Playlist is already used for streaming
		// Check if the URL is already streaming from another client.
		for id := range playlist.Streams {

			currentStream = playlist.Streams[id]
			currentClient = playlist.Clients[id]

			currentStream.BackupChannel1 = backupStream1
			currentStream.BackupChannel2 = backupStream2
			currentStream.BackupChannel3 = backupStream3
			currentStream.ChannelName = channelName
			currentStream.Status = false

			if streamingURL == currentStream.URL {

				streamID = id
				newStream = false
				currentClient.Connection += 1

				playlist.Clients[streamID] = currentClient

				config.Lock.Lock()
				config.BufferInformation.Store(playlistID, playlist)
				config.Lock.Unlock()

				debug = fmt.Sprintf("Restream Status:Playlist: %s - Channel: %s - Connections: %d", playlist.PlaylistName, currentStream.ChannelName, currentClient.Connection)

				cli.ShowDebug(debug, 1)

				if c, ok := config.BufferClients.Load(playlistID + currentStream.MD5); ok {

					var clients = c.(structs.ClientConnection)
					clients.Connection = currentClient.Connection

					cli.ShowInfo(fmt.Sprintf("Streaming Status:Channel: %s (Clients: %d)", currentStream.ChannelName, clients.Connection))

					config.BufferClients.Store(playlistID+currentStream.MD5, clients)

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
					Buffering(backupStream1.PlaylistID, backupStream1.URL, nil, backupStream2, backupStream3, channelName, w, r)
				} else if backupStream2 != nil {
					Buffering(backupStream2.PlaylistID, backupStream2.URL, nil, nil, backupStream3, channelName, w, r)
				} else if backupStream3 != nil {
					Buffering(backupStream3.PlaylistID, backupStream3.URL, nil, nil, nil, channelName, w, r)
				}

				cli.ShowInfo(fmt.Sprintf("Streaming Status:Playlist: %s - No new connections available. Tuner = %d", playlist.PlaylistName, playlist.Tuner))

				if value, ok := web.WebUI["html/video/stream-limit.ts"]; ok {

					content := web.GetHTMLString(value.(string))

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
			currentStream = structs.ThisStream{}
			currentClient = structs.ThisClient{}

			streamID = CreateID(playlist.Streams, client.GetIP(r), r.UserAgent())

			currentClient.Connection = 1
			currentStream.URL = streamingURL
			currentStream.ChannelName = channelName
			currentStream.Status = false
			currentStream.BackupChannel1 = backupStream1
			currentStream.BackupChannel2 = backupStream2
			currentStream.BackupChannel3 = backupStream3

			playlist.Streams[streamID] = currentStream
			playlist.Clients[streamID] = currentClient

			config.Lock.Lock()
			config.BufferInformation.Store(playlistID, playlist)
			config.Lock.Unlock()

		}

	}

	// Check whether the stream is already being played by another client
	if !playlist.Streams[streamID].Status && newStream {

		// New buffer is needed
		currentStream = playlist.Streams[streamID]
		currentStream.MD5 = crypt.GetMD5(streamingURL)
		currentStream.Folder = playlist.Folder + currentStream.MD5 + string(os.PathSeparator)
		currentStream.PlaylistID = playlistID
		currentStream.PlaylistName = playlist.PlaylistName
		currentStream.BackupChannel1 = backupStream1
		currentStream.BackupChannel2 = backupStream2
		currentStream.BackupChannel3 = backupStream3

		playlist.Streams[streamID] = currentStream

		config.Lock.Lock()
		config.BufferInformation.Store(playlistID, playlist)
		config.Lock.Unlock()

		switch playlist.Buffer {

		case "ffmpeg", "vlc":
			go buffer.ThirdParty(streamID, playlistID, false, 0)

		default:
			break

		}

		cli.ShowInfo(fmt.Sprintf("Streaming Status 1:Playlist: %s - Tuner: %d / %d", playlist.PlaylistName, len(playlist.Streams), playlist.Tuner))

		var clients structs.ClientConnection
		clients.Connection = 1
		config.BufferClients.Store(playlistID+currentStream.MD5, clients)

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
							client.KillConnection(streamID, stream.PlaylistID, false)
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
							client.KillConnection(streamID, playlistID, false)
							return

						default:
							if c, ok := config.BufferClients.Load(playlistID + stream.MD5); ok {

								var clients = c.(structs.ClientConnection)
								if clients.Error != nil {
									cli.ShowError(clients.Error, 0)
									client.KillConnection(streamID, playlistID, false)
									return
								}

							} else {

								return

							}

						}

					}

					if _, err := config.BufferVFS.Stat(stream.Folder); storage.FSIsNotExistErr(err) {
						client.KillConnection(streamID, playlistID, false)
						return
					}

					var tmpFiles = getBufTmpFiles(&stream)
					//fmt.Println("Buffer Loop:", stream.Connection)

					for _, f := range tmpFiles {

						if _, err := config.BufferVFS.Stat(stream.Folder); storage.FSIsNotExistErr(err) {
							client.KillConnection(streamID, playlistID, false)
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
										client.KillConnection(streamID, playlistID, false)
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
										client.KillConnection(streamID, playlistID, false)
										return
									}

									err = file.Close()
									if err != nil {
										cli.ShowError(err, 0)
										client.KillConnection(streamID, playlistID, false)
										return
									}
									streaming = true

								}

								err = file.Close()
								if err != nil {
									cli.ShowError(err, 0)
									client.KillConnection(streamID, playlistID, false)
									return
								}
							}

							var n = utilities.IndexOfString(f, oldSegments)

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
							client.KillConnection(streamID, playlistID, false)
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
				client.KillConnection(streamID, stream.PlaylistID, false)
				cli.ShowInfo(fmt.Sprintf("Streaming Status:Playlist: %s - Tuner: %d / %d", playlist.PlaylistName, len(playlist.Streams), playlist.Tuner))
				return

			}

		} // End BufferInformation

	} // End Loop 1

}

func getBufTmpFiles(stream *structs.ThisStream) (tmpFiles []string) {

	var tmpFolder = stream.Folder
	var fileIDs []float64

	if _, err := config.BufferVFS.Stat(tmpFolder); !storage.FSIsNotExistErr(err) {

		files, err := config.BufferVFS.ReadDir(storage.GetPlatformPath(tmpFolder))
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

				if utilities.IndexOfString(fileName, stream.OldSegments) == -1 {
					tmpFiles = append(tmpFiles, fileName)
					stream.OldSegments = append(stream.OldSegments, fileName)
				}

			}

		}

	}

	return
}
