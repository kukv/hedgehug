# RDRAAgent - 使い方ガイド

## 概要

RDRAAgent は、RDRA(Relationship Driven Requirement Analysis)の手法で、初期要望から要件定義と仕様のたたき台を作るツールです。`node menu.js` のメニューから、Claude Code(`claude` コマンド)を呼び出して各成果物を生成します。

このリポジトリでは、hedgehug の設計を立て直すために使います。入力の `初期要望.txt` には hedgehug の初期要望を書いています。

## 必要なもの

- Node.js 18 以上(外部ライブラリは使わないので `npm install` は不要)
- Claude Code(`claude` コマンドにログイン済みであること)
- OS:macOS または Windows 11
  - メニュー 11・12・22 はクリップボードとブラウザを使うため、macOS と Windows だけで動きます

## 入力

| ファイル | 内容 |
|---|---|
| `初期要望.txt` | システム名、背景、アクター、外部システム、要求、制約、ビジネスポリシー、ビジネスパラメータ、業務概要。書き方は `Samples/` の各 `初期要望.txt` を参照 |
| `モデル設定.json` | 生成に使う CLI とモデル(下の「モデル設定」) |

Phase1 は、初期要望に具体的に書かれたアクターと外部システムだけを洗い出します。洗い出してほしいものは初期要望に書いてください。

## 起動

```bash
node menu.js
```

## メニュー

| 番号 | 内容 | 出力 |
|---|---|---|
| 1 | フェーズ単位の実行。成果物がそろっていない最初の Phase を 1 つ実行する。Phase4 まで揃っていれば Phase5 を実行する | `0_RDRAZeroOne/phase1〜4/`、`1_RDRA/` |
| 2 | 一括要件定義。成果物がない Phase から Phase5 まで続けて実行する | 同上 |
| 11 | RDRAGraph を表示。関連データを作り、クリップボードにコピーして RDRAGraph をブラウザで開く | `1_RDRA/関連データ.txt` |
| 12 | Spreadsheet に展開。RDRA 定義をクリップボードにコピーし、Google Spreadsheet を開く。ZeroOne シートに貼り付けて使う | `1_RDRA/ZeroOne.txt` |
| 21 | 仕様の作成。論理データモデル・ビジネスルール・画面を生成する。先に `1_RDRA/関連データ.txt` が必要 | `2_RDRASpec/` |
| 22 | BUC・アクター別の画面を表示。http://localhost:3002/ でサーバーを起動し、ブラウザで開く。先にメニュー 21 が必要 | なし |
| 0 | メニュー終了。起動中のサーバーも止める | なし |
| 99 | 生成した成果物の削除 | `0_RDRAZeroOne/phase1〜4/`、`1_RDRA/`、`2_RDRASpec/`、`2_RDRASpec/phase1/` の中のファイルを削除する |

### 各 Phase で作るもの

| Phase | 内容 | ファイル |
|---|---|---|
| 1 | 要素の洗い出し | `アクター.tsv` `外部システム.tsv` `ビジネスポリシー.tsv` `ビジネスパラメータ.tsv` `業務.tsv` `情報.tsv` `状態.tsv` `バリエーション.tsv` `条件.tsv` `要求.tsv` `初期要望分析.md` |
| 2 | 業務・情報・状態の詳細化 | `業務.tsv` `情報.tsv` `状態.tsv` |
| 3 | アクターのコンテキスト化、BUC の定義 | `アクター.tsv` `BUC.tsv` `情報.tsv` |
| 4 | 関係モデル化 | `BUC.tsv` `条件.tsv` `バリエーション.tsv` `状態.tsv` |
| 5 | `1_RDRA/` へのまとめ、システム概要と関連データの作成 | `1_RDRA/` の各 TSV、`システム概要.json`、`関連データ.txt` |

メニュー 21 は、`2_RDRASpec/phase1/` に中間の成果物を作ったあと、`2_RDRASpec/論理データモデル.md`・`ビジネスルール.md`・`画面照会.json` を作ります。

## 進め方

1. `初期要望.txt` を書く
2. メニュー 2 で Phase1〜5 を一括で実行する(1 つずつ確かめたいときはメニュー 1 を繰り返す)
3. メニュー 11 で RDRAGraph を開き、要素のつながりを確かめる
4. 気になる点は、`初期要望.txt` を直してメニュー 99 で消してからやり直すか、該当する Phase 以降のフォルダーを消して再実行する
5. メニュー 21 で仕様を作り、メニュー 22 で画面を確かめる

ある Phase をやり直すには、その Phase 以降のフォルダー(例:Phase3 からなら `0_RDRAZeroOne/phase3/` と `phase4/`)を消して、メニュー 2 を実行します。メニュー 1・2 はファイルがそろっている Phase を飛ばすので、後ろの Phase を残すと古い成果物のまま Phase5 に進みます。

## LLM の呼び出し方

メニュー 1・2・21 は、`RDRA_Knowledge/helper_tools/parallelRun/parallel-runner.js` から、Phase の中のプロンプト(`RDRA_Knowledge/0_RDRAZeroOne/phaseN/*.md` など)ごとに `claude -p` を 1 つずつ同時に起動します。

- 各プロンプトの本文は標準入力で渡します
- Phase で共通に使うファイル(`RDRA.md`、`初期要望.txt`、前の Phase の成果物)は、実行のたびに `RDRA_Knowledge/system_prompts/` に 1 つのファイルとしてまとめ、`--append-system-prompt-file` で渡します
- 1 つのプロンプトのタイムアウトは 10 分です

### モデル設定

`モデル設定.json` の `default.provider` で使う CLI を選び、`providers.<名前>` でコマンドと引数を決めます。claude の既定は次のとおりです。

```
claude -p --output-format text --permission-mode auto --setting-sources= \
  --model claude-sonnet-5-5 --append-system-prompt-file <共通コンテキストのファイル>
```

- `--permission-mode auto`:ファイルの読み書きなどを、確認のダイアログなしで Claude Code の自動判定に任せます
- `--setting-sources=`:ユーザー・プロジェクトの設定と `CLAUDE.md` を読み込みません。生成に関係のない指示が混ざらないようにするためです
- モデルを変えるときは `providers.claude.vars.model` を書き換えます

## フォルダー構成

```
├── menu.js                   # メニュー
├── 初期要望.txt               # 入力
├── モデル設定.json             # 使う CLI とモデル
├── Samples/                  # 初期要望のサンプル(介護事業者向けシステム、図書館システム、貸し会議室SaaS)
├── RDRA_Knowledge/
│   ├── 0_RDRAZeroOne/        # Phase1〜5 のプロンプト
│   ├── 1_RDRA/               # RDRA の考え方(RDRA.md)、RDRAGraph・RDRASheet の形式
│   ├── 2_RDRASpec/           # 仕様の作成のプロンプト(phase1・phase2)
│   ├── system_prompts/       # 実行時に作る共通コンテキスト
│   └── helper_tools/         # メニューの実装、並列実行、変換・コピー・削除のツール、画面のサーバー
├── 0_RDRAZeroOne/            # 出力:Phase1〜4
├── 1_RDRA/                   # 出力:まとめた RDRA 定義
└── 2_RDRASpec/               # 出力:仕様
```

TSV は UTF-8 のタブ区切りで、1 行目がヘッダーです。

## Cursor から使う

Cursor では、`/` のスラッシュコマンド(`.cursor/commands/`)から同じ手順を対話で実行できます。

## トラブルシューティング

- **Phase のファイルができない**:ターミナルに出る `[プロンプト名]` 付きのエラーを確かめ、メニュー 1 で同じ Phase をやり直してください。そろっていないファイルがある Phase は、もう一度実行されます。
- **メニュー 21 で「関連データ.txt が存在しません」と出る**:メニュー 2 で Phase5 まで終えるか、メニュー 11 を実行してください。
- **メニュー 22 の画面が開かない**:ポート 3002 を別のプロセスが使っていないか確かめてください。メニュー 0 で終了すると、サーバーも止まります。

## 参考

- RDRA ツール:https://vsa.co.jp/rdratool/
- RDRAGraph:https://vsa.co.jp/rdratool/graph/
- RDRA の考え方:`RDRA_Knowledge/1_RDRA/RDRA.md`

## ライセンス

MIT License
