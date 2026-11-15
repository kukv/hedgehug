package rank

// Moved は、並び ranks の i 番目を 1 つ上(up)か下へ動かしたときの値。新しい隣どうしのあいだの値で、端の外は空。
// 呼ぶ側は、ranks が厳密に増えていること(Refill のあと)と、動かせる位置(端でない)であることを守る。
func Moved(ranks []string, i int, up bool) string {
	if up {
		return Between(at(ranks, i-2), ranks[i-1])
	}
	return Between(ranks[i+1], at(ranks, i+2))
}

// at は ranks[i]。範囲の外なら空(端)。
func at(ranks []string, i int) string {
	if i < 0 || i >= len(ranks) {
		return ""
	}
	return ranks[i]
}

// Refill は、空か、前に残るもの以下(同じ値を含む)の値だけを、表示の順を保つ値に振り直した並びを返す。
// changed は振り直した位置。空でなく厳密に増えている値は変えない。振り直す値は、前の値と、次に残るものの
// あいだの値にする。ranks は書き換えない。
func Refill(ranks []string) (out []string, changed []bool) {
	keep := kept(ranks)
	out = make([]string, len(ranks))
	changed = make([]bool, len(ranks))
	prev := ""
	for i, r := range ranks {
		if keep[i] {
			out[i] = r
		} else {
			out[i], changed[i] = Between(prev, nextKept(ranks, keep, i)), true
		}
		prev = out[i]
	}
	return out, changed
}

// kept は、値を変えないもの(空でなく、前に残るものより大きい)を true にして返す。
func kept(ranks []string) []bool {
	keep := make([]bool, len(ranks))
	prev := ""
	for i, r := range ranks {
		if r != "" && r > prev {
			keep[i], prev = true, r
		}
	}
	return keep
}

// nextKept は、ranks[i] より後ろで最初に残るものの値。なければ空。
func nextKept(ranks []string, keep []bool, i int) string {
	for j := i + 1; j < len(ranks); j++ {
		if keep[j] {
			return ranks[j]
		}
	}
	return ""
}
