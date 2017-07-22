package historiography

import (
	"math/rand"
	"time"
)

var src = rand.NewSource(time.Now().UnixNano())

const letterBytes = "abcdefghijklmnopqrstuvwxyz0123456789"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

// Create a random lower case alphanumerical string of a specified size.
//
// Implementation comes from:
// http://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-lenght-in-golang
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

// Works the same as rand.Intn but with an internal generated seed.
func Intn(n int) int {
	return int(src.Int63() % int64(n))
}

// Create a repartition function weighted according to params.
// For instance, if we want a repartition of 50%, 25%, 25% we call weighted this way:
//	Weighted(2, 1, 1) // same as Weighted(50, 25, 25)
//
// This will return 0, 50% of the calls, 1, 25% of the calls and 2, the last 25%.
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
