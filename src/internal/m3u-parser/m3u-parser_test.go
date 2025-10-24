package m3u

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"testing"
)

type M3UStream struct {
	GroupTitle string `json:"group-title"`
	Name       string `json:"name"`
	TvgID      string `json:"tvg-id"`
	TvgLogo    string `json:"tvg-logo"`
	TvgName    string `json:"tvg-name"`
	URL        string `json:"url"`
	UUIDKey    string `json:"_uuid.key,omitempty"`
	UUIDValue  string `json:"_uuid.value,omitempty"`
}

func TestStream1(t *testing.T) {

	var file = "test_list_1.m3u"
	var content, err = os.ReadFile(file)
	if err != nil {
		t.Error(err)
		return
	}

	streams, err := MakeInterfaceFromM3U(content)

	if err != nil {
		t.Error(err)
	}

	err = checkStream(streams)
	if err != nil {
		t.Error(err)
	}

	fmt.Println("Streams:", len(streams))
	t.Log(streams)

}

func checkStream(streamInterface []interface{}) (err error) {

	for i, s := range streamInterface {

		var stream = s.(map[string]string)
		var m3uStream M3UStream

		jsonString, err := json.MarshalIndent(stream, "", "  ")

		if err == nil {

			err = json.Unmarshal(jsonString, &m3uStream)
			if err == nil {

				log.Printf("Stream:        %d\n", i)
				log.Printf("Name*:         %s\n", m3uStream.Name)
				log.Printf("URL*:          %s\n", m3uStream.URL)
				log.Printf("tvg-name:      %s\n", m3uStream.TvgName)
				log.Printf("tvg-id**:      %s\n", m3uStream.TvgID)
				log.Printf("tvg-logo:      %s\n", m3uStream.TvgLogo)
				log.Printf("group-title**: %s\n", m3uStream.GroupTitle)

				if len(m3uStream.UUIDKey) > 0 {
					log.Printf("UUID key***:   %s\n", m3uStream.UUIDKey)
					log.Printf("UUID value:    %s\n", m3uStream.UUIDValue)
				} else {
					log.Println("UUID key:    false")
				}

			}

		}

		log.Println("- - - - - (*: Required) | (**: Nice to have) | (***: Love it) - - - - -")
	}

	return
}
