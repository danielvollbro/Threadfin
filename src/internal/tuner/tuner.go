package tuner

import (
	"strconv"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/config"
	"threadfin/src/internal/provider"
)

func Get(id, playlistType string) (tuner int) {
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

		i, err := strconv.Atoi(provider.GetProviderParameter(id, playlistType, "tuner"))
		if err == nil {
			tuner = i
		} else {
			cli.ShowError(err, 0)
			tuner = 1
		}

	}

	return
}
