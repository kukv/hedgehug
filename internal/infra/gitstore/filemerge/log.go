// Package filemerge は、Store のファイルを 2 つの版から合わせる規則を持つ(保存形式「同期」の手順 3)。
// 入出力をしない。どちらの版を a・b に渡しても同じ結果になる。
package filemerge

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"time"
)

type row struct {
	id   string
	at   time.Time
	line []byte
}

// MergeLog は、log.jsonl・transfers.jsonl・purged.jsonl の 2 つの版の行を、id で重ねずに合わせ、
// at(UTC)・id の順に並べる。形の違う行(id か at が読めない行)は残さない。
// 同じ id で中身が違う行は、バイト列として小さいほうを残す。
func MergeLog(a, b []byte) []byte {
	rows := map[string]row{}
	for _, src := range [][]byte{a, b} {
		for _, line := range bytes.Split(src, []byte("\n")) {
			r, ok := parseRow(line)
			if !ok {
				continue
			}
			if old, seen := rows[r.id]; !seen || bytes.Compare(r.line, old.line) < 0 {
				rows[r.id] = r
			}
		}
	}
	list := make([]row, 0, len(rows))
	for _, r := range rows {
		list = append(list, r)
	}
	slices.SortFunc(list, func(x, y row) int {
		if c := x.at.Compare(y.at); c != 0 {
			return c
		}
		return strings.Compare(x.id, y.id)
	})
	var out bytes.Buffer
	for _, r := range list {
		out.Write(r.line)
		out.WriteByte('\n')
	}
	return out.Bytes()
}

func parseRow(line []byte) (row, bool) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return row{}, false
	}
	var v struct {
		ID string `json:"id"`
		At string `json:"at"`
	}
	if json.Unmarshal(line, &v) != nil || v.ID == "" {
		return row{}, false
	}
	at, err := time.Parse(time.RFC3339, v.At)
	if err != nil {
		return row{}, false
	}
	return row{id: v.ID, at: at, line: line}, true
}
