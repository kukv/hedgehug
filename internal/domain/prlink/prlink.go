// Package prlink は、GitHub の PR のブランチ名・タイトル・本文・コメントから、
// PR が指すアイテムの ID(WEB-1)と PR 用の印(hh:<ULID>)を読む(仕様 07「見る場所と形」)。
//
// ブランチ名は大文字と小文字を区別せず、先頭が hh/ か hedgehug/ のものだけを読む。
// タイトル・本文・コメントの ID は大文字の形だけを読み、前後が英数字か - のものは読まない。
// 印は前後が英数字のものを読まない。本文に印が 1 つでもあるとき、タイトルと本文の ID は読まない。
package prlink

import (
	"regexp"
	"strconv"
	"strings"
)

// Text は、PR の中で ID と印を探す文字列。Comments は古い順に 100 件まで。
type Text struct {
	Branch, Title, Body string
	Comments            []string
}

// Mentions は、PR が指すもの。IDs は「WEB-1」の形(大文字)、Marks は内部の ID(大文字)。
// 見つけた順で、重ならない。
type Mentions struct {
	IDs   []string
	Marks []string
}

var (
	branchRE = regexp.MustCompile(`(?i)^(?:hh|hedgehug)/([a-z][a-z0-9]{0,9}-[1-9][0-9]*)(?:-|$)`)
	idRE     = regexp.MustCompile(`[A-Z][A-Z0-9]{0,9}-[1-9][0-9]*`)
	markRE   = regexp.MustCompile(`(?:hedgehug|hh):([0-9A-Za-z]{26})`)
	splitRE  = regexp.MustCompile(`^([A-Z][A-Z0-9]{0,9})-([1-9][0-9]*)$`)
)

// Read は t から ID と印を読む。
func Read(t Text) Mentions {
	var m Mentions
	add := func(ids, marks []string) {
		m.IDs = appendNew(m.IDs, ids)
		m.Marks = appendNew(m.Marks, marks)
	}
	if b := branchRE.FindStringSubmatch(t.Branch); b != nil {
		add([]string{strings.ToUpper(b[1])}, nil)
	}
	marked := len(found(markRE, t.Body, isAlnum, 1)) > 0
	for _, s := range []string{t.Title, t.Body} {
		var ids []string
		if !marked {
			ids = found(idRE, s, isIDChar, 0)
		}
		add(ids, upper(found(markRE, s, isAlnum, 1)))
	}
	for _, c := range t.Comments {
		add(found(idRE, c, isIDChar, 0), upper(found(markRE, c, isAlnum, 1)))
	}
	return m
}

// SplitID は「WEB-1」を接頭辞と番号に分ける。
func SplitID(id string) (prefix string, number int, ok bool) {
	m := splitRE.FindStringSubmatch(id)
	if m == nil {
		return "", 0, false
	}
	n, err := strconv.Atoi(m[2])
	if err != nil {
		return "", 0, false
	}
	return m[1], n, true
}

// found は re に当たった group 番目の部分を順に返す。当たりの前後が neighbor の文字なら除く。
// RE2 には後読みがないので、当たった位置の前後の 1 文字を見て除く。
func found(re *regexp.Regexp, s string, neighbor func(byte) bool, group int) []string {
	var out []string
	for _, m := range re.FindAllStringSubmatchIndex(s, -1) {
		if m[0] > 0 && neighbor(s[m[0]-1]) || m[1] < len(s) && neighbor(s[m[1]]) {
			continue
		}
		out = append(out, s[m[2*group]:m[2*group+1]])
	}
	return out
}

func isAlnum(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}

func isIDChar(c byte) bool { return isAlnum(c) || c == '-' }

func upper(ss []string) []string {
	for i, s := range ss {
		ss[i] = strings.ToUpper(s)
	}
	return ss
}

func appendNew(dst, src []string) []string {
	for _, s := range src {
		seen := false
		for _, d := range dst {
			if d == s {
				seen = true
				break
			}
		}
		if !seen {
			dst = append(dst, s)
		}
	}
	return dst
}
