package programs

import (
	"fmt"
	"regexp"
	"strings"
	"threadfin/src/internal/config"
	"threadfin/src/internal/structs"
	"time"
	"unicode"
)

func CreateLiveProgram(xepgChannel structs.XEPGChannelStruct, channelId string) []*structs.Program {
	var programs []*structs.Program

	var currentTime = time.Now()
	localLocation := currentTime.Location() // Central Time (CT)

	startTime := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 0, 0, 0, currentTime.Nanosecond(), localLocation)
	stopTime := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 23, 59, 59, currentTime.Nanosecond(), localLocation)

	name := ""
	if xepgChannel.XName != "" {
		name = xepgChannel.XName
	} else {
		name = xepgChannel.TvgName
	}

	// Search for Datetime or Time
	// Datetime examples: '12/31-11:59 PM', '1.1 6:30 AM', '09/15-10:00PM', '7/4 12:00 PM', '3.21 3:45 AM', '6/30-8:00 AM', '4/15 3AM'
	// Time examples: '11:59 PM', '6:30 AM', '11:59PM', '1PM'
	re := regexp.MustCompile(`((\d{1,2}[./]\d{1,2})[-\s])*(\d{1,2}(:\d{2})*\s*(AM|PM)?(?:\s*(ET|CT|MT|PT|EST|CST|MST|PST))?)`)
	matches := re.FindStringSubmatch(name)
	if len(matches) > 0 {
		timePart := matches[len(matches)-2]
		if timePart == "" {
			timePart = matches[len(matches)-1]
		}

		timeString := strings.TrimSpace(timePart)
		timeString = strings.ReplaceAll(timeString, "  ", " ")

		// Handle timezone if present
		var location *time.Location
		if strings.Contains(timeString, "ET") || strings.Contains(timeString, "EST") {
			location, _ = time.LoadLocation("America/New_York")
		} else if strings.Contains(timeString, "CT") || strings.Contains(timeString, "CST") {
			location, _ = time.LoadLocation("America/Chicago")
		} else if strings.Contains(timeString, "MT") || strings.Contains(timeString, "MST") {
			location, _ = time.LoadLocation("America/Denver")
		} else if strings.Contains(timeString, "PT") || strings.Contains(timeString, "PST") {
			location, _ = time.LoadLocation("America/Los_Angeles")
		} else {
			location = currentTime.Location()
		}

		// Remove timezone from timeString
		timeString = strings.ReplaceAll(timeString, "ET", "")
		timeString = strings.ReplaceAll(timeString, "CT", "")
		timeString = strings.ReplaceAll(timeString, "MT", "")
		timeString = strings.ReplaceAll(timeString, "PT", "")
		timeString = strings.ReplaceAll(timeString, "EST", "")
		timeString = strings.ReplaceAll(timeString, "CST", "")
		timeString = strings.ReplaceAll(timeString, "MST", "")
		timeString = strings.ReplaceAll(timeString, "PST", "")
		timeString = strings.TrimSpace(timeString)

		// Handle different date formats
		var datePart string
		if len(matches) > 3 && matches[2] != "" {
			datePart = matches[2]
			// Convert slashes to dots for consistency
			datePart = strings.ReplaceAll(datePart, "/", ".")
		}

		// Build the full time string
		var fullTimeString string
		if datePart != "" {
			// If we have a date part, use it
			parts := strings.Split(datePart, ".")
			if len(parts) == 2 {
				month := parts[0]
				day := parts[1]
				fullTimeString = fmt.Sprintf("%d.%s.%s %s", currentTime.Year(), month, day, timeString)
			}
		} else {
			// If no date part, use current date
			fullTimeString = fmt.Sprintf("%d.%d.%d %s", currentTime.Year(), currentTime.Month(), currentTime.Day(), timeString)
		}

		// Determine layout based on time format
		var layout string
		if strings.Contains(timeString, ":") {
			if strings.Contains(timeString, "AM") || strings.Contains(timeString, "PM") {
				layout = "2006.1.2 3:04 PM"
			} else {
				layout = "2006.1.2 15:04"
			}
		} else {
			if strings.Contains(timeString, "AM") || strings.Contains(timeString, "PM") {
				layout = "2006.1.2 3PM"
			} else {
				layout = "2006.1.2 15"
			}
		}

		startTimeParsed, err := time.ParseInLocation(layout, fullTimeString, location)
		if err != nil {
			startTime = time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 6, 0, 0, 0, location)
		} else {
			localTime := startTimeParsed.In(localLocation)
			startTime = localTime
		}
	}

	// Add "CHANNEL OFFLINE" program for the time before the event
	beginningOfDay := time.Date(startTime.Year(), startTime.Month(), startTime.Day(), 0, 0, 0, 0, localLocation)

	// Handle non-ASCII characters in offline text
	var offlineText = "CHANNEL OFFLINE"
	if !config.Settings.EnableNonAscii {
		offlineText = strings.TrimSpace(strings.Map(func(r rune) rune {
			if r > unicode.MaxASCII {
				return -1
			}
			return r
		}, offlineText))
	}

	programBefore := &structs.Program{
		Channel: channelId,
		Start:   beginningOfDay.Format("20060102150405 -0700"),
		Stop:    startTime.Format("20060102150405 -0700"),
		Title:   []*structs.Title{{Lang: "en", Value: offlineText}},
		Desc:    []*structs.Desc{{Lang: "en", Value: offlineText}},
	}
	programs = append(programs, programBefore)

	// Add the main program
	mainProgram := &structs.Program{
		Channel: channelId,
		Start:   startTime.Format("20060102150405 -0700"),
		Stop:    stopTime.Format("20060102150405 -0700"),
	}

	if config.Settings.XepgReplaceChannelTitle && xepgChannel.XMapping == "PPV" {
		title := []*structs.Title{}
		title_parsed := fmt.Sprintf("%s %s", name, xepgChannel.XPpvExtra)

		// Handle non-ASCII characters in title
		if !config.Settings.EnableNonAscii {
			title_parsed = strings.TrimSpace(strings.Map(func(r rune) rune {
				if r > unicode.MaxASCII {
					return -1
				}
				return r
			}, title_parsed))
		}

		t := &structs.Title{Lang: "en", Value: title_parsed}
		title = append(title, t)
		mainProgram.Title = title

		desc := []*structs.Desc{}
		d := &structs.Desc{Lang: "en", Value: title_parsed}
		desc = append(desc, d)
		mainProgram.Desc = desc
	}
	programs = append(programs, mainProgram)

	// Add "CHANNEL OFFLINE" program for the time after the event
	midnightNextDayStart := time.Date(stopTime.Year(), stopTime.Month(), stopTime.Day()+1, 0, 0, 0, currentTime.Nanosecond(), localLocation)
	midnightNextDayStop := time.Date(stopTime.Year(), stopTime.Month(), stopTime.Day()+1, 23, 59, 59, currentTime.Nanosecond(), localLocation)
	programAfter := &structs.Program{
		Channel: channelId,
		Start:   midnightNextDayStart.Format("20060102150405 -0700"),
		Stop:    midnightNextDayStop.Format("20060102150405 -0700"),
		Title:   []*structs.Title{{Lang: "en", Value: offlineText}},
		Desc:    []*structs.Desc{{Lang: "en", Value: offlineText}},
	}
	programs = append(programs, programAfter)

	return programs
}
