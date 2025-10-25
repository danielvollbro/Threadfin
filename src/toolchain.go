package src

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"threadfin/src/internal/cli"
)

func parseTemplate(content string, tmpMap map[string]interface{}) (result string) {

	t := template.Must(template.New("template").Parse(content))

	var tpl bytes.Buffer

	if err := t.Execute(&tpl, tmpMap); err != nil {
		cli.ShowError(err, 0)
	}
	result = tpl.String()

	return
}

func getBaseUrl(host string, port string) string {
	if strings.Contains(host, ":") {
		return host
	} else {
		return fmt.Sprintf("%s:%s", host, port)
	}
}
