package src

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os/exec"
	"regexp"
	"strings"

	"threadfin/src/internal/config"
	"threadfin/src/internal/structs"
)

// Streams filtern
func filterThisStream(s interface{}) (status bool, liveEvent bool) {
	var stream = s.(map[string]string)
	var regexpYES = `[{]+[^.]+[}]`
	var regexpNO = `!+[{]+[^.]+[}]`

	liveEvent = false

	for _, filter := range config.Data.Filter {

		if filter.Rule == "" {
			continue
		}

		liveEvent = filter.LiveEvent

		var group, name, search string
		var exclude, include string
		var match = false

		var streamValues = strings.ReplaceAll(stream["_values"], "\r", "")

		if v, ok := stream["group-title"]; ok {
			group = v
		}

		if v, ok := stream["name"]; ok {
			name = v
		}

		// Unerwünschte Streams !{DEU}
		r := regexp.MustCompile(regexpNO)
		val := r.FindStringSubmatch(filter.Rule)

		if len(val) == 1 {

			exclude = val[0][2 : len(val[0])-1]
			filter.Rule = strings.ReplaceAll(filter.Rule, " "+val[0], "")
			filter.Rule = strings.ReplaceAll(filter.Rule, val[0], "")

		}

		// Muss zusätzlich erfüllt sein {DEU}
		r = regexp.MustCompile(regexpYES)
		val = r.FindStringSubmatch(filter.Rule)

		if len(val) == 1 {

			include = val[0][1 : len(val[0])-1]
			filter.Rule = strings.ReplaceAll(filter.Rule, " "+val[0], "")
			filter.Rule = strings.ReplaceAll(filter.Rule, val[0], "")

		}

		switch filter.CaseSensitive {

		case false:

			streamValues = strings.ToLower(streamValues)
			filter.Rule = strings.ToLower(filter.Rule)
			exclude = strings.ToLower(exclude)
			include = strings.ToLower(include)
			group = strings.ToLower(group)
			name = strings.ToLower(name)

		}

		switch filter.Type {

		case "group-title":
			search = name

			if group == filter.Rule {
				match = true
			}

		case "custom-filter":
			search = streamValues
			if strings.Contains(search, filter.Rule) {
				match = true
			}
		}

		if match {

			if len(exclude) > 0 {
				var status = checkConditions(search, exclude, "exclude")
				if !status {
					return false, liveEvent
				}
			}

			if len(include) > 0 {
				var status = checkConditions(search, include, "include")
				if !status {
					return false, liveEvent
				}
			}

			return true, liveEvent

		}

	}

	return false, liveEvent
}

// Bedingungen für den Filter
func checkConditions(streamValues, conditions, coType string) (status bool) {

	switch coType {

	case "exclude":
		status = true

	case "include":
		status = false

	}

	conditions = strings.ReplaceAll(conditions, ", ", ",")
	conditions = strings.ReplaceAll(conditions, " ,", ",")

	var keys = strings.Split(conditions, ",")

	for _, key := range keys {

		if strings.Contains(streamValues, key) {

			switch coType {

			case "exclude":
				return false

			case "include":
				return true

			}

		}

	}

	return
}

func probeChannel(request structs.RequestStruct) (string, string, string, error) {

	ffmpegPath := config.Settings.FFmpegPath
	ffprobePath := strings.Replace(ffmpegPath, "ffmpeg", "ffprobe", 1)

	cmd := exec.Command(ffprobePath, "-v", "error", "-show_streams", "-of", "json", request.ProbeURL)
	output, err := cmd.Output()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to execute ffprobe: %v", err)
	}

	var ffprobeOutput structs.FFProbeOutput
	err = json.Unmarshal(output, &ffprobeOutput)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to parse ffprobe output: %v", err)
	}

	var resolution, frameRate, audioChannels string

	for _, stream := range ffprobeOutput.Streams {
		if stream.CodecType == "video" {
			resolution = fmt.Sprintf("%dp", stream.Height)
			frameRateParts := strings.Split(stream.RFrameRate, "/")
			if len(frameRateParts) == 2 {
				frameRate = fmt.Sprintf("%d", parseFrameRate(frameRateParts))
			} else {
				frameRate = stream.RFrameRate
			}
		}
		if stream.CodecType == "audio" {
			audioChannels = stream.ChannelLayout
			if audioChannels == "" {
				switch stream.Channels {
				case 1:
					audioChannels = "Mono"
				case 2:
					audioChannels = "Stereo"
				case 6:
					audioChannels = "5.1"
				case 8:
					audioChannels = "7.1"
				default:
					audioChannels = fmt.Sprintf("%d channels", stream.Channels)
				}
			}
		}
	}

	return resolution, frameRate, audioChannels, nil
}

func parseFrameRate(parts []string) int {
	numerator, denom := 1, 1
	_, err := fmt.Sscanf(parts[0], "%d", &numerator)
	if err != nil {
		log.Println("Error parsing frame rate numerator:", err)
		return 0
	}

	_, err = fmt.Sscanf(parts[1], "%d", &denom)
	if err != nil {
		log.Println("Error parsing frame rate denominator:", err)
		return 0
	}

	if denom == 0 {
		return 0
	}
	return int(math.Round(float64(numerator) / float64(denom)))
}
