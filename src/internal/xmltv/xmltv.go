package xmltv

import (
	"encoding/xml"
	"fmt"
	"os"
	"threadfin/src/internal/cli"
	"threadfin/src/internal/structs"
)

// ParseStream : Streaming XML parser for large XMLTV files
func ParseStream(file string, xmltv *structs.XMLTV) error {
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
