package jsonserializer

import "encoding/json"

func MapToJSON(tmpMap interface{}) string {

	jsonString, err := json.MarshalIndent(tmpMap, "", "  ")
	if err != nil {
		return "{}"
	}

	return string(jsonString)
}

func JSONToInterface(content string) (tmpMap interface{}, err error) {
	err = json.Unmarshal([]byte(content), &tmpMap)
	return
}
