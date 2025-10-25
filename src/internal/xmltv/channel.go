package xmltv

import (
	"encoding/json"
	"log"
	"strings"
	"threadfin/internal/config"
	"threadfin/internal/structs"
	"unicode"
)

func GetData(xepgChannel structs.XEPGChannelStruct) (xepgXML structs.XMLTV, err error) {
	var xmltvFile = config.System.Folder.Data + xepgChannel.XmltvFile
	var channelID = xepgChannel.XMapping

	var xmltvStruct structs.XMLTV

	if strings.Contains(xmltvFile, "Threadfin Dummy") {
		xmltvStruct = createDummy(xepgChannel)
	} else {
		if xepgChannel.XmltvFile != "" {
			err = GetLocal(xmltvFile, &xmltvStruct)
			if err != nil {
				return
			}
		}
	}

	for _, xmltvProgram := range xmltvStruct.Program {
		if xmltvProgram.Channel == channelID {
			var program = &structs.Program{}

			// Channel ID
			program.Channel = xepgChannel.XChannelID
			program.Start = xmltvProgram.Start
			program.Stop = xmltvProgram.Stop

			// Title
			if len(xmltvProgram.Title) > 0 {
				if !config.Settings.EnableNonAscii {
					xmltvProgram.Title[0].Value = strings.TrimSpace(strings.Map(func(r rune) rune {
						if r > unicode.MaxASCII {
							return -1
						}
						return r
					}, xmltvProgram.Title[0].Value))
				}
				program.Title = xmltvProgram.Title
			}

			filters := []structs.FilterStruct{}
			for _, filter := range config.Settings.Filter {
				filter_json, _ := json.Marshal(filter)
				f := structs.FilterStruct{}
				err = json.Unmarshal(filter_json, &f)
				if err != nil {
					log.Println("XEPG:getProgramData:Error unmarshalling filter:", err)
					return
				}
				filters = append(filters, f)
			}

			// Category (Kategorie)
			getCategory(program, xmltvProgram, xepgChannel, filters)

			// Sub-Title
			program.SubTitle = xmltvProgram.SubTitle

			// Description
			program.Desc = xmltvProgram.Desc

			// Credits : (Credits)
			program.Credits = xmltvProgram.Credits

			// Rating (Bewertung)
			program.Rating = xmltvProgram.Rating

			// StarRating (Bewertung / Kritiken)
			program.StarRating = xmltvProgram.StarRating

			// Country (Länder)
			program.Country = xmltvProgram.Country

			// Program icon (Poster / Cover)
			getPoster(program, xmltvProgram, xepgChannel, config.Settings.ForceHttps)

			// Language (Sprache)
			program.Language = xmltvProgram.Language

			// Episodes numbers (Episodennummern)
			getEpisodeNum(program, xmltvProgram, xepgChannel)

			// Video (Videoparameter)
			getVideo(program, xmltvProgram, xepgChannel)

			// Date (Datum)
			program.Date = xmltvProgram.Date

			// Previously shown (Wiederholung)
			program.PreviouslyShown = xmltvProgram.PreviouslyShown

			// New (Neu)
			program.New = xmltvProgram.New

			// Live
			program.Live = xmltvProgram.Live

			// Premiere
			program.Premiere = xmltvProgram.Premiere

			xepgXML.Program = append(xepgXML.Program, program)

		}

	}

	return
}
