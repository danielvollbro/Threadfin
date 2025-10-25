package plex

import (
	"fmt"
	"threadfin/src/internal/config"
)

// Eindeutige Geräte ID für Plex generieren
func SetDeviceID() {
	var id = config.Settings.UUID

	switch config.Settings.Tuner {
	case 1:
		config.System.DeviceID = id

	default:
		config.System.DeviceID = fmt.Sprintf("%s:%d", id, config.Settings.Tuner)
	}
}
