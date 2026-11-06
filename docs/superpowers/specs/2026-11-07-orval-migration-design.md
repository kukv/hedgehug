# API クライアントを Orval へ移す 設計

- 日付: 2026-11-07
- 置き換える部分: `docs/superpowers/specs/2026-11-01-phase4-gui-design.md` の、openapi-typescript と openapi-fetch を使う記述

## 目的

openapi-typescript が保守を終え、更新もセキュリティ修正も出なくなった。同じリポジトリの openapi-fetch も更新が止まっている。型の生成とクライアントを Orval に移す。

TypeScript は 6 のままにする。svelte-check の peer が 6 までなので、7 へ上げるのはこの移行に含めない。

## 候補の比較（2026-11-07）

実際に `api/openapi.yaml` から生成して比べた。

| 観点 | Orval 8.46.0 | Hey API 0.99.0 |
|---|---|---|
| 8/1 以降の `main` のコミット | 100 件以上、約 30 人 | 10 件、うち 7 件が 1 人 |
| `main` の最終コミット | 11/6 | 9/30 |
| TS 7 で生成 | 動く | 落ちる。直った版はプレリリースだけ |
| 既存の型との一致 | 166 中 165 | 163 型は一致、3 型は改名 |

機能の差ではなく保守の状況で Orval を選んだ。移行の理由が保守の終了なので、開発の止まりかけた候補には移らない。

## 決定の一覧

| # | 対象 | 決定 |
|---|---|---|
| 1 | 生成物 | `web/core/api/generated.ts`。`pnpm gen` で作り、CI が古さを検出する |
| 2 | 通信 | 全操作の 1 回の要求は `web/core/api/fetcher.ts` の `fetcher` が担う。トークン、401 `auth.token`、`ApiError` への変換をここに集める |
| 3 | 呼び出しの形 | `api.moveTask(id, body, query)`。引数はパスパラメーター、本文、クエリー、オプションの順 |
| 4 | 失敗 | 操作は成功なら本体を返し、失敗なら `ApiError` を投げる。`unwrap` はなくす |
| 5 | インスタンス | `createApi` が全操作に接続先を束縛する。モジュールの大域変数は使わない |
| 6 | スキーマの型 | `Schemas.TaskView` の形で参照する |

決定 5 の理由: Orval が生成する関数はモジュールの関数で、接続先を持てない。テストは `fakeApi` で別々の `fetch` を持つインスタンスを作るので、大域の設定では干渉する。全呼び出しに設定を渡す形は、渡し忘れを型で防げない。生成された操作はオプションを最後の引数に取るので、`createApi` がその位置に接続先を入れる。

## Orval の設定で必須のもの

- `urlEncodeParameters: true`。既定ではパスパラメーターをエンコードせず、`/` や `#` を含む名前で URL が壊れる
- `override.fetch.includeHttpResponseReturnType: false`。これがないと戻り値が `{ data, status, headers }` になる

## 移行で見つかった不具合

`/api/reports/cfd` の入力が非公開の型を埋め込んでいたため、huma が `store`、`workspace`、`since`、`until` を読まなかった。GUI の累積フロー図は指定を無視して直近 30 日を返していた。Orval の型が余分なプロパティとして検出した。入力の型を公開して直す。

## 変えないもの

`StoresChanged` の型だけ `Record<string, never>` から `{ [key: string]: unknown }` に緩む。参照している箇所はない。

## 検証

- `pnpm lint`、`pnpm check`、`pnpm test`、`pnpm gen` の差分なし
- `make e2e` で実サーバーに対して画面を確かめる
- `go test ./...`
