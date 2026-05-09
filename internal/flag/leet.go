package flag

import (
	"crypto/sha256"
	"fmt"
)

var defaultLeet = map[byte]string{
	'a': "4", 'A': "4",
	'b': "8", 'B': "8",
	'e': "3", 'E': "3",
	'g': "9", 'G': "9",
	'i': "1", 'I': "1",
	'l': "1", 'L': "1",
	'o': "0", 'O': "0",
	's': "5", 'S': "5",
	't': "7", 'T': "7",
	'z': "2", 'Z': "2",
}

func Seed(userID int, challengeID string) []byte {
	h := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", userID, challengeID)))
	return h[:]
}

func Generate(baseFlag string, seed []byte, overrides map[byte]string) string {
	leet := defaultLeet
	if overrides != nil {
		leet = make(map[byte]string, len(defaultLeet)+len(overrides))
		for k, v := range defaultLeet {
			leet[k] = v
		}
		for k, v := range overrides {
			leet[k] = v
		}
	}

	out := make([]byte, 0, len(baseFlag))
	for i := 0; i < len(baseFlag); i++ {
		c := baseFlag[i]
		if isLetter(c) {
			sb := seed[i%len(seed)]
			r := sb % 3
			if r == 0 {
				out = append(out, c)
			} else {
				leetStr, ok := leet[c]
				if !ok {
					out = append(out, c)
				} else {
					out = append(out, leetStr...)
				}
			}
		} else {
			out = append(out, c)
		}
	}
	return string(out)
}

func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
