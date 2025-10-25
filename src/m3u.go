package src

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os/exec"
	"strings"

	"threadfin/src/internal/config"
	"threadfin/src/internal/structs"
)

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
