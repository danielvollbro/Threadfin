package xmltv

import (
	"threadfin/internal/config"
	"threadfin/internal/structs"
)

// Programm Poster Cover aus der XMLTV Datei laden
func getPoster(program *structs.Program, xmltvProgram *structs.Program, xepgChannel structs.XEPGChannelStruct, forceHttps bool) {

	var imgc = config.Data.Cache.Images

	for _, poster := range xmltvProgram.Poster {
		poster.Src = imgc.Image.GetURL(poster.Src, config.Settings.HttpThreadfinDomain, config.Settings.Port, config.Settings.ForceHttps, config.Settings.HttpsPort, config.Settings.HttpsThreadfinDomain)
		program.Poster = append(program.Poster, poster)
	}

	if config.Settings.XepgReplaceMissingImages {

		if len(xmltvProgram.Poster) == 0 {
			var poster structs.Poster
			poster.Src = imgc.Image.GetURL(xepgChannel.TvgLogo, config.Settings.HttpThreadfinDomain, config.Settings.Port, config.Settings.ForceHttps, config.Settings.HttpsPort, config.Settings.HttpsThreadfinDomain)
			program.Poster = append(program.Poster, poster)
		}

	}

}
