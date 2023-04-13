package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

func main() {
	ls := strings.Join(left[:], "|")
	rs := strings.Join(right[:], "|")
	re := regexp.MustCompile(fmt.Sprintf("\\b(%s)_(%s)\\b", ls, rs))
	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {
		t := re.FindString(s.Text())
		if t != "" {
			fmt.Println(t)
		}
	}
}
