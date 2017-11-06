package eater

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"
)

var (
	tags = regexp.MustCompile("(<[^>]*>)")
)

// ExtractJSONString tries to find a js assignment to json in a haystack of
// javascript and html.
func ExtractJSONString(haystack, needle string) string {

	// There is probably a better way than this, but, I'm too lazy to write
	// a nice parser. This shit is definitely going to break if html tags are
	// in js strings.
	lines := tags.ReplaceAllString(haystack, "\n$1\n")
	scanner := bufio.NewScanner(bytes.NewBufferString(lines))
	scanner.Buffer([]byte{}, 512*1024) // yes, 512kB
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), needle) {
			e := strings.Index(scanner.Text(), "=")
			return scanner.Text()[e+1:]
		}
	}
	return ""
}
