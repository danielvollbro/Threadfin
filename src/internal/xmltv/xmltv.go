package xmltv

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/config"
	"threadfin/src/internal/storage"
	"threadfin/src/internal/structs"
)

// Load local provider XMLTV file
func GetLocal(file string, xmltvStruct *structs.XMLTV) (err error) {

	if _, ok := config.Data.Cache.XMLTV[file]; !ok {

		// Initialize cache
		if len(config.Data.Cache.XMLTV) == 0 {
			config.Data.Cache.XMLTV = make(map[string]structs.XMLTV)
		}

		// Check file size to determine parsing strategy
		fileInfo, err := os.Stat(file)
		if err != nil {
			err = errors.New("local copy of the file no longer exists")
			return err
		}

		// For large files (>50MB), use streaming parser
		if fileInfo.Size() > 50*1024*1024 {
			cli.ShowInfo("XEPG:" + "Using streaming parser for large XMLTV file: " + file)
			err = parseStream(file, xmltvStruct)
		} else {
			// Use original method for smaller files
			content, err := storage.ReadByteFromFile(file)
			if err != nil {
				err = errors.New("local copy of the file no longer exists")
				return err
			}

			// Parse XML file
			err = xml.Unmarshal(content, &xmltvStruct)
			if err != nil {
				return err
			}
		}

		if err != nil {
			return err
		}

		config.Data.Cache.XMLTV[file] = *xmltvStruct

	} else {
		*xmltvStruct = config.Data.Cache.XMLTV[file]
	}

	return
}

// parseStream : Streaming XML parser for large XMLTV files
func parseStream(file string, xmltv *structs.XMLTV) error {
	xmlFile, err := os.Open(file)
	if err != nil {
		return err
	}
	defer func() {
		err = xmlFile.Close()
	}()
	if err != nil {
		return err
	}

	decoder := xml.NewDecoder(xmlFile)

	xmltv.Channel = make([]*structs.Channel, 0)
	xmltv.Program = make([]*structs.Program, 0)

	var currentElement string
	var channelCount, programCount int

	for {
		token, err := decoder.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return err
		}

		switch se := token.(type) {
		case xml.StartElement:
			currentElement = se.Name.Local

			switch currentElement {
			case "tv":
				for _, attr := range se.Attr {
					switch attr.Name.Local {
					case "generator-info-name":
						xmltv.Generator = attr.Value
					case "source-info-name":
						xmltv.Source = attr.Value
					}
				}

			case "channel":
				var channel structs.Channel
				if err := decoder.DecodeElement(&channel, &se); err != nil {
					cli.ShowDebug("XMLTV Stream:Error parsing channel: "+err.Error(), 2)
					continue
				}
				xmltv.Channel = append(xmltv.Channel, &channel)
				channelCount++

				if channelCount%1000 == 0 {
					cli.ShowInfo(fmt.Sprintf("XMLTV Stream:Parsed %d channels", channelCount))
				}

			case "programme":
				var program structs.Program
				if err := decoder.DecodeElement(&program, &se); err != nil {
					cli.ShowDebug("XMLTV Stream:Error parsing program: "+err.Error(), 3)
					continue
				}
				xmltv.Program = append(xmltv.Program, &program)
				programCount++

				if programCount%10000 == 0 {
					cli.ShowInfo(fmt.Sprintf("XMLTV Stream:Parsed %d programs", programCount))
				}
			}
		}
	}

	cli.ShowInfo(fmt.Sprintf("XMLTV Stream:Completed - %d channels, %d programs", channelCount, programCount))
	return nil
}
