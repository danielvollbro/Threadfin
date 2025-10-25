package playlist

import "threadfin/src/internal/config"

func GetActiveCount() (count int) {
	count = 0
	config.BufferInformation.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}
