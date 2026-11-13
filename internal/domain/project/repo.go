package project

import (
	"regexp"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// repoPattern は、07 の owner/name の形(owner は英数字とハイフン、name は英数字・ピリオド・アンダースコア・ハイフン)。
var repoPattern = regexp.MustCompile(`^[A-Za-z0-9-]+/[A-Za-z0-9._-]+$`)

// NormalizeRepo は、前後の空白を除いて owner/name の形かを見る(02「リポジトリの追加と削除」)。
func NormalizeRepo(s string) (string, error) {
	r := strings.TrimSpace(s)
	if !repoPattern.MatchString(r) {
		return "", refusal.New("リポジトリは owner/name の形式で入力してください")
	}
	return r, nil
}

// IndexRepo は、repos の中で repo と大文字小文字を区別せずに同じものの位置を返す。なければ -1。
func IndexRepo(repos []string, repo string) int {
	return slices.IndexFunc(repos, func(r string) bool { return strings.EqualFold(r, repo) })
}
