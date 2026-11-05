# 構成見直し 回 6（最終確認）

> 概要 plan: `2026-11-04-codebase-restructure.md`。spec: `docs/superpowers/specs/2026-11-04-codebase-restructure-design.md` の 6 節の PR 6。ブランチ `restructure6-final`（main 170d061 から）。

**Goal:** 回 5 の後の main を spec と突き合わせ、利用者の最終判断で出た直しをこの回で行う。tools/restructure を消す。

## 突き合わせの結果（2026-11-05、main 170d061）

層別構成の Artifact（https://claude.ai/artifact/WFTVztg3Wvhgj4UnCwzsiH）を `go list` の結果で作り直した（v3）。

| 項目 | 結果 |
| --- | --- |
| 2.1 の配置 | 一覧にあるものはすべてある。`storage/mount/mounttest` と `usecase/usecasetest` が一覧にない（決定 12・14 にはある） |
| 層の向き（決定 2） | 本体・テストとも 0 件 |
| 4.2 の禁止 | 本体・テストとも 0 件 |
| 領域どうし（2.3・決定 17） | 1 件。`replication/sync.go` が `integration.Fetch`・`HasTargets` を呼ぶ。フェーズ 5a（#61）が integration を足したとき、depguard の `usecase-domains` の一覧に入らず、lint は通っていた |
| OS の境界（3 節） | forbidigo の名前は 0 件。一覧にない `os.Open`・`CreateTemp`・`MkdirTemp`・`Mkdir`・`Chmod` が osfs の外に 15 か所 |

## 利用者の決定（2026-11-05）

1. 取り込みは spec 2.3 のとおり土台に下ろす（replication だけを例外にする案は採らない）。integration に残るのはリンクの付け外し。フェーズ 5b で同期から呼ぶもの（Issue 同期、ルールの実行）も土台に置く
2. 15 か所の OS の呼び出しは osfs に寄せ、forbidigo に 5 つの名前を足す

## Task 1: 取り込みを土台へ

- `integration/fetch.go`・`apply.go`・`autolink.go` を `usecase/github_fetch.go`・`github_apply.go`・`github_autolink.go` へ移す。公開の名前は土台の中で意味が通るように `Fetch` → `FetchGitHub`、`FetchReport` → `GitHubFetchReport`、`HasTargets` → `HasGitHubTargets` にする。非公開の名前は変えない（ぶつかったら build が教える）
- テストは試している操作の場所へ: `apply_test.go` → `usecase/github_apply_test.go`（package usecase）。`autolink_test.go` → `usecase/github_autolink_test.go`（package usecase_test、`AutoLinks` は usecase の export_test.go へ）。`fetch_test.go`・`fetch_repeat_test.go`・`review_fixes_test.go` の Fetch のテスト → `usecase/github_fetch_test.go` ほか。`replication.Sync`・`SyncStore` を試すテスト（`TestSync*`、`fetch_targets_test.go`）→ `replication` のテスト。`TestLinkViewCarriesMergedAtAndStateReason` は integration に残す
- 確かめ方: `go build ./... && go vet ./...`、`go test ./internal/app/usecase/... `（テストの件数が移す前と同じ）、`go list -deps` で replication が integration を import しない

## Task 2: depguard

- `usecase-domains` の deny を `github.com/kukv/hedgehug/internal/app/usecase/` の前方一致 1 行にする（一覧をやめ、5b の automations も足さずに止まる）。土台（末尾の `/` がない）は止まらない
- 確かめ方: Task 1 の前の main でこの設定にすると replication の 1 件だけが出る（確認済み）。Task 1 の後は `make lint` が 0 件

## Task 3: OS の境界

- osfs の `files_unix.go` に `Open`・`CreateTemp`・`MkdirTemp`・`Mkdir`・`Chmod` を足す（os の同名を呼ぶだけ）
- 15 か所を置き換える: gitrepo（maintain.go 93・250・272・300・320・326、pack.go 46・62、objects.go 248、refs.go 42、conn.go 97）、store/lock.go 77、config/config.go 652、cli/editor.go 43、tools/devenv/main.go 331
- forbidigo の 1 つ目の pattern に 5 つの名前を足す。spec 3 節の禁止の一覧も同じにする。`Stat`・`Lstat`・`ReadDir` は OS で振る舞いが違わない読み取りなので対象外のまま（spec 3 節に書く）
- 確かめ方: `make lint` が 0 件。置き換えを 1 か所戻すと forbidigo が止める

## Task 4: 仕上げ

- `tools/restructure` を消す
- spec: 決定 20（取り込みを土台へ）・21（OS の名前の追加）、2.1 に mounttest・usecasetest、2.3 の土台と integration の行、3 節。フェーズ 5 の設計の表（`usecase/integration` の行と土台）、5a の handoff の `integration.Fetch`・`integration.apply` の名前。概要 plan の回 6 の完了
- 親設計の「パッケージ構成」に integration の中身の記述があれば直す
- `go build ./...`・`go vet ./...`・`make lint`・`make fmt-check`・`make tidy-check`・`make test`（1 回）

## 結果（2026-11-05）

- Task 1〜4 を実施。usecase 系のテストは移す前後で 669 件（usecase 155→181、integration 40→7、replication 17→24）。共通の補助は `usecasetest.GitHubSession` にした
- forbidigo は、置き換えた 1 か所を `os.Mkdir` に戻すと止めることを確かめた。置き換えで要らなくなった `//nolint:gosec`（G304・G703）を外した
- 親設計の「パッケージ構成」に、フェーズ 5a で増えた `storage/ghstate`・`usecase/integration` と、土台の GitHub の取り込みを足した（rev 376）
- `go build`・`go vet`・`make lint`（0 件）・`make fmt-check`・`make tidy-check`・`go test -race ./...`（全 44 パッケージ通過）
