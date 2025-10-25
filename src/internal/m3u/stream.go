package m3u

import (
	"regexp"
	"strings"
	"threadfin/internal/config"
)

// Streams filtern
func FilterThisStream(s interface{}) (status bool, liveEvent bool) {
	var stream = s.(map[string]string)
	var regexpYES = `[{]+[^.]+[}]`
	var regexpNO = `!+[{]+[^.]+[}]`

	liveEvent = false

	for _, filter := range config.Data.Filter {

		if filter.Rule == "" {
			continue
		}

		liveEvent = filter.LiveEvent

		var group, name, search string
		var exclude, include string
		var match = false

		var streamValues = strings.ReplaceAll(stream["_values"], "\r", "")

		if v, ok := stream["group-title"]; ok {
			group = v
		}

		if v, ok := stream["name"]; ok {
			name = v
		}

		// Unerwünschte Streams !{DEU}
		r := regexp.MustCompile(regexpNO)
		val := r.FindStringSubmatch(filter.Rule)

		if len(val) == 1 {

			exclude = val[0][2 : len(val[0])-1]
			filter.Rule = strings.ReplaceAll(filter.Rule, " "+val[0], "")
			filter.Rule = strings.ReplaceAll(filter.Rule, val[0], "")

		}

		// Muss zusätzlich erfüllt sein {DEU}
		r = regexp.MustCompile(regexpYES)
		val = r.FindStringSubmatch(filter.Rule)

		if len(val) == 1 {

			include = val[0][1 : len(val[0])-1]
			filter.Rule = strings.ReplaceAll(filter.Rule, " "+val[0], "")
			filter.Rule = strings.ReplaceAll(filter.Rule, val[0], "")

		}

		switch filter.CaseSensitive {

		case false:

			streamValues = strings.ToLower(streamValues)
			filter.Rule = strings.ToLower(filter.Rule)
			exclude = strings.ToLower(exclude)
			include = strings.ToLower(include)
			group = strings.ToLower(group)
			name = strings.ToLower(name)

		}

		switch filter.Type {

		case "group-title":
			search = name

			if group == filter.Rule {
				match = true
			}

		case "custom-filter":
			search = streamValues
			if strings.Contains(search, filter.Rule) {
				match = true
			}
		}

		if match {

			if len(exclude) > 0 {
				var status = CheckConditions(search, exclude, "exclude")
				if !status {
					return false, liveEvent
				}
			}

			if len(include) > 0 {
				var status = CheckConditions(search, include, "include")
				if !status {
					return false, liveEvent
				}
			}

			return true, liveEvent

		}

	}

	return false, liveEvent
}
