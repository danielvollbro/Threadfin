package xmltv

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"threadfin/internal/cli"
	"threadfin/internal/compression"
	"threadfin/internal/config"
	jsonserializer "threadfin/internal/json-serializer"
	"threadfin/internal/storage"
	"threadfin/internal/structs"
	"threadfin/internal/utilities"
)

// XMLTV Datei erstellen
func CreateFile() (err error) {

	// Image Cache
	// 4edd81ab7c368208cc6448b615051b37.jpg
	var imgc = config.Data.Cache.Images

	config.Data.Cache.ImagesFiles = []string{}
	config.Data.Cache.ImagesURLS = []string{}
	config.Data.Cache.ImagesCache = []string{}

	files, err := os.ReadDir(config.System.Folder.ImagesCache)
	if err == nil {

		for _, file := range files {

			if utilities.IndexOfString(file.Name(), config.Data.Cache.ImagesCache) == -1 {
				config.Data.Cache.ImagesCache = append(config.Data.Cache.ImagesCache, file.Name())
			}

		}

	}

	if len(config.Data.XMLTV.Files) == 0 && len(config.Data.Streams.Active) == 0 {
		config.Data.XEPG.Channels = make(map[string]interface{})
		return
	}

	cli.ShowInfo("XEPG:" + fmt.Sprintf("Create XMLTV file (%s)", config.System.File.XML))

	// Stream XML to disk to avoid huge memory usage
	xmlFile, err := os.Create(config.System.File.XML)
	if err != nil {
		return err
	}
	defer func() {
		err = xmlFile.Close()
	}()
	if err != nil {
		return err
	}

	// Use buffered writer for performance
	writer := bufio.NewWriterSize(xmlFile, 1<<20) // 1MB buffer
	defer func() {
		err = writer.Flush()
	}()
	if err != nil {
		return err
	}

	var xepgXML structs.XMLTV

	xepgXML.Generator = config.System.Name

	if config.System.Branch == "main" {
		xepgXML.Source = fmt.Sprintf("%s - %s", config.System.Name, config.System.Version)
	} else {
		xepgXML.Source = fmt.Sprintf("%s - %s.%s", config.System.Name, config.System.Version, config.System.Build)
	}

	var tmpProgram = &structs.XMLTV{}

	if _, err = writer.WriteString(xml.Header); err != nil {
		return err
	}
	if _, err = writer.WriteString("<tv>\n"); err != nil {
		return err
	}

	if _, err = fmt.Fprintf(writer, "  <generator>%s</generator>\n", xepgXML.Generator); err != nil {
		return err
	}
	if _, err = fmt.Fprintf(writer, "  <source>%s</source>\n", xepgXML.Source); err != nil {
		return err
	}

	type channelEntry struct {
		idx int
		ch  structs.XEPGChannelStruct
	}
	var entries []channelEntry

	for _, dxc := range config.Data.XEPG.Channels {
		var xepgChannel structs.XEPGChannelStruct
		err := json.Unmarshal([]byte(jsonserializer.MapToJSON(dxc)), &xepgChannel)
		if err == nil {
			entries = append(entries, channelEntry{idx: len(entries), ch: xepgChannel})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		chI := entries[i].ch.TvgChno
		chJ := entries[j].ch.TvgChno

		numI, errI := strconv.ParseFloat(chI, 64)
		numJ, errJ := strconv.ParseFloat(chJ, 64)

		if errI == nil && errJ == nil {
			return numI < numJ
		}

		if errI == nil && errJ != nil {
			return true
		}
		if errI != nil && errJ == nil {
			return false
		}

		return chI < chJ
	})

	for _, e := range entries {
		xepgChannel := e.ch
		if xepgChannel.TvgName == "" {
			xepgChannel.TvgName = xepgChannel.Name
		}
		if xepgChannel.XName == "" {
			xepgChannel.XName = xepgChannel.TvgName
		}

		if xepgChannel.XActive && !xepgChannel.XHideChannel {
			if (config.Settings.XepgReplaceChannelTitle && xepgChannel.XMapping == "PPV") || xepgChannel.XName != "" {
				channel := structs.Channel{
					ID: xepgChannel.XChannelID,
					Icon: structs.Icon{
						Src: imgc.Image.GetURL(xepgChannel.TvgLogo, config.Settings.HttpThreadfinDomain, config.Settings.Port, config.Settings.ForceHttps, config.Settings.HttpsPort, config.Settings.HttpsThreadfinDomain),
					},
					DisplayName: []structs.DisplayName{
						{Value: xepgChannel.XName},
					},
					Active: xepgChannel.XActive,
					Live:   xepgChannel.Live,
				}
				bytes, _ := xml.MarshalIndent(channel, "  ", "    ")
				if _, err = writer.Write(bytes); err != nil {
					return err
				}
				if _, err = writer.WriteString("\n"); err != nil {
					return err
				}
			}
		}
	}

	for _, e := range entries {
		xepgChannel := e.ch
		if xepgChannel.XActive && !xepgChannel.XHideChannel {
			*tmpProgram, err = GetData(xepgChannel)
			if err == nil {
				for _, p := range tmpProgram.Program {
					bytes, _ := xml.MarshalIndent(p, "  ", "    ")
					if _, err = writer.Write(bytes); err != nil {
						return err
					}
					if _, err = writer.WriteString("\n"); err != nil {
						return err
					}
				}
			} else {
				cli.ShowDebug("XEPG:"+fmt.Sprintf("Error: %s", err), 3)
			}
		}
	}

	// Close tv root
	if _, err = writer.WriteString("</tv>\n"); err != nil {
		return err
	}

	cli.ShowInfo("XEPG:" + fmt.Sprintf("Compress XMLTV file (%s)", config.System.Compressed.GZxml))
	if err = compression.CompressGZIPFile(config.System.File.XML, config.System.Compressed.GZxml); err != nil {
		return err
	}

	return
}

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
