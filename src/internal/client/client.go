package client

import (
	"fmt"
	"net/http"
	"strings"
	"threadfin/internal/cli"
	"threadfin/internal/config"
	"threadfin/internal/structs"
)

func Connection(stream structs.ThisStream) (status bool) {
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

func KillConnection(streamID int, playlistID string, force bool) {
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

func GetIP(r *http.Request) string {
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

func GetActiveCount() (count int) {
	count = 0
	cleanUpStale() // Ensure stale clients are removed first

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

func cleanUpStale() {
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
