package imgcache

import (
	"crypto/md5"
	"encoding/hex"
	"threadfin/internal/utilities"
)

func strToMD5(str string) string {
	md5Hasher := md5.New()
	md5Hasher.Write([]byte(str))
	return hex.EncodeToString(md5Hasher.Sum(nil))
}

func removeStringFromSlice(str string, slice []string) []string {

	var i = utilities.IndexOfString(str, slice)

	if i != -1 {
		slice = append(slice[:i], slice[i+1:]...)
	}

	return slice
}
