package archtest

import (
	"bytes"
	"regexp"
)

// 設計書 4 節の上限。
const (
	maxFileLines     = 300
	maxTestFileLines = 600
	maxPackageFiles  = 20
)

var generatedMark = regexp.MustCompile(`(?m)^// Code generated .* DO NOT EDIT\.$`)

func isGenerated(src []byte) bool { return generatedMark.Match(src) }

func lineCount(src []byte) int {
	n := bytes.Count(src, []byte("\n"))
	if len(src) > 0 && src[len(src)-1] != '\n' {
		n++
	}
	return n
}
