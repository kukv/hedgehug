import { NotFound, Refused } from "../../model/errors";
import type { Member } from "../../model/member";
import type { Comment } from "../../model/notes";
import type { CatalogPort } from "../../port/catalog";
import type { NotesPort, NoteTarget } from "../../port/notes";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Drafts } from "../drafts/drafts";
import type { Toasts } from "../toasts/toasts";

// コメントの区画の状態(02「コメント」)。アイテムの詳細パネルとプロジェクトの概要が、それぞれ 1 つずつ持つ(A4a-7・A4a-21)。

export const commentPlaceholder = "コメントを書く。@ログイン名 でメンション。Ctrl+Enter で送信";
export const commentDeletedMessage = "コメントを削除しました";
export const commentDeleteDialog = {
  title: "コメントを削除",
  lines: ["このコメントを削除します。"],
  ok: "削除",
};
// 断りでも「ない」でもない失敗(通信の失敗など)のときの文言。仕様にない文言なので、決まったら差し替える。
export const commentFailedMessage = "コメントを保存できませんでした";
const unknownUser = "不明なユーザー";

export interface CommentsState {
  target: NoteTarget | null;
  rows: Comment[];
  members: Member[];
  me: string;
  newBody: string;
  // editing は、編集中のコメントの内部 ID と入力。なければ null。
  editing: { id: string; body: string } | null;
  // deleting は、削除の確認を出しているコメントの内部 ID。なければ null。
  deleting: string | null;
  // busy は、送信中(二重に投稿しない)。
  busy: boolean;
}

export interface Comments {
  state: Readable<CommentsState>;
  // open は、持ち主のコメントを読む。持ち主が変わったら入力を空にする。同じ持ち主なら入力は残す。
  open(target: NoteTarget): Promise<void>;
  // reload は、画面への反映(06)。入力は書き換えない。
  reload(): Promise<void>;
  // close は、区画を閉じたとき。入力と一覧を捨て、書きかけの登録を外す。
  close(): void;
  setNew(text: string): void;
  // post は、新しいコメントを投稿する。空なら何もしない(false)。
  post(): Promise<boolean>;
  escapeNew(): Promise<void>;
  startEdit(id: string): void;
  setEditing(text: string): void;
  cancelEdit(): void;
  // saveEdit は、編集を保存する。空にしたときと前と同じときは、保存せずに閉じる(true)。断られたら閉じない(false)。
  saveEdit(): Promise<boolean>;
  escapeEdit(): Promise<void>;
  askDelete(id: string): void;
  cancelDelete(): void;
  confirmDelete(): Promise<void>;
}

export interface CommentsDeps {
  notes: Pick<NotesPort, "listComments" | "postComment" | "editComment" | "deleteComment">;
  catalogPort: Pick<CatalogPort, "listMembers" | "me">;
  toasts: Toasts;
  drafts: Pick<Drafts, "register" | "confirmLeave">;
  // scope は、書きかけのキーを分ける。詳細パネルとプロジェクトの概要が同時に開くため。
  scope: "item" | "project";
}

// isMine は、自分が書いたコメントか(編集と削除のボタンを出す。ログイン名は大文字と小文字を区別しない)。
export const isMine = (c: Comment, me: string): boolean =>
  me !== "" && c.by.toLowerCase() === me.toLowerCase();

// commentAvatar は、アバターに出す名前。メンバーにいない人は中身のない丸(01「不明なユーザー」)。
export const commentAvatar = (c: Comment): string => (c.byName === unknownUser ? "" : c.byName);

const sameTarget = (a: NoteTarget | null, b: NoteTarget) =>
  a !== null && a.kind === b.kind && a.storeId === b.storeId && a.id === b.id;

const initial = (): CommentsState => ({
  target: null,
  rows: [],
  members: [],
  me: "",
  newBody: "",
  editing: null,
  deleting: null,
  busy: false,
});

export function createComments(deps: CommentsDeps): Comments {
  const state = writable<CommentsState>(initial());
  const patch = (p: Partial<CommentsState>) => state.update((s) => ({ ...s, ...p }));
  const isOpen = (target: NoteTarget) => sameTarget(get(state).target, target);

  async function load(target: NoteTarget) {
    try {
      const [rows, members, me] = await Promise.all([
        deps.notes.listComments(target),
        deps.catalogPort.listMembers(target.storeId).catch((): Member[] => []),
        // me が決まらないときは空(A2 の決定 3)。
        deps.catalogPort.me(target.storeId).catch(() => ""),
      ]);
      // 待っているあいだに別の持ち主へ移った(または閉じた)なら、この結果は捨てる。
      if (!isOpen(target)) return;
      // 読み直したら消えていたコメントの編集と削除の確認は、続けられないので外す(見えない書きかけを残さない)。
      const alive = (id: string | undefined) => rows.some((r) => r.id === id);
      state.update((s) => ({
        ...s,
        rows,
        members,
        me,
        editing: s.editing && alive(s.editing.id) ? s.editing : null,
        deleting: alive(s.deleting ?? undefined) ? s.deleting : null,
      }));
    } catch (e) {
      // 持ち主がなくなっていたら、詳細パネルが閉じるので何もしない。
      if (!(e instanceof NotFound)) throw e;
    }
  }

  // failed は、書き込みの失敗の始末。ないものは黙って読み直し、断りはその文言を、ほかはエラーのトーストを出す。
  async function failed(e: unknown, target: NoteTarget) {
    if (e instanceof NotFound) await load(target);
    else deps.toasts.show(e instanceof Refused ? e.message : commentFailedMessage, { error: true });
  }

  // settled は、書き込みが終わったときの送信中の印の戻し。別の持ち主へ移っていたら、そちらの印を触らない。
  const settled = (target: NoteTarget) => {
    if (isOpen(target)) patch({ busy: false });
  };

  const original = (id: string) => get(state).rows.find((r) => r.id === id)?.body ?? "";

  function editDirty(): boolean {
    const { editing } = get(state);
    return editing !== null && editing.body !== original(editing.id);
  }

  // キーは comment: で始める(確認の本文が決まる。Task 6)。Esc はその入力欄のぶんだけ確認する。
  const newKey = `comment:new:${deps.scope}`;
  const editKey = `comment:edit:${deps.scope}`;
  let unregister: (() => void)[] = [];
  // 書きかけの登録は、区画を開いているあいだだけ持つ(open で足し、close で外す)。
  function register() {
    if (unregister.length > 0) return;
    unregister = [
      deps.drafts.register(newKey, () => get(state).newBody.trim() !== ""),
      // 反映を待つのは、失うものがあるとき(空にした編集は、違うが、失うものがない)。
      deps.drafts.register(
        editKey,
        editDirty,
        () => editDirty() && (get(state).editing?.body.trim() ?? "") !== "",
      ),
    ];
  }

  async function post(): Promise<boolean> {
    const { target, newBody, busy } = get(state);
    const body = newBody.trim();
    if (!target || body === "" || busy) return false;
    patch({ busy: true });
    try {
      await deps.notes.postComment(target, body);
    } catch (e) {
      settled(target);
      await failed(e, target);
      return false;
    }
    settled(target);
    // 送っているあいだに打ち足した文字は残す(送った入力のままのときだけ空にする)。
    if (isOpen(target) && get(state).newBody === newBody) patch({ newBody: "" });
    await load(target);
    return true;
  }

  async function saveEdit(): Promise<boolean> {
    const { target, editing, busy } = get(state);
    if (!target || !editing || busy) return false;
    const body = editing.body.trim();
    if (body === "" || body === original(editing.id)) {
      patch({ editing: null });
      return true;
    }
    patch({ busy: true });
    try {
      await deps.notes.editComment(target, editing.id, body);
    } catch (e) {
      settled(target);
      await failed(e, target);
      return false;
    }
    settled(target);
    // 保存しているあいだに打ち足した文字は残す(保存した入力のままのときだけ閉じる)。
    if (isOpen(target) && get(state).editing?.body === editing.body) patch({ editing: null });
    await load(target);
    return true;
  }

  async function confirmDelete() {
    const { target, deleting } = get(state);
    patch({ deleting: null });
    if (!target || !deleting) return;
    try {
      await deps.notes.deleteComment(target, deleting);
    } catch (e) {
      // ほかで消えていたら、読み直すだけにする。
      await failed(e, target);
      return;
    }
    deps.toasts.show(commentDeletedMessage);
    await load(target);
  }

  return {
    state: readonly(state),
    open(target) {
      if (!isOpen(target)) state.set({ ...initial(), target });
      register();
      return load(target);
    },
    async reload() {
      const { target } = get(state);
      if (target) await load(target);
    },
    close() {
      unregister.forEach((off) => {
        off();
      });
      unregister = [];
      state.set(initial());
    },
    setNew: (text) => patch({ newBody: text }),
    post,
    async escapeNew() {
      // 空なら何も失わないので、確認しない。
      if (get(state).newBody.trim() === "") return;
      if (await deps.drafts.confirmLeave(newKey)) patch({ newBody: "" });
    },
    startEdit(id) {
      patch({ editing: { id, body: original(id) } });
    },
    setEditing: (text) =>
      state.update((s) => (s.editing ? { ...s, editing: { ...s.editing, body: text } } : s)),
    cancelEdit: () => patch({ editing: null }),
    saveEdit,
    async escapeEdit() {
      if (!editDirty() || (await deps.drafts.confirmLeave(editKey))) patch({ editing: null });
    },
    askDelete: (id) => patch({ deleting: id }),
    cancelDelete: () => patch({ deleting: null }),
    confirmDelete,
  };
}
