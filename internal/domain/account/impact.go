package account

import (
	"fmt"
	"strings"
)

// StoreUse は、アカウントを割り当てた Store(非表示・閲覧のみを含む)。削除・編集の確認に使う。
type StoreUse struct {
	Name   string
	Remote bool
}

// DeleteLines は、07「削除の影響」の確認の本文を、当てはまる部品だけ、この順に返す。
// uses は a を割り当てた Store、machineLogin はこのマシンの設定「ログイン名」(削除後の「あなた」)。
func DeleteLines(a Account, uses []StoreUse, machineLogin string) []string {
	lines := []string{"「" + a.Name + "」を削除します。"}
	var synced []string
	for _, u := range uses {
		if u.Remote {
			synced = append(synced, u.Name)
		}
	}
	if len(synced) > 0 {
		lines = append(lines, strings.Join(synced, "・")+" の同期に使っているため、それらは同期できなくなります。")
	}
	return append(lines, identityLines(a.Login, machineLogin, uses)...)
}

// EditLines は、07「編集」の「アカウントを変更」の確認の本文。「あなた」が変わる Store がなければ空。
func EditLines(a Account, newLogin string, uses []StoreUse) []string {
	return identityLines(a.Login, strings.TrimSpace(newLogin), uses)
}

func identityLines(from, to string, uses []StoreUse) []string {
	if strings.EqualFold(from, to) {
		return nil
	}
	lines := make([]string, 0, len(uses))
	for _, u := range uses {
		lines = append(lines, fmt.Sprintf("%s での あなた が @%s から %s に変わります。", u.Name, from, to))
	}
	return lines
}
