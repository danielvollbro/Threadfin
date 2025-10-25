package xmltv

import (
	"strings"
	"threadfin/src/internal/structs"
)

// Kategorien erweitern (createXMLTVFile)
func getCategory(program *structs.Program, xmltvProgram *structs.Program, xepgChannel structs.XEPGChannelStruct, filters []structs.FilterStruct) {

	for _, i := range xmltvProgram.Category {

		category := &structs.Category{}
		category.Value = i.Value
		category.Lang = i.Lang
		program.Category = append(program.Category, category)

	}

	if len(xepgChannel.XCategory) > 0 {

		category := &structs.Category{}
		category.Value = strings.ToLower(xepgChannel.XCategory)
		category.Lang = "en"
		program.Category = append(program.Category, category)

	}
}
