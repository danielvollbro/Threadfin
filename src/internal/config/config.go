package config

import (
	"sync"
	"threadfin/src/internal/structs"

	"github.com/avfs/avfs"
)

// System : Beinhaltet alle Systeminformationen
var System structs.SystemStruct

// WebScreenLog : Logs werden im RAM gespeichert und für das Webinterface bereitgestellt
var WebScreenLog structs.WebScreenLogStruct

// Settings : Inhalt der settings.json
var Settings structs.SettingsStruct

// Data : Alle Daten werden hier abgelegt. (Lineup, XMLTV)
var Data structs.DataStruct

// SystemFiles : Alle Systemdateien
var SystemFiles = []string{"authentication.json", "pms.json", "settings.json", "xepg.json", "urls.json"}

// BufferInformation : Informationen über den Buffer (aktive Streams, maximale Streams)
var BufferInformation sync.Map

// bufferVFS : Filesystem to use for the Buffer
var BufferVFS avfs.VFS

// BufferClients : Anzahl der Clients die einen Stream über den Buffer abspielen
var BufferClients sync.Map

// Lock : Lock Map
var Lock = sync.RWMutex{}

var (
	XepgMutex   sync.Mutex
	InfoMutex   sync.Mutex
	LogMutex    sync.Mutex
	SystemMutex sync.Mutex
)
