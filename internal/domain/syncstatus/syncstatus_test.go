package syncstatus_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// 06「同期のエラー」の表の「短い理由」と、ミラーの段落の短い理由。
func TestShortIsTheSpecWording(t *testing.T) {
	cases := []struct {
		r    syncstatus.Reason
		want string
	}{
		{syncstatus.NoAccount, "アカウントが設定されていません"},
		{syncstatus.AuthFailed, "認証に失敗しました"},
		{syncstatus.Unreachable, "remote に接続できません"},
		{syncstatus.WriteDenied, "remote への書き込みが許可されていません"},
		{syncstatus.Unreadable, "remote を読めません。ご利用アカウントに付与されているリポジトリ権限をご確認ください。"},
		{syncstatus.NotStore, "remote が hedgehug の Store ではありません"},
		{syncstatus.NewerFormat, "hedgehug を更新してください"},
		{syncstatus.Unrelated, "remote の履歴がこの Store と関係がありません"},
		{syncstatus.Rewritten, "remote の履歴が書き換えられています"},
		{syncstatus.SendConflict, "送信が他の人の更新と重なりました。もう一度同期してください"},
		{syncstatus.MirrorUnrelated, "ミラーの履歴がこの Store と関係がありません"},
		{syncstatus.MirrorAhead, "ミラーの履歴が先に進んでいます"},
	}
	for _, c := range cases {
		if got := c.r.Short(); got != c.want {
			t.Errorf("%s: Short = %q, want %q", c.r, got, c.want)
		}
		if !c.r.Valid() {
			t.Errorf("%s: not Valid", c.r)
		}
	}
	if syncstatus.Reason("x").Valid() || syncstatus.Reason("x").Short() != "" || syncstatus.Reason("").Valid() {
		t.Error("unknown reason is valid")
	}
}

// 06「同期のエラー」の「手動で同期したときの表示」。
func TestFailed(t *testing.T) {
	if got := syncstatus.Failed("team", syncstatus.NoAccount); got != "team にアカウントが設定されていないため同期できません" {
		t.Errorf("no account: %q", got)
	}
	if got := syncstatus.Failed("team", syncstatus.AuthFailed); got != "team の同期に失敗しました: 認証に失敗しました" {
		t.Errorf("auth: %q", got)
	}
	if got := syncstatus.Failed("team", syncstatus.SendConflict); got != "team の同期に失敗しました: 送信が他の人の更新と重なりました。もう一度同期してください" {
		t.Errorf("conflict: %q", got)
	}
}

// 06「同期の結果」と「手動で同期する」「同期のエラー」の、失敗でない文言。
func TestMessages(t *testing.T) {
	if got := syncstatus.NoRemote("solo"); got != "solo には remote がないため、同期するものはありません" {
		t.Errorf("NoRemote: %q", got)
	}
	if got := syncstatus.Synced("team", 2, 3); got != "team を同期しました(送信 2 件・受信 3 件)" {
		t.Errorf("Synced: %q", got)
	}
	if got := syncstatus.MirrorsFailed(2); got != "ミラー 2 件に送れませんでした" {
		t.Errorf("MirrorsFailed: %q", got)
	}
	if syncstatus.NothingToSync != "同期する Store がありません(remote のある、表示中の Store が要ります)" {
		t.Errorf("NothingToSync: %q", syncstatus.NothingToSync)
	}
}

// 06「同期の結果」の「すべて同期」のトースト。
func TestAll(t *testing.T) {
	if got := syncstatus.All(3, nil); !slices.Equal(got, []string{"3 件の Store を同期しました"}) {
		t.Errorf("all ok: %q", got)
	}
	got := syncstatus.All(1, []syncstatus.Failure{
		{Store: "team", Reason: syncstatus.NoAccount},
		{Store: "web", Reason: syncstatus.Unreachable},
	})
	want := []string{"成功 1 件・失敗 2 件", "team: アカウントが設定されていません", "web: remote に接続できません"}
	if !slices.Equal(got, want) {
		t.Errorf("mixed: %q, want %q", got, want)
	}
}
