package utilities

import (
	"fmt"
	"strings"
)

func GetBaseUrl(host string, port string) string {
	if strings.Contains(host, ":") {
		return host
	} else {
		return fmt.Sprintf("%s:%s", host, port)
	}
}
