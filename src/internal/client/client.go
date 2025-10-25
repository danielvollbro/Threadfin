package client

import (
	"fmt"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/config"
	"threadfin/src/internal/structs"
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
