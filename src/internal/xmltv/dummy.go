package xmltv

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"threadfin/internal/cli"
	"threadfin/internal/config"
	"threadfin/internal/programs"
	"threadfin/internal/structs"
	"time"
	"unicode"
)

// Dummy Daten erstellen (createXMLTVFile)
func createDummy(xepgChannel structs.XEPGChannelStruct) (dummyXMLTV structs.XMLTV) {
	if xepgChannel.XMapping == "PPV" {
		var channelID = xepgChannel.XMapping
		programs := programs.CreateLiveProgram(xepgChannel, channelID)
		dummyXMLTV.Program = programs
		return
	}

	var imgc = config.Data.Cache.Images
	var currentTime = time.Now()
	var dateArray = strings.Fields(currentTime.String())
	var offset = " " + dateArray[2]
	var currentDay = currentTime.Format("20060102")
	var startTime, _ = time.Parse("20060102150405", currentDay+"000000")

	cli.ShowInfo("Create Dummy Guide:" + "Time offset" + offset + " - " + xepgChannel.XName)

	var dummyLength = 30 // Default to 30 minutes if parsing fails
	var err error
	var dl = strings.Split(xepgChannel.XMapping, "_")
	if dl[0] != "" {
		// Check if the first part is a valid integer
		if match, _ := regexp.MatchString(`^\d+$`, dl[0]); match {
			dummyLength, err = strconv.Atoi(dl[0])
			if err != nil {
				cli.ShowError(err, 000)
				// Continue with default value instead of returning
			}
		} else {
			// For non-numeric formats that aren't "PPV" (which is handled above),
			// use the default value
			cli.ShowInfo(fmt.Sprintf("Non-numeric format for XMapping: %s, using default duration of 30 minutes", xepgChannel.XMapping))
		}
	}

	for d := 0; d < 4; d++ {

		var epgStartTime = startTime.Add(time.Hour * time.Duration(d*24))

		for t := dummyLength; t <= 1440; t = t + dummyLength {

			var epgStopTime = epgStartTime.Add(time.Minute * time.Duration(dummyLength))

			var epg structs.Program
			poster := structs.Poster{}

			epg.Channel = xepgChannel.XMapping
			epg.Start = epgStartTime.Format("20060102150405") + offset
			epg.Stop = epgStopTime.Format("20060102150405") + offset

			// Create title with proper handling of non-ASCII characters
			var titleValue = xepgChannel.XName + " (" + epgStartTime.Weekday().String()[0:2] + ". " + epgStartTime.Format("15:04") + " - " + epgStopTime.Format("15:04") + ")"
			if !config.Settings.EnableNonAscii {
				titleValue = strings.TrimSpace(strings.Map(func(r rune) rune {
					if r > unicode.MaxASCII {
						return -1
					}
					return r
				}, titleValue))
			}
			epg.Title = append(epg.Title, &structs.Title{Value: titleValue, Lang: "en"})

			if len(xepgChannel.XDescription) == 0 {
				var descValue = "Threadfin: (" + strconv.Itoa(dummyLength) + " Minutes) " + epgStartTime.Weekday().String() + " " + epgStartTime.Format("15:04") + " - " + epgStopTime.Format("15:04")
				if !config.Settings.EnableNonAscii {
					descValue = strings.TrimSpace(strings.Map(func(r rune) rune {
						if r > unicode.MaxASCII {
							return -1
						}
						return r
					}, descValue))
				}
				epg.Desc = append(epg.Desc, &structs.Desc{Value: descValue, Lang: "en"})
			} else {
				var descValue = xepgChannel.XDescription
				if !config.Settings.EnableNonAscii {
					descValue = strings.TrimSpace(strings.Map(func(r rune) rune {
						if r > unicode.MaxASCII {
							return -1
						}
						return r
					}, descValue))
				}
				epg.Desc = append(epg.Desc, &structs.Desc{Value: descValue, Lang: "en"})
			}

			if config.Settings.XepgReplaceMissingImages {
				poster.Src = imgc.Image.GetURL(xepgChannel.TvgLogo, config.Settings.HttpThreadfinDomain, config.Settings.Port, config.Settings.ForceHttps, config.Settings.HttpsPort, config.Settings.HttpsThreadfinDomain)
				epg.Poster = append(epg.Poster, poster)
			}

			if xepgChannel.XCategory != "Movie" {
				epg.EpisodeNum = append(epg.EpisodeNum, &structs.EpisodeNum{Value: epgStartTime.Format("2006-01-02 15:04:05"), System: "original-air-date"})
			}

			epg.New = &structs.New{Value: ""}

			dummyXMLTV.Program = append(dummyXMLTV.Program, &epg)
			epgStartTime = epgStopTime

		}

	}

	return
}
