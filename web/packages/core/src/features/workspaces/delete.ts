import { NotFound, Refused } from "../../model/errors";
import type { WorkspaceDeletePreview } from "../../model/workspace";
import type { WorkspacesPort } from "../../port/workspaces";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Catalog } from "../catalog/catalog";
import { itemDeleteFailedMessage } from "../items/delete";
import type { DetailView } from "../items/detail";
import type { Navigation } from "../router/navigation";
import type { Toasts } from "../toasts/toasts";

// ワークスペースの削除(03「削除」)。確認ダイアログと、名前の照合と、削除のあとの移り先。
// プロジェクトの削除(projects/delete.ts)と同じ形で、名前の入力が加わる。

// DeleteWorkspaceTarget は、削除するワークスペース。prefix は、いまの画面がこのワークスペースのものかを調べるのに使う。
export interface DeleteWorkspaceTarget {
  storeId: string;
  id: string;
  name: string;
  prefix: string;
}

export const nameMismatch = "名前が一致しません";

export const workspaceDeletedMessage = (name: string) => `「${name}」を削除しました`;

export interface WorkspaceDeleteDialogState {
  workspace: DeleteWorkspaceTarget;
  title: string;
  lines: string[];
  label: string;
  ok: string;
}

// workspaceDeleteDialog は、確認ダイアログの中身(B5b-9)。
export function workspaceDeleteDialog(
  ws: DeleteWorkspaceTarget,
  p: WorkspaceDeletePreview,
): WorkspaceDeleteDialogState {
  return {
    workspace: ws,
    title: "ワークスペースを削除",
    lines: [
      `「${ws.name}」と、その中の ${p.projects} 件のプロジェクト・${p.items} 件のアイテム、スプリント・リリース・設定を削除します。`,
      `設定の「削除済み」から戻せます。${p.retentionDays} 日を過ぎると完全に削除されます。`,
    ],
    label: "確認のため、ワークスペースの名前を入力してください",
    ok: "削除",
  };
}

export type WorkspaceDeleteState = WorkspaceDeleteDialogState & { error: string | null };

export interface WorkspaceDeleteDeps {
  actions: Pick<WorkspacesPort, "workspaceDeletePreview" | "deleteWorkspace">;
  catalog: Catalog;
  toasts: Toasts;
  // afterDelete は、削除できたあと。app が、ホームへ移し、サイドバーの一覧を読み直す。
  afterDelete(ws: DeleteWorkspaceTarget): Promise<void>;
}

export interface WorkspaceDelete {
  state: Readable<WorkspaceDeleteState | null>;
  // open は、予告を読んでから確認ダイアログを出す。閲覧のみの Store では何もしない。
  open(ws: DeleteWorkspaceTarget): Promise<void>;
  // confirm は、入力の前後の空白を除いた値が名前と一致するときだけ削除する。一致しないときは error を入れる。
  confirm(typed: string): Promise<void>;
  // clearError は、入力が変わったときに、一致しないという表示を消す。
  clearError(): void;
  cancel(): void;
}

export function createWorkspaceDelete(deps: WorkspaceDeleteDeps): WorkspaceDelete {
  const state = writable<WorkspaceDeleteState | null>(null);
  // seq は、開く・閉じるの世代。古い予告の結果は捨てる。
  let seq = 0;
  // busy は、削除の実行中。そのあいだの confirm は受けない。
  let busy = false;

  const failed = (e: unknown) =>
    deps.toasts.show(e instanceof Refused ? e.message : itemDeleteFailedMessage, { error: true });

  return {
    state: readonly(state),
    async open(ws) {
      if (deps.catalog.store(ws.storeId)?.readOnly) return;
      const gen = ++seq;
      try {
        const p = await deps.actions.workspaceDeletePreview(ws.storeId, ws.id);
        if (gen === seq) state.set({ ...workspaceDeleteDialog(ws, p), error: null });
      } catch (e) {
        if (gen !== seq) return;
        // すでに消えていたら、確認なしで移る・読み直す。
        if (e instanceof NotFound) await deps.afterDelete(ws);
        else failed(e);
      }
    },
    async confirm(typed) {
      const s = get(state);
      if (!s || busy) return;
      if (typed.trim() !== s.workspace.name) {
        state.set({ ...s, error: nameMismatch });
        return;
      }
      busy = true;
      try {
        await deps.actions.deleteWorkspace(s.workspace.storeId, s.workspace.id);
        state.set(null);
        deps.toasts.show(workspaceDeletedMessage(s.workspace.name));
      } catch (e) {
        state.set(null);
        // すでに消えているのは、消した結果と同じなので、トーストなしで移る。
        if (!(e instanceof NotFound)) return failed(e);
      } finally {
        busy = false;
      }
      await deps.afterDelete(s.workspace);
    },
    clearError() {
      const s = get(state);
      if (s?.error) state.set({ ...s, error: null });
    },
    cancel() {
      seq += 1;
      state.set(null);
    },
  };
}

export interface LeaveDeletedWorkspaceDeps {
  navigation: Pick<Navigation, "route" | "replace">;
  catalog: Pick<Catalog, "stores" | "load">;
  detail: { view: Readable<Pick<DetailView, "item"> | null>; close(): void };
}

// leaveDeletedWorkspace は、削除できたあとの移り先(B5b-12)。いまの画面がそのワークスペースのもの(概要・設定・
// 中のプロジェクト・アイテム)ならホームへ移り(履歴には積まない)、開いている詳細パネルがそのワークスペースの
// アイテムなら閉じ、サイドバーの一覧を読み直す。先に移るのは、一覧を先に読み直すと、消えたアドレスがホームに
// 解決されるため。読み直しの失敗は、削除はできているので投げない(次の同期の読み直しで直る)。
export async function leaveDeletedWorkspace(
  deps: LeaveDeletedWorkspaceDeps,
  ws: DeleteWorkspaceTarget,
): Promise<void> {
  const route = get(deps.navigation.route);
  const store = get(deps.catalog.stores).find((s) => s.id === ws.storeId);
  if (store && "store" in route && route.store === store.name) {
    const inside =
      route.name === "workspace"
        ? route.prefix === ws.prefix
        : (route.name === "project" ? route.project : route.item).startsWith(`${ws.prefix}-`);
    if (inside) deps.navigation.replace({ name: "home" });
  }
  const open = get(deps.detail.view)?.item;
  if (open?.storeId === ws.storeId && open.workspaceId === ws.id) deps.detail.close();
  await deps.catalog.load().catch(() => {});
}
