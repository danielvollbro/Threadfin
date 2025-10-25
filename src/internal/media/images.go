package media

import (
	b64 "encoding/base64"
	"fmt"
	"strings"
	"threadfin/src/internal/config"
	"threadfin/src/internal/storage"
)

func UploadLogo(input, filename string) (logoURL string, err error) {
	b64data := input[strings.IndexByte(input, ',')+1:]

	// BAse64 in bytes umwandeln un speichern
	sDec, err := b64.StdEncoding.DecodeString(b64data)
	if err != nil {
		return
	}

	var file = fmt.Sprintf("%s%s", config.System.Folder.ImagesUpload, filename)

	err = storage.WriteByteToFile(file, sDec)
	if err != nil {
		return
	}

	// Respect Force HTTPS setting when generating logo URL
	if config.Settings.ForceHttps && config.Settings.HttpsThreadfinDomain != "" {
		logoURL = fmt.Sprintf("https://%s:%d/data_images/%s", config.Settings.HttpsThreadfinDomain, config.Settings.HttpsPort, filename)
	} else if config.Settings.HttpThreadfinDomain != "" {
		logoURL = fmt.Sprintf("http://%s:%s/data_images/%s", config.Settings.HttpThreadfinDomain, config.Settings.Port, filename)
	} else {
		logoURL = fmt.Sprintf("%s://%s/data_images/%s", config.System.ServerProtocol.XML, config.System.Domain, filename)
	}

	return

}
