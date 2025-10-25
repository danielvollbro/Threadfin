package utilities

import (
	"crypto/rand"
	"log"
	"time"
)

func CreateUUID() (uuid string) {
	uuid = time.Now().Format("2006-01") + "-" + RandomString(4) + "-" + RandomString(6)
	return
}

// Sonstiges
func RandomString(n int) string {

	const alphanum = "AB1CD2EF3GH4IJ5KL6MN7OP8QR9ST0UVWXYZ"

	var bytes = make([]byte, n)

	_, err := rand.Read(bytes)
	if err != nil {
		log.Fatal(err)
		return ""
	}

	for i, b := range bytes {
		bytes[i] = alphanum[b%byte(len(alphanum))]
	}

	return string(bytes)
}
