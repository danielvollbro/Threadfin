package xmltv

import (
	"strings"
	"threadfin/src/internal/structs"
)

// Videoparameter erstellen (createXMLTVFile)
func getVideo(program *structs.Program, xmltvProgram *structs.Program, xepgChannel structs.XEPGChannelStruct) {

	var video structs.Video
	video.Present = xmltvProgram.Video.Present
	video.Colour = xmltvProgram.Video.Colour
	video.Aspect = xmltvProgram.Video.Aspect
	video.Quality = xmltvProgram.Video.Quality

	if len(xmltvProgram.Video.Quality) == 0 {

		if strings.Contains(strings.ToUpper(xepgChannel.XName), " HD") || strings.Contains(strings.ToUpper(xepgChannel.XName), " FHD") {
			video.Quality = "HDTV"
		}

		if strings.Contains(strings.ToUpper(xepgChannel.XName), " UHD") || strings.Contains(strings.ToUpper(xepgChannel.XName), " 4K") {
			video.Quality = "UHDTV"
		}

	}

	program.Video = video
}
