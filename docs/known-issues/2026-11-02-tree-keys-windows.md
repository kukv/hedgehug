# 既知の不具合: Windows の Chrome / Edge でツリーのキー操作が効かない

- 状態: 解決（2026-11-03）。原因は GUI ではなく、ブラウザ拡張の Vimium だった。利用者が Vimium を止めることにしたので、GUI のコードは変えない。詳しくは「調査の結果」
- 調査: Windows 側の Claude Code で行った。この文書はその引き継ぎ資料と、調査の結果

## 調査の結果（2026-11-03）

利用者と同じ形（WSL2 の Ubuntu で `hedgehug ui`、Windows 11 の Chrome 157 で開く。拡張はそのまま）で、下の「証拠の取り方」のスニペットを使って調べた。

### 症状 1・3: Vimium がキーを奪っていた

Chrome に Vimium 2.4.2 が入っていた。Vimium は window の capture で keydown を受け、自分のキーなら `stopImmediatePropagation` と `preventDefault` で握りつぶす。対になる keyup も握りつぶす。拡張の content script はページのスクリプトより先に listener を付けるので、ページの listener は window の capture でも、そのキーを一度も受け取れない（Vimium のソースにも「ページが先にキーを取れないよう window に付ける」と書いてある）。

- 症状 1（入力中の Esc）: 入力欄にフォーカスがあると Vimium は insert mode になり、Esc の keydown で `activeElement.blur()` を呼んで、その Esc を握りつぶす（`content_scripts/mode_insert.js`）。`AddRow` に Esc が届かないので入力行は閉じず、文字が残ったままフォーカスだけ外れる。記録: 入力欄で `test` と打って Esc を押すと、Esc の keydown がページに一度も届かないまま `focusout input.title -> null` が起き、フォーカスは body に移り、入力行の値は `test` のまま。変換中の Esc（keyCode 229）は Vimium が IME に任せるので、この症状は変換中でないときに出る。
- 症状 3（K）: Vimium の既定のキーで `K` は nextTab、`k` は scrollUp（`j` も scrollDown）。行にフォーカスがあると Vimium は normal mode なので、K を握りつぶす。記録: `k`・`K`・`j` は keydown も keyup もページに届かず、同じ行で押した `s` と `↓` は届いてツリーが処理した。
- アプリのほかのキーで Vimium の既定と重なるものはない（`c`、`s`・`S`、Space、Tab、Enter、矢印、Alt+↑↓、Ctrl+K、Ctrl+Shift+S）。

PR #31 の直しが効かなかったのは、どれもページにキーが届いたあとの処理を直したもので、届かないことは直せないから。Playwright の Chromium には拡張が入っていないので再現しなかった。

### 症状 2: Windows 側では起きていない

合成のキー（連打と 1 秒間隔、↑ と ↓）でも、利用者の実際のキーボード（日本語入力オン・オフ）でも、Alt+↑↓ はすべてツリーに届き、押した回数だけ並べ替わった。実キーボードの記録では Alt+↑↓ の keydown 51 回がすべて行に届き、握りつぶされたものはなかった。`preventDefault` されなかったのは、端の行でそれ以上動かせないとき（`keyAction` が `none`）だけ。利用者も「Windows 側だと問題なく動いてそう」と確かめた。Vimium の既定のキーに Alt+矢印はない。

### 対処

ページ側からは Vimium より先にキーを受け取れないので、GUI のコードでは直せない（入力欄が行き先なしにフォーカスを失ったら閉じる、K を別のキーにする、などの回避は spec の変更になる）。利用者に聞き、利用者が Vimium を止めることにした。GUI のコードは変えない。

### 調査で分かった、ほかのこと

- Windows でネイティブにビルドすると `internal/atomicfile`（`Rename` と `ReadFile` が `atomicfile_unix.go` にしかない）と `internal/testenv`（`syscall.Kill`）でコンパイルが通らない。afc3dc1 で Windows 向けのコードをやめ、README に「Windows では WSL を使う」と書いてあるので、これは想定どおり。Windows で調べるときは `GOOS=linux` でビルドして WSL で起動する。

## 症状

利用者の環境: WSL2 で `hedgehug ui` を起動し、Windows の Chrome または Edge で開く。行はマウスでクリックして選ぶ。日本語入力のオン・オフは分かっていない。ブラウザ拡張は入っている。

1. 入力行でタイトルを入力しているときに Esc を押すと、入力した文字が残ったままフォーカスだけ外れる。入力行は閉じない。
2. ⌥↑↓（Windows では Alt+↑↓）の並べ替えが、2 回目以降に効かないことがある。
3. 行を選んで K を押しても、種類のメニューが出ない。

期待する動き（spec `docs/superpowers/specs/2026-11-01-phase4-gui-design.md` と plan `docs/superpowers/plans/2026-11-02-phase4d3-tree.md`）:

1. 入力中の Esc で入力行が閉じ、フォーカスは行に移る。変換中の Esc は IME に任せる。
2. Alt+↑↓ を押した回数だけ、順に並べ替える。
3. 行にフォーカスがあるとき、K で種類のメニューが開く。

## これまでに直したもの（それでも利用者の環境では直らなかった）

すべて PR #31（2026-11-02 マージ、5bffa01）に入っている。

| コミット | 内容 |
| --- | --- |
| c442177 | 作成の応答が返る前に Esc で閉じた入力行が、応答のあとに開き直してフォーカスを奪っていた |
| 5d5dc55〜f98e0e9 | 移動のキー（Alt+↑↓、Tab、Shift+Tab）とドロップを待ち行列で順に送る |
| 8c899ff・516a23e | ID・開閉ボタンのクリックでフォーカスがボタンに残り、行のキーがツリーに届かなかった |
| afb3ba1 | 行をクリックしたら入力行を閉じる |
| 82e1b13 | 変換中の Esc（`isComposing`）では入力行を閉じない |

Linux の Chromium（Playwright、IME なし）では、書き込みを 1 秒遅らせても 3 つとも再現しなかった。サーバーが最新のビルドを配信していることは確かめてある。

## 調査の決まり

- 推測で直さない。推測による直しはすでに 2 回外れている。まず利用者の環境で証拠を取り、原因を特定してから直す。
- 「拡張を外して使う」という前提は置かない（利用者が却下した）。拡張が入っていても動くように直す。根拠なしに拡張のせいと決めつけない。
- 直すときは、直す前に落ちるテスト（単体と E2E）を先に書く。
- spec や設計と違う挙動が必要になったら、自分で決めずに利用者に聞く。

## 関係するコード

| ファイル | 見どころ |
| --- | --- |
| `web/app/tree/TreeView.svelte` | `.scroller`（role=tree）の `onkeydown` が行のキーを受ける。`input, textarea, select, [contenteditable]` が target のときは何もしない。移動の待ち行列（`moveChain`・`moves`） |
| `web/app/tree/keyboard.ts` | `keyAction` がキーから動作を決める。K・S・Space・Tab は schema を読むまで待つ（`needsSchema`） |
| `web/app/tree/AddRow.svelte` | 入力行。Enter と Esc は `!e.isComposing` のときだけ処理する |
| `web/app/tree/TreeRow.svelte` | 行。`onclickcapture` で選択してフォーカスを置く。開閉ボタンは keydown を `stopPropagation` する |
| `web/app/keys.ts` | window に付ける keydown（⌘K / Ctrl+K など）。ツリーのキーと競合しないか確認する |
| `internal/server/static.go` | `index.html` と資産の配信。Cache-Control を付けていない。古い JS が使われていないか、証拠の `scripts=` で確かめる |

ツリーのキーは `.scroller` の keydown で受けている。フォーカスが `.scroller` の外（body など）にあると、K も Alt+↑↓ も届かない。

## 証拠の取り方

Windows の Chrome / Edge で GUI を開き、開発者ツールのコンソールに次のスニペットを貼ってから症状を出す。そのあとコンソールで `copy(__hhlog.join('\n'))` を実行すると、記録がクリップボードに入る。

記録する内容: keydown を capture と bubble の両方で（`key`・`code`・修飾キー・`isComposing`・`defaultPrevented`・target・bubble まで届いたか）、その時点と 100ms 後の `document.activeElement`、入力行の値、メニューの有無、`document.hasFocus()`。ほかに keyup、composition、focusin / focusout とその移動先、window の focus / blur、pointerdown、UserAgent と読み込んだ script の URL。

```js
(() => {
  const log = (window.__hhlog = []);
  const d = (el) => {
    if (!el || el === document) return String(el && el.nodeName);
    if (!(el instanceof Element)) return String(el);
    const c = typeof el.className === "string" ? el.className.trim().split(/\s+/).slice(0, 3).join(".") : "";
    const t = (el.getAttribute("aria-label") || el.textContent || "").trim().slice(0, 20);
    return `${el.tagName.toLowerCase()}${c ? "." + c : ""}${t ? ` "${t}"` : ""}`;
  };
  const state = () => {
    const add = document.querySelector(".add input");
    return `active=${d(document.activeElement)} addInput=${add ? JSON.stringify(add.value) : "なし"} menu=${document.querySelector("[role=menu]") ? "あり" : "なし"} hasFocus=${document.hasFocus()}`;
  };
  let n = 0;
  const seen = new Map();
  window.addEventListener("keydown", (e) => {
    const id = ++n;
    seen.set(e, id);
    log.push(`#${id} cap key=${JSON.stringify(e.key)} code=${e.code} keyCode=${e.keyCode} alt=${e.altKey} ctrl=${e.ctrlKey} shift=${e.shiftKey} meta=${e.metaKey} composing=${e.isComposing} repeat=${e.repeat} trusted=${e.isTrusted} target=${d(e.target)} | ${state()}`);
    setTimeout(() => {
      if (seen.has(e)) log.push(`#${id} bubble に届かず（途中で stopPropagation） prevented=${e.defaultPrevented}`);
      log.push(`#${id} +100ms ${state()}`);
    }, 100);
  }, true);
  window.addEventListener("keydown", (e) => {
    const id = seen.get(e);
    seen.delete(e);
    log.push(`#${id} bub prevented=${e.defaultPrevented}`);
  });
  for (const t of ["keyup", "compositionstart", "compositionend"])
    window.addEventListener(t, (e) => log.push(`${t} key=${JSON.stringify(e.key ?? e.data)} target=${d(e.target)}`), true);
  window.addEventListener("focusout", (e) => log.push(`focusout ${d(e.target)} -> ${d(e.relatedTarget)}`), true);
  window.addEventListener("focusin", (e) => log.push(`focusin ${d(e.target)}`), true);
  window.addEventListener("blur", () => log.push("window blur"));
  window.addEventListener("focus", () => log.push("window focus"));
  window.addEventListener("pointerdown", (e) => log.push(`pointerdown ${d(e.target)}`), true);
  log.push(`start ${navigator.userAgent} scripts=${[...document.scripts].map((s) => s.src).join(",")}`);
  console.log("記録を開始しました。症状を出したあと、コンソールで copy(__hhlog.join('\\n')) を実行して貼り付けてください");
})();
```

症状ごとに、記録を始めてから次の手順で操作する。

1. 「タスクを追加」で入力行を開き、タイトルを入力して Esc。日本語入力のオンとオフの両方で試す。
2. 行をクリックして Alt+↑ を 3 回（応答を待たずに続けて、と 1 秒ずつあけて、の両方）。
3. 行をクリックして K。

## 起動の手順

`hedgehug ui` は GUI を `web/dist` から埋め込む。`web/dist` はリポジトリに入っていないので、先にビルドする（Node と pnpm のバージョンは `mise.toml`）。

```sh
make build              # web をビルドしてから bin/hedgehug を作る
bin/hedgehug ui         # 127.0.0.1 で起動してブラウザを開く。--no-open で URL だけ出す
```

利用者の環境と同じ形（WSL2 で起動して Windows のブラウザで開く）と、Windows で直接起動する形の両方で試すと、サーバー側とブラウザ側のどちらの差かを切り分けられる。CI は Linux だけで、Windows でのビルドとテストは 2026-11-02 から確かめていない。

## 過去の調査の記録

- `.superpowers/sdd/2026-11-02-phase4d3-tree/debug-tree-report.md`・`debug-tree2-report.md`（worktree scalable-splashing-mitten にあり、リポジトリには入っていない）。要点はこの文書の「これまでに直したもの」にまとめた。
- 再調査で見つけた、症状 1 と見た目が同じ状態: 入力行を開いたまま行をクリックすると、文字を残したまま入力行が開き続け、Esc が効かなかった。これは afb3ba1 で、行のクリックで入力行を閉じるように直した。
