# コードベースの構成見直し（概要）

> 6 回に分ける（2026-11-04 に利用者が決定。回 6 の最終確認は同日に利用者が追加。回 5 は 2026-11-05 に 5a・5b の 2 つの PR に分けた）。この文書は 6 回に共通の決まりと回の分け方。Task の本文は各回の plan に書く: `2026-11-04-restructure1-move.md`（移動と改名）、`2026-11-04-restructure2-osfs.md`（OS の境界）、`2026-11-04-restructure3-usecase.md`（usecase の再編）、`2026-11-04-restructure4-entry.md`（入口の境界）、`2026-11-04-restructure5-tests.md`（テストと仕上げ）、`2026-11-04-restructure6-review.md`（最終確認）。各回は前の回のマージの後に main から切り、その時点のコードを読んで plan を書く（前の回でファイルの場所と行が変わるため。フェーズ 4e と同じ進め方）。

**Goal:** spec のとおり、パッケージを lib・domain・storage・remote・usecase・cli/server の層に並べ、層の向きを lint で強制する。

**Architecture:** 最初の回で全パッケージを最終的なディレクトリへ機械的に移し、以降の回はその配置の上で中身を分け直す。挙動は回 4 の `--json` の形の他は変えない。

**Tech Stack:** Go、golangci-lint 2.15.0（depguard・forbidigo）、GitHub Actions。

**Spec:** `docs/superpowers/specs/2026-11-04-codebase-restructure-design.md`

## Global Constraints

- 挙動を変えない。変えてよいのは、回 4 で CLI の `--json` を server と同じ usecase の型（包みを含む）にすることで変わる形（spec 4.3。2026-11-04 に利用者が「包みまで server に揃える」と決定）と、回 5b の壊れた Store のエラー（spec 決定 19。2026-11-05 利用者が決定）だけ
- Windows 版の実装を書かない（spec 決定 8）
- テストの中身の変更は、import・型の名前・パスの書き換えだけにする。例外は回 4 の `--json` の形と cli・server のテストの書き直し（spec 決定 11。確かめる内容は変えない）と、回 3 の領域分け（呼び方の書き換え、外部テスト化、構造体の中を触る 4 か所。回 3 の plan「この回で決めたこと」）と、回 5 のテストの配り直し（spec 5 節）と、回 5b の状態の作り方の書き換え（spec 決定 18）
- 各回で `go build ./...`・`go vet ./...`・`make lint`・`make fmt-check` を通す。全テスト（`make test`）は各回の仕上げに 1 回だけ回し、細かい直しは `go vet` だけで push して CI に任せる
- テストが落ちたら、失敗の出力と実行条件（コマンド、`-race` の有無、shard）を PR のコメントか作業記録に残す
- spec と違う挙動が必要になったら、止めて利用者に聞く
- 必須チェックの名前（`test`）を変えない。ruleset と必須チェックは Terraform 管理なので `gh api` で直接変えない
- コミットは利用者が席にいるときだけ（1Password の署名）。離席中は index に積んでおく

## 回の分け方と完了条件

| 回 | 中身（spec の節） | 完了条件 |
| --- | --- | --- |
| 1 移動と改名 | 2.2 の行き先への移動（mount と context は `usecase/mount`・`usecase/current` に仮置き）、名前の変更、F（4.1）、CI の shard と Makefile のパス | `internal/` 直下が `lib/` と `app/` だけになる。`cmd/hedgehug` は main.go だけ。全テストと CI が通る |
| 2 OS の境界 | 3 節、github の `Options` 化 | `lib/atomicfile` がなくなり `lib/osfs` に統合される。forbidigo が有効で通る。github が app を import しない |
| 3 usecase の再編 | 2.2 の mount と context の解体、途中残りの検出を storage/store へ、2.3 の領域分け、領域の操作を関数にする（決定 13）、usecase のテストを領域へ移す（決定 14） | `usecase/mount`・`usecase/current` がなくなる。ops の旧ファイルが 2.3 の 11 領域と土台に分かれる。storage/mount が replicate を import しない（2026-11-04 完了。領域の操作は関数、テストは外部テストパッケージ、補助は usecasetest と mounttest） |
| 4 入口の境界 | 4.2〜4.4、Poller の中身を usecase/watch へ（決定 15）、cli・server のテストの書き直し（決定 11） | cli と server が、テストを含めて storage・remote・lib/gitrepo・lib/github を import しない（例外は決定 12 のテスト用の部品）。cli・server の depguard のルールが有効で通る。`cli/prompt` の 2 つの部品で 7 つの確認が書かれている。形が変わる `--json`（spec 4.3）が新しい形を期待するテストで固定されている。server/poll.go の中身が usecase/watch にある（2026-11-05 完了） |
| 5a テストと仕上げ | 5 節（差し込み口を除く）、depguard の残りのルール（決定 17 を含む）、shard の見直し、親設計（8 節） | depguard が全ルールで有効で通る。`final_*_test.go` がなくなる。`cli/scenario` ができ、組み立てたバイナリを動かすテストだけがある（決定 16）。history のベンチが usecase を import しない。親設計の「パッケージ構成」節が 2.1 と一致する |
| 5b テストのための差し込み口 | 決定 18・19（2026-11-05 追加） | 回 5 の plan の「差し込み口の一覧」の A・B・C がすべて直り、D だけが残る。本物の手段に書き直したテストが、実装をわざと壊すと落ちることを確かめてある。壊れた Store のエラー（決定 19）が、再現するテストとともに直っている |
| 6 最終確認 | spec 6 節の PR 6。回 5 のマージ後の main で層別構成の Artifact を作り直し、spec と突き合わせる | Artifact が回 5 の後の `go list` の結果で更新されている。2.1 の配置とのずれ、層の向きに反する依存、4.2 の禁止に反する依存の一覧（どれも 0 件のはず）を添えて利用者に見せ、利用者が最終判断を出している。直すものが出たら、この回で直してマージしている。tools/restructure を消している（2026-11-05 実施。結果と決定は `2026-11-04-restructure6-review.md`。領域どうしの依存 1 件と OS の境界の外の呼び出し 56 か所を直した。spec 決定 20・21） |
