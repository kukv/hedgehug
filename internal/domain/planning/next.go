package planning

// NextSprint は、sprint:next が指すスプリント。計画中(開始も完了も削除もされていない)のうち開始日がいちばん早いもの。
// 開始日のないものは最後で、同じなら ScreenOrder の順。なければ nil。
func NextSprint(live []Sprint) *Sprint {
	var planned []Sprint
	for _, s := range live {
		if s.State == Planned && s.Deleted == nil {
			planned = append(planned, s)
		}
	}
	if len(planned) == 0 {
		return nil
	}
	next := ScreenOrder(planned)[0]
	return &next
}
