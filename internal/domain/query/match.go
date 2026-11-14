package query

// Match は、すべての語に一致すれば Match を真にする。語の結果は Not で反転する。
func Match(q Query, f Facts, today string) Hit {
	var hit Hit
	for _, t := range q.Terms {
		var ok bool
		if t.Key == "" {
			var old bool
			ok, old = freeWord(t.Values[0], f)
			hit.OldID = hit.OldID || (ok && old && !t.Not)
		} else {
			ok = matchKey(t, f, today)
		}
		if t.Not {
			ok = !ok
		}
		if !ok {
			return Hit{}
		}
	}
	hit.Match = true
	return hit
}

// matchKey は、キーのある語が(Not 前で)一致するか。値は `,` で OR。
func matchKey(t Term, f Facts, today string) bool {
	if r, ok := matchNames(t, f); ok {
		return r
	}
	return matchRest(t, f, today)
}

// matchRest は、Task 3 が作るキー(数・日付・スプリント・リリース・is・has・mentions・追加プロパティ)の入り口。
// いまは一致しない。
func matchRest(Term, Facts, string) bool {
	return false
}
