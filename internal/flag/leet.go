package flag

import (
	"crypto/sha256"
	"fmt"
)

var defaultLeet = map[byte][]string{
	'a': {"4", "A", "@"}, 'A': {"4", "a", "@"},
	'b': {"8", "B", "6"}, 'B': {"8", "b", "6"},
	'c': {"(", "C", "<"}, 'C': {"(", "c", "<"},
	'e': {"3", "E"},      'E': {"3", "e"},
	'g': {"9", "G"},      'G': {"9", "g"},
	'h': {"4", "H", "#"}, 'H': {"4", "h", "#"},
	'i': {"1", "I", "!"}, 'I': {"1", "i", "!"},
	'l': {"1", "L", "7"}, 'L': {"1", "l", "7"},
	'o': {"0", "O"},      'O': {"0", "o"},
	's': {"5", "S", "$"}, 'S': {"5", "s", "$"},
	't': {"7", "T", "+"}, 'T': {"7", "t", "+"},
	'z': {"2", "Z"},      'Z': {"2", "z"},
}

func Seed(userID int, challengeID string) []byte {
	h := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", userID, challengeID)))
	return h[:]
}

func Generate(baseFlag string, seed []byte, overrides map[byte][]string) string {
	leet := defaultLeet
	if overrides != nil {
		leet = make(map[byte][]string, len(defaultLeet)+len(overrides))
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
			r := sb % 4
			if r == 0 {
				out = append(out, c)
			} else {
				leetOpts, ok := leet[c]
				if ok {
					idx := (int(sb) / 4) % len(leetOpts)
					out = append(out, leetOpts[idx]...)
				} else if c >= 'a' && c <= 'z' {
					out = append(out, c-32)
				} else {
					out = append(out, c+32)
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
