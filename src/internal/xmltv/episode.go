package xmltv

import (
	"strings"
	"threadfin/internal/cli"
	"threadfin/internal/structs"
	"time"
)

// Episodensystem übernehmen, falls keins vorhanden ist und eine Kategorie im Mapping eingestellt wurden, wird eine Episode erstellt
func getEpisodeNum(program *structs.Program, xmltvProgram *structs.Program, xepgChannel structs.XEPGChannelStruct) {

	program.EpisodeNum = xmltvProgram.EpisodeNum

	if len(xepgChannel.XCategory) > 0 && xepgChannel.XCategory != "Movie" {

		if len(xmltvProgram.EpisodeNum) == 0 {

			var timeLayout = "20060102150405"

			t, err := time.Parse(timeLayout, strings.Split(xmltvProgram.Start, " ")[0])
			if err == nil {
				program.EpisodeNum = append(program.EpisodeNum, &structs.EpisodeNum{Value: t.Format("2006-01-02 15:04:05"), System: "original-air-date"})
			} else {
				cli.ShowError(err, 0)
			}

		}

	}
}
