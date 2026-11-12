import { Refused } from "../../model/errors";
import type { Workspace } from "../../model/workspace";
import type { WorkspacesPort } from "../../port/workspaces";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Catalog } from "../catalog/catalog";

// ワークスペースの作成ダイアログ(03「ワークスペースの作成」)。検査はサーバーに任せ、断りの文言を赤字で出す。
// 作ったら一覧を読み直し、onCreated でそのワークスペースの概要を開く。
// 非表示の Store と既定の Store は、まだない(トラック G が足す。B2-18)。

export interface WorkspaceForm {
  storeId: string;
  name: string;
  prefix: string;
}

export interface WorkspaceDialogState {
  form: WorkspaceForm;
  // stores は、書き込める Store(閲覧のみを除く)。
  stores: { value: string; label: string }[];
  // blocked は、作れないときの文。空なら入力を出す。
  blocked: string;
  error: string;
}

export const prefixPreviewHelp = "Epic・Story・Task・Bug・Step の ID が WEB-1 のようになります。";

export interface WorkspaceDialog {
  state: Readable<WorkspaceDialogState | null>;
  // open は、storeId の Store で開く。書けないか省いたときは、書ける先頭の Store。
  open(storeId?: string): void;
  // update は、入力を変える。接頭辞は大文字にして持つ。
  update(patch: Partial<WorkspaceForm>): void;
  submit(): Promise<void>;
  close(): void;
}

export interface WorkspaceDialogDeps {
  catalog: Catalog;
  port: WorkspacesPort;
  onCreated(storeId: string, workspace: Workspace): void;
}

export function createWorkspaceDialog(deps: WorkspaceDialogDeps): WorkspaceDialog {
  const state = writable<WorkspaceDialogState | null>(null);

  return {
    state: readonly(state),
    open(storeId) {
      const all = get(deps.catalog.stores);
      const writableStores = all.filter((s) => !s.readOnly);
      const store = writableStores.find((s) => s.id === storeId) ?? writableStores[0];
      let blocked = "";
      if (all.length === 0) blocked = "先に Store を追加してください";
      else if (!store) blocked = "ワークスペースを追加できる Store がありません";
      state.set({
        form: { storeId: store?.id ?? "", name: "", prefix: "" },
        stores: writableStores.map((s) => ({ value: s.id, label: s.name })),
        blocked,
        error: "",
      });
    },
    update(patch) {
      state.update((s) => {
        if (!s) return s;
        const next = { ...s.form, ...patch };
        if (patch.prefix !== undefined) next.prefix = patch.prefix.toUpperCase();
        return { ...s, form: next };
      });
    },
    async submit() {
      const s = get(state);
      if (!s || s.blocked) return;
      try {
        const created = await deps.port.create(s.form.storeId, s.form.name, s.form.prefix);
        state.set(null);
        await deps.catalog.load();
        deps.onCreated(s.form.storeId, created);
      } catch (e) {
        if (!(e instanceof Refused)) throw e;
        state.update((cur) => (cur ? { ...cur, error: e.message } : cur));
      }
    },
    close() {
      state.set(null);
    },
  };
}
