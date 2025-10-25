package src

/*
  Render tuner-limit image as video [ffmpeg]
  -loop 1 -i stream-limit.jpg -c:v libx264 -t 1 -pix_fmt yuv420p -vf scale=1920:1080  stream-limit.ts
*/

import (
	"threadfin/src/internal/config"

	"github.com/avfs/avfs/vfs/memfs"
)

func getActivePlaylistCount() (count int) {
	count = 0
	config.BufferInformation.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

func initBufferVFS() {
	config.BufferVFS = memfs.New()
}
