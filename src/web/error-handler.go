package web

import (
	"fmt"
	"net/http"
)

func HttpStatusError(w http.ResponseWriter, httpStatusCode int) {
	http.Error(w, fmt.Sprintf("%s [%d]", http.StatusText(httpStatusCode), httpStatusCode), httpStatusCode)
}
