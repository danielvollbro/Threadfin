package dvr

import (
	"encoding/json"
	"fmt"
	"threadfin/internal/config"
	jsonserializer "threadfin/internal/json-serializer"
	"threadfin/internal/structs"
)

// Filterregeln erstellen
func createFilterRules() (err error) {

	config.Data.Filter = nil
	var dataFilter structs.Filter

	for _, f := range config.Settings.Filter {

		var filter structs.FilterStruct

		var exclude, include string

		err = json.Unmarshal([]byte(jsonserializer.MapToJSON(f)), &filter)
		if err != nil {
			return
		}

		switch filter.Type {

		case "custom-filter":
			dataFilter.CaseSensitive = filter.CaseSensitive
			dataFilter.Rule = filter.Filter
			dataFilter.Type = filter.Type

			config.Data.Filter = append(config.Data.Filter, dataFilter)

		case "group-title":
			if len(filter.Include) > 0 {
				include = fmt.Sprintf(" {%s}", filter.Include)
			}

			if len(filter.Exclude) > 0 {
				exclude = fmt.Sprintf(" !{%s}", filter.Exclude)
			}

			dataFilter.CaseSensitive = filter.CaseSensitive
			dataFilter.LiveEvent = filter.LiveEvent
			dataFilter.Rule = fmt.Sprintf("%s%s%s", filter.Filter, include, exclude)
			dataFilter.Type = filter.Type

			config.Data.Filter = append(config.Data.Filter, dataFilter)
		}

	}

	return
}
