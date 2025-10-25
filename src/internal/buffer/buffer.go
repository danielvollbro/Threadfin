package buffer

/*
  Render tuner-limit image as video [ffmpeg]
  -loop 1 -i stream-limit.jpg -c:v libx264 -t 1 -pix_fmt yuv420p -vf scale=1920:1080  stream-limit.ts
*/

import (
	"threadfin/src/internal/config"

	"github.com/avfs/avfs/vfs/memfs"
)

func InitVFS() {
	config.BufferVFS = memfs.New()
}
