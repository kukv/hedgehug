# フェーズ 5b の引き継ぎ（2026-11-05 時点）

5b（自動化と開発の開始）は plan の Task 1〜16 を実装した。CLI だけで `hh start` から PR のマージまでを、手動のステータス変更なしで回せる（`internal/app/cli/scenario/github_test.go`）。

## plan から変えた点

決定 21〜26 は、実装とレビューの中で足した。plan の表に理由を書いた。ほかに次を変えた。

1. 取り込み（`Fetch`・`FetchRepo`・`HasTargets`、自動リンク、ルールの実行）を `usecase/integration` から土台の `usecase/github_*.go` へ下ろした。replication が integration を import していて、depguard の deny の漏れで隠れていた。構成見直しの spec の「領域から別の領域を呼ぶ必要が出たら、その関数を土台に下ろす」に従った
2. `hh unlink` は同じコミットで `gh` の要素も外す。5a は `links` だけを変えていて、spec の記述と違っていた
3. `hh link` は新規と `off` からの再リンクで `gh` の同じ ref の要素を消す。`hh undo` でリンクが消えても、貼り直すと `pr.opened` が発火する
4. `store.Op.SkipRequired` を足した。使うのは `hh undo` だけ
5. `model/node.go` の `idKey` に `id` を足し、`Put` が YAML 1.1 の真偽値の語のキー（`on` など）を引用符なしで書く
6. `hh mv --store` の GUI の見出しを「プロジェクトの設定（表示と自動化）」にした。拒否の間も持っていくかの選択肢を描く

## 5c 以降が使うもの

- `automation.File.IssueFields`（`DefaultSync`・`Push`・`Mirror`・`Props`）。`props` の既定は空
- `usecase.Fetch` のルールの実行は `github_rules.go`。5c の `mirror` の反映は取り込みのトランザクションの中で評価し、`issue-closed` のルールはその後に当てる（spec）
- `automation.TrailerRejected` は `mirror` の拒否でも同じ形で使う
- web の inbox は `automation`・`automation_rejected` を既定の分岐で出している。タブと文言は 5d-2
- ルールの `set` は、活動では通常の op として出る。「自動化」の絞り込みは 5d-1
- GUI のライブ表示は、ルールの `set` を「CLI で更新」と出す（`web/core/cache.ts` の `SYNC_OPS`）。分類は 5d-1 で直す

## 未実施のこと

5a の引き継ぎの「未実施のこと」（検証用 Org での手の確認と、`github-live.yaml` の secret・PR）は、5b でも実施していない。利用者が席にいるときに行う。`hh pr` の実 API（PR の作成、`default_branch`）も、そのときに確かめる。
