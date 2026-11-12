package account

import (
	"slices"
	"testing"
)

func TestDeleteLines(t *testing.T) {
	a := Account{ID: "A1", Name: "work", Login: "taro", Method: GhCLI}
	uses := []StoreUse{{Name: "チーム", Remote: true}, {Name: "個人", Remote: false}, {Name: "OSS", Remote: true}}
	got := DeleteLines(a, uses, "hanako")
	want := []string{
		"「work」を削除します。",
		"チーム・OSS の同期に使っているため、それらは同期できなくなります。",
		"チーム での あなた が @taro から hanako に変わります。",
		"個人 での あなた が @taro から hanako に変わります。",
		"OSS での あなた が @taro から hanako に変わります。",
	}
	if !slices.Equal(got, want) {
		t.Errorf("DeleteLines =\n%q\nwant\n%q", got, want)
	}
}

func TestDeleteLinesOnlyWhatApplies(t *testing.T) {
	a := Account{ID: "A1", Name: "work", Login: "taro", Method: GhCLI}
	// remote がなく、マシンのログイン名が同じ人(大文字と小文字は区別しない)なら、1 行目だけ。
	got := DeleteLines(a, []StoreUse{{Name: "個人"}}, "Taro")
	if !slices.Equal(got, []string{"「work」を削除します。"}) {
		t.Errorf("DeleteLines = %q", got)
	}
}

func TestDeleteLinesEmptyMachineLogin(t *testing.T) {
	a := Account{ID: "A1", Name: "work", Login: "taro", Method: GhCLI}
	uses := []StoreUse{{Name: "チーム", Remote: true}, {Name: "個人", Remote: false}, {Name: "OSS", Remote: true}}
	got := DeleteLines(a, uses, "")
	want := []string{
		"「work」を削除します。",
		"チーム・OSS の同期に使っているため、それらは同期できなくなります。",
	}
	if !slices.Equal(got, want) {
		t.Errorf("DeleteLines = %q, want %q", got, want)
	}
}

func TestEditLines(t *testing.T) {
	a := Account{ID: "A1", Name: "work", Login: "taro", Method: GhCLI}
	uses := []StoreUse{{Name: "チーム", Remote: true}, {Name: "個人"}}
	want := []string{
		"チーム での あなた が @taro から jiro に変わります。",
		"個人 での あなた が @taro から jiro に変わります。",
	}
	if got := EditLines(a, " jiro ", uses); !slices.Equal(got, want) {
		t.Errorf("EditLines = %q", got)
	}
	if got := EditLines(a, "TARO", uses); len(got) != 0 {
		t.Errorf("same login: EditLines = %q", got)
	}
	if got := EditLines(a, "  ", uses); len(got) != 0 {
		t.Errorf("blank login: EditLines = %q", got)
	}
}
