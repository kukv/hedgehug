# フェーズ 4e-3 の引き継ぎ（2026-11-03 時点）

WSL から mac に作業を移すための引き継ぎ。4e-3（カンバン・テーブル・保存済みビュー・マイタスク）は **plan を作っている途中** で、実装のコードはまだない。

## いまの状態

- ブランチ `phase4e3-views`（main の 79a5277 から）
- plan の下書き: `docs/superpowers/plans/2026-11-03-phase4e3-views.md`（約 15,600 行、Task 1〜7 と末尾の「決めたこと」1〜21）
  - 6 人の writer（sonnet）が部ごとに書き、各自の作業用の複製で `pnpm fix`・`svelte-check`（警告 0）・`biome ci`・`vitest`（Task 1 は `go test`）を通した。ほかの Task のものは境目の型のスタブで確かめただけで、**実物どうしをつないで通したことはまだない**
  - Task 1: Go（`hh view show` を全 Store で評価）／Task 2: 骨組み（ルート・`ViewDirectory`・CLI の文字列・`ListScope`・`ListBar`・TanStack Table 9.3.1）／Task 3: ツリーを `ListScope` に対応／Task 4: カンバン／Task 5: テーブル／Task 6: ビューの画面・クエリの画面・ダイアログ・タブの入れ物／Task 7: マイタスク・もしかして・入口の結線・E2E
- 概要 plan `2026-11-03-phase4e-screens.md` の細部 36・39・58 を利用者の決定に合わせて直し、細部 81〜90（4e-3 で足した細部）を追加した
- spec `docs/superpowers/specs/2026-11-01-phase4-gui-design.md` に 4e-3 の決定の段落を追加した

## 利用者の決定（2026-11-03、すべて推奨案）

plan 末尾の「決めたこと」と概要 plan の細部 81〜90 に記録済み。

1. 保存していないクエリの画面は `/query/<layout>?q=`（全 Store で評価、レイアウトの切り替えは保存しない、「ビューを保存」あり。router はこのルートだけ `?q=` を残す）
2. `hh view show` は `--store` をビューを読む場所としてだけ使い、クエリは常に全マウント Store で評価する（親設計「ビューは常に統合する」に合わせる）。Go の直しを Task 1 に入れた
3. プロジェクトのカンバンは scoped の `statuses` に加え、タスクのいる表示外のステータスの列も出す（タスクを隠さない）
4. カンバンで移す先の必須の項目のうち、ダイアログで入力できないもの（親・ブロッカー・ウォッチャー・リポジトリなど）が欠けていれば、移動を止めて「詳細で入力」へ案内する（サーバーの 422 を起こさない）
5. 「もしかして」は編集距離 2 以内、なければ打った語（3 文字以上）を含む名前

## 残作業（順番に）

### 1. plan の通しの確認（途中で止めた。最初からやり直す）

Task 1→7 を1つの複製（main 79a5277 の写し）に順に当て、各 Task の終わりに検査（Go: 関係するパッケージの `go test -race` と `go vet`、web: `pnpm fix`・`pnpm check`・`biome ci`・`vitest run` 全件）を流す。最後に `pnpm build` と E2E（`make e2e`）を全件、もう1回 repeat。

- 当たらない「変更前」、型・名前の食い違い、通らないテストは、後の Task を前の Task の実物に合わせる形で plan の中を直し、plan の末尾に「## 組み上げで直した点」を足して一行ずつ記録する
- 落ちたテストは失敗の出力と実行条件を残す
- **全件のテストを同時に何本も流さない**（負荷で時間切れが出るうえ、WSL ではメモリを食った）

### 2. Task 3 の最初の Step に、既存テストの時間切れの直しを2つ足す

Task 3 の最初の Step（TreeView の「lets a late wait for a row give way to a newer one」を偽のタイマーにする直し）は plan に入っている。同じ `test(web): …` のコミットに次の2つを足す。理由は plan の「決めたこと」21。

(a) `web/app/detail/BodyEditor.test.ts` の「offers the people for @ from GET /api/people」の `mount` のあとを次にする:

```ts
    const { container } = mount(BodyEditor, props({ value: "" }), k);
    const view = viewOf(container);
    // The completion starts 100ms after the typing (CodeMirror's activateOnTypingDelay). The clock
    // is a fake one: on a busy machine the real 100ms ran past the 1s of waitFor.
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
    try {
      view.dispatch({
        changes: { from: 0, insert: "@su" },
        selection: { anchor: 3 },
        userEvent: "input.type",
      });
      await vi.advanceTimersByTimeAsync(150);
      expect(k.api.GET).toHaveBeenCalledWith("/api/people", {
        params: { query: { store: "shared" } },
      });
    } finally {
      vi.useRealTimers();
    }
```

(b) `web/vitest.config.ts` の `test` に:

```ts
    setupFiles: ["./vitest.setup.ts"],
    // A test takes about 0.5s alone; on a busy machine (other suites at the same time) it took
    // over the default 5s. A test that hangs still fails, at 20s.
    testTimeout: 20_000,
```

証拠（WSL2・16 コア、2026-11-03）:

- TreeView のテスト: 全件 2 本を裏で流しながら 4 回 → 元のテストは 4 回中 2 回落ち（`expected null to be 'M199'`）、直すと 4 回とも通る。seq の判定を外すと直したテストが落ちる（不具合を捕まえることの確認）。計測では、負荷で 80ms のタイマーが約 1.1 秒遅れていた。製品のコードの不具合ではない
- ほか: 全件 3 本を同時に 3 回 → 9 本中 7 本が落ちた。BodyEditor の上のテストが 3 回（`expected "vi.fn()" to be called with arguments: [ '/api/people', … ]`）。TaskDetail.sections・TaskDetail・TaskDetail.more・CreateTask の各ファイルの最初のテストが `Test timed out in 5000ms`（計 13 回。どれも単独では 0.5 秒前後）。(a)(b) を当てると 9 本とも 866 件通る

### 3. plan に残っている「要相談」の節を消す

結論はすべて plan 末尾の「決めたこと」にある。Task 2（部 A）・Task 3（部 B）・Task 4（部 C）・Task 6（部 E）の末尾の「要相談」の節を消し、必要ならその Task の本文を結論に合わせる。

- Task 2: `viewShow` は `a.ss` の差し替え（決めたこと 20）、`router.replace` は通知する・`sameDraft` は空白だけ除く（6・19）
- Task 3: Store をまたぐドロップの文言は既存の「置けません」のまま（5）、部 A への依頼は Task 2 で対応済み（19）
- Task 4: プロジェクトの見出しは Task 7 で `ProjectHeader` に切り出し済み（16）、カードのキーボード操作は作らない（17）
- Task 6: 同じ名前のビューは保存を無効にして理由（14）、空のクエリは TreeView がそのまま使う（7）、足元は `hh view show`（10）

### 4. 仕上げ

- plan 冒頭の「着手の前に」の確かめた内容の文を、通しの確認でやったことに書き換える
- 親設計（Claude Docs の Hedgehug 設計ドキュメント、https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc）の「保存済みビュー」に、`hh view show` は `--store` をビューを読む場所としてだけ使いクエリは全マウント Store で評価する、の1行を足す（利用者の決定 2）
- plan・概要 plan・spec をコミットして push し、利用者の承認をもらう

### 5. 実装

承認のあと、新しいセッションで SDD（subagent-driven-development）。実装者は sonnet。Task の番号の順に1つずつ（同時に2人動かすとコミットが混ざる）。Task 2〜6 の画面は Task 7 の結線まで入口が出ないので、E2E の全件は Task 7 で流す。最後に最終レビュー（opus）と PR。

## 作業の材料

WSL の worktree の `.superpowers/sdd/2026-11-03-phase4e3-views/`（git の対象外）に置いたもの。mac には来ないので、要るものは作り直す。

- キャンバス（デザイン）のテキスト: Hedgehug GUI デザイン https://claude.ai/artifact/K14rwULxyyS2nYNbMKSr5L を Artifact の read（paths）で落として読む
- writer への共通の指示（e3-brief.md）: plan の冒頭と各 Task の Interfaces に中身は入っている
- ギャップ調査・親設計のテキスト: 必要なら読み直す
