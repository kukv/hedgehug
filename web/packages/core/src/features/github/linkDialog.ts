import { NotFound, Refused } from "../../model/errors";
import type { LinkKind } from "../../model/link";
import type { LinksPort } from "../../port/links";
import { get, type Readable, readonly, writable } from "../../reactive/store";

// 「PR をリンク」「Issue をリンク」のダイアログ(07「追加」)。題名と種別だけが違う。検査はサーバーに任せ、
// 断りはダイアログの赤字に出す。問い合わせのあいだは「リンク」を無効にする(D2 画面の決定 S15 と同じ)。
// 断り以外の失敗は出さない(D2 画面の決定 S20)。リンクできたら閉じ、トーストは出さない。

export interface LinkDialogState {
  kind: LinkKind;
  repos: string[];
  repo: string;
  number: string;
  addToProject: boolean;
  asking: boolean;
  error: string;
}

export interface LinkDialog {
  state: Readable<LinkDialogState | null>;
  open(kind: LinkKind, storeId: string, itemId: string, repos: string[]): void;
  update(p: Partial<Pick<LinkDialogState, "repo" | "number" | "addToProject">>): void;
  // setRepos は、開いたあとに読み直しで届いたプロジェクトのリポジトリを取り込む。入力済みの repo は、選択肢に
  // あれば(大文字小文字を区別しない)その選択肢に、なければ先頭にする(セレクトに無い値を残さない)。
  setRepos(repos: string[]): void;
  submit(): Promise<void>;
  close(): void;
}

export interface LinkDialogDeps {
  links: LinksPort;
  // onAdded は、リンクしたあと(GitHub の欄と概要の一覧を読み直す)。
  onAdded(): void;
}

// choose は、repos が届いたときの repo。repos が空なら入力のまま。
function choose(repo: string, repos: string[]): string {
  if (repos.length === 0) return repo;
  const typed = repo.trim().toLowerCase();
  return repos.find((r) => r.toLowerCase() === typed) ?? repos[0] ?? "";
}

export function createLinkDialog(deps: LinkDialogDeps): LinkDialog {
  const state = writable<LinkDialogState | null>(null);
  let target: { storeId: string; itemId: string } | null = null;
  // opened は、開くたびに増やす(閉じたあとに届いた答えを捨てる)。
  let opened = 0;

  const patch = (p: Partial<LinkDialogState>) => state.update((s) => (s ? { ...s, ...p } : s));

  function close() {
    opened += 1;
    target = null;
    state.set(null);
  }

  return {
    state: readonly(state),
    open(kind, storeId, itemId, repos) {
      opened += 1;
      target = { storeId, itemId };
      state.set({
        kind,
        repos,
        repo: repos[0] ?? "",
        number: "",
        addToProject: true,
        asking: false,
        error: "",
      });
    },
    update: patch,
    setRepos(repos) {
      state.update((s) => {
        if (!s || (s.repos.length === repos.length && s.repos.every((r, i) => r === repos[i])))
          return s;
        return { ...s, repos, repo: choose(s.repo, repos) };
      });
    },
    async submit() {
      const s = get(state);
      if (!s || !target || s.asking) return;
      const mine = opened;
      const t = target;
      patch({ asking: true, error: "" });
      try {
        await deps.links.add(t.storeId, t.itemId, {
          kind: s.kind,
          repo: s.repo,
          number: s.number,
          addToProject: s.repos.length === 0 && s.addToProject,
        });
      } catch (e) {
        if (mine !== opened) return;
        if (e instanceof NotFound) {
          close();
          deps.onAdded();
          return;
        }
        patch({ asking: false, error: e instanceof Refused ? e.message : "" });
        return;
      }
      if (mine !== opened) return;
      close();
      deps.onAdded();
    },
    close,
  };
}
