package channels

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/config"
	jsonserializer "threadfin/src/internal/json-serializer"
	"threadfin/src/internal/storage"
	"threadfin/src/internal/structs"
)

// Remove duplicate channels from XEPG database using consistent hash logic
func RemoveDuplicateChannels() {
	cli.ShowInfo("XEPG:" + "Remove duplicate channels")

	// Track channels by hash to identify exact duplicates (same URL + name + source)
	hashToChannelID := make(map[string]string)
	var channelsToRemove []string
	var duplicatesFound int

	for id, dxc := range config.Data.XEPG.Channels {
		var xepgChannel structs.XEPGChannelStruct
		err := json.Unmarshal([]byte(jsonserializer.MapToJSON(dxc)), &xepgChannel)
		if err != nil {
			continue
		}

		if xepgChannel.TvgName == "" {
			xepgChannel.TvgName = xepgChannel.Name
		}

		// Create consistent channel hash using URL as primary identifier
		// Use TvgID when available, since names can change but IDs should remain stable
		var hashInput string
		if xepgChannel.TvgID != "" {
			// Use TvgID when available for stable identification
			hashInput = xepgChannel.URL + xepgChannel.TvgID + xepgChannel.FileM3UID
		} else {
			// Fall back to URL + FileM3UID only when TvgID is blank
			hashInput = xepgChannel.URL + xepgChannel.FileM3UID
		}
		hash := md5.Sum([]byte(hashInput))
		channelHash := hex.EncodeToString(hash[:])

		// Check for hash-based duplicates (exact same content)
		if existingChannelID, exists := hashToChannelID[channelHash]; exists {
			channelsToRemove = append(channelsToRemove, handleDuplicate(id, existingChannelID, "hash"))
			duplicatesFound++
		} else {
			hashToChannelID[channelHash] = id
		}

		// DISABLED: Name-based duplicate removal - backup channels with different URLs are legitimate
		// Only remove true duplicates (exact same URL + tvg-id + source) via hash-based detection
		//
		// Note: Channels like "NFL RedZone (1)", "NFL RedZone (2)", "NFL RedZone (3)" with same tvg-id
		// but different URLs are backup channels and should be preserved, not treated as duplicates
	}

	// Remove duplicate channels
	for _, channelID := range channelsToRemove {
		delete(config.Data.XEPG.Channels, channelID)
	}

	if duplicatesFound > 0 {
		cli.ShowInfo(fmt.Sprintf("XEPG:Removed %d duplicate channels", duplicatesFound))
		// Save the cleaned database
		err := storage.SaveMapToJSONFile(config.System.File.XEPG, config.Data.XEPG.Channels)
		if err != nil {
			cli.ShowError(err, 000)
		}
	} else {
		cli.ShowInfo("XEPG:No duplicate channels found")
	}
}

// Helper function to handle duplicate channel removal
func handleDuplicate(currentID, existingID, duplicateType string) string {
	currentChannel := getByID(currentID)
	existingChannel := getByID(existingID)

	if currentChannel == nil || existingChannel == nil {
		return currentID // Default to removing current
	}

	// Prefer active channels over inactive ones
	if currentChannel.XActive && !existingChannel.XActive {
		cli.ShowInfo(fmt.Sprintf("XEPG:Removing %s duplicate %s (%s), keeping %s (%s)",
			duplicateType, existingID, existingChannel.XName, currentID, currentChannel.XName))
		return existingID
	} else if !currentChannel.XActive && existingChannel.XActive {
		cli.ShowInfo(fmt.Sprintf("XEPG:Removing %s duplicate %s (%s), keeping %s (%s)",
			duplicateType, currentID, currentChannel.XName, existingID, existingChannel.XName))
		return currentID
	}

	// Both have same active status, prefer one with XMLTV mapping
	currentHasMapping := currentChannel.XmltvFile != "" && currentChannel.XmltvFile != "-"
	existingHasMapping := existingChannel.XmltvFile != "" && existingChannel.XmltvFile != "-"

	if currentHasMapping && !existingHasMapping {
		cli.ShowInfo(fmt.Sprintf("XEPG:Removing %s duplicate %s (%s), keeping %s (%s)",
			duplicateType, existingID, existingChannel.XName, currentID, currentChannel.XName))
		return existingID
	} else if !currentHasMapping && existingHasMapping {
		cli.ShowInfo(fmt.Sprintf("XEPG:Removing %s duplicate %s (%s), keeping %s (%s)",
			duplicateType, currentID, currentChannel.XName, existingID, existingChannel.XName))
		return currentID
	}

	// Prefer the one created later (larger XEPG ID suggests later creation)
	if currentID > existingID {
		cli.ShowInfo(fmt.Sprintf("XEPG:Removing %s duplicate %s (%s), keeping %s (%s)",
			duplicateType, existingID, existingChannel.XName, currentID, currentChannel.XName))
		return existingID
	} else {
		cli.ShowInfo(fmt.Sprintf("XEPG:Removing %s duplicate %s (%s), keeping %s (%s)",
			duplicateType, currentID, currentChannel.XName, existingID, existingChannel.XName))
		return currentID
	}
}

// Helper function to get channel by ID
func getByID(id string) *structs.XEPGChannelStruct {
	if dxc, exists := config.Data.XEPG.Channels[id]; exists {
		var channel structs.XEPGChannelStruct
		if err := json.Unmarshal([]byte(jsonserializer.MapToJSON(dxc)), &channel); err == nil {
			return &channel
		}
	}
	return nil
}
