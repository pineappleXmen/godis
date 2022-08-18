package util

import "time"

type monotime = int64

func GetMonotonicUs() int64 {
	return time.Now().Unix()
}
