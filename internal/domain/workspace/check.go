package workspace

import (
	"strings"

	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// NormalizeName は、保存する名前の形(前後の空白だけを除く。途中の空白は残す。03「名前の重複」)。
func NormalizeName(s string) string { return strings.TrimSpace(s) }

// NormalizePrefix は、接頭辞の形(前後の空白を除いて大文字。03「接頭辞」)。
func NormalizePrefix(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

// NameTaken は、w 以外に同じ名前のワークスペースがあるか(完全一致。大文字と小文字は区別する。03「名前の重複」)。
func NameTaken(w Workspace, others []Workspace) bool {
	for _, o := range others {
		if o.ID != w.ID && o.Name == w.Name {
			return true
		}
	}
	return false
}

// PrefixTaken は、w 以外に同じ接頭辞のワークスペースがあるか。
func PrefixTaken(w Workspace, others []Workspace) bool {
	for _, o := range others {
		if o.ID != w.ID && o.Prefix == w.Prefix {
			return true
		}
	}
	return false
}

// CheckRename は、名前の変更の検査(03「名前」)。w の名前は NormalizeName を通した形、others は同じ Store のワークスペース。
func CheckRename(w Workspace, others []Workspace) error {
	switch {
	case w.Name == "":
		return refusal.New("名前は空にできません")
	case NameTaken(w, others):
		return refusal.New("この Store には同じ名前のワークスペースがあります")
	}
	return nil
}

// CheckPrefix は、接頭辞の変更の検査(03「接頭辞」。作成と同じ検査と文言)。
// w の接頭辞は NormalizePrefix を通した形、others は同じ Store のワークスペース。
func CheckPrefix(w Workspace, others []Workspace) error {
	switch {
	case w.Prefix == "":
		return refusal.New("接頭辞を入力してください")
	case !ValidPrefix(w.Prefix):
		return refusal.New("接頭辞は英大文字で始まる 10 文字以内の英大文字と数字にしてください")
	case PrefixTaken(w, others):
		return refusal.New("この Store には同じ接頭辞のワークスペースがあります")
	}
	return nil
}

// CheckCreate は、ワークスペースの作成の検査(03「ワークスペースの作成」の表の順)。
// w の名前と接頭辞は NormalizeName・NormalizePrefix を通した形、others は同じ Store のワークスペース。
func CheckCreate(w Workspace, others []Workspace) error {
	switch {
	case w.Name == "":
		return refusal.New("名前を入力してください")
	case NameTaken(w, others):
		return refusal.New("この Store には同じ名前のワークスペースがあります")
	case w.Prefix == "":
		return refusal.New("接頭辞を入力してください")
	case !ValidPrefix(w.Prefix):
		return refusal.New("接頭辞は英大文字で始まる 10 文字以内の英大文字と数字にしてください")
	case PrefixTaken(w, others):
		return refusal.New("この Store には同じ接頭辞のワークスペースがあります")
	}
	return nil
}
