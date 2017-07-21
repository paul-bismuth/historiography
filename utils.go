package historiography

import (
	"math/rand"
	"time"
)

// http://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-lenght-in-golang
var src = rand.NewSource(time.Now().UnixNano())

const letterBytes = "abcdefghijklmnopqrstuvwxyz0123456789"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func SecureRandomString(n int) string {
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}

func Intn(n int) int {
	return int(src.Int63() % int64(n))
}

func Weighted(weights ...int) func() int {
	repartition := []int{}
	for i, weight := range weights {
		for j := 0; j < weight; j++ {
			repartition = append(repartition, i)
		}
	}
	limit := int64(len(repartition))
	return func() int {
		return repartition[int(src.Int63()%limit)]
	}
}
