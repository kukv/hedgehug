import { Refused } from "../../model/errors";
import type { WorkspaceDetail, WorkspaceUsage } from "../../model/workspace";
import type { WorkspacesPort } from "../../port/workspaces";
import { type Readable, readonly, writable } from "../../reactive/store";
import type { Catalog } from "../catalog/catalog";
import type { Toasts } from "../toasts/toasts";

// ワークスペースの設定の画面の状態(03「範囲と画面の構成」)。
// 操作は run に渡す。断りはエラーのトーストにして、設定を読み直し、入力を元に戻す。

export type SettingsTab =
  | "general"
  | "properties"
  | "statuses"
  | "priorities"
  | "systems"
  | "workflow"
  | "templates"
  | "automation";

export const settingsTabs: { id: SettingsTab; label: string }[] = [
  { id: "general", label: "一般" },
  { id: "properties", label: "プロパティ" },
  { id: "statuses", label: "ステータス" },
  { id: "priorities", label: "優先度" },
  { id: "systems", label: "関連システム" },
  { id: "workflow", label: "ワークフロー" },
  { id: "templates", label: "テンプレート" },
  { id: "automation", label: "自動化" },
];

export interface SettingsState {
  storeId: string;
  readOnly: boolean;
  tab: SettingsTab;
  detail: WorkspaceDetail;
  usage: WorkspaceUsage;
}

export interface WorkspaceSettingsScreen {
  state: Readable<SettingsState | null>;
  // 同じワークスペースならサブタブを保つ。別ならサブタブは一般。
  open(storeId: string, workspaceId: string): Promise<void>;
  // 履歴に積まない(01)。
  selectTab(tab: SettingsTab): void;
  reload(): Promise<void>;
  // run は操作を 1 つ行い、読み直す。断りはエラーのトーストにして false を返す。
  // ok を渡したら、成功のときにトーストを出す。
  // onRefused を渡したら、断りをトーストにせず、その文言を渡す(ダイアログの中の赤字用)。
  run(
    op: (port: WorkspacesPort, storeId: string, workspaceId: string) => Promise<unknown>,
    ok?: string,
    onRefused?: (message: string) => void,
  ): Promise<boolean>;
}

export function createWorkspaceSettings(deps: {
  port: WorkspacesPort;
  catalog: Catalog;
  toasts: Toasts;
}): WorkspaceSettingsScreen {
  const { port, catalog, toasts } = deps;
  const state = writable<SettingsState | null>(null);
  let current: { storeId: string; workspaceId: string } | null = null;
  let tab: SettingsTab = "general";
  // 読み直しが重なったときは、最後に始めたものの結果だけを使う。
  let latest = 0;

  async function reload() {
    if (!current) return;
    const { storeId, workspaceId } = current;
    latest += 1;
    const mine = latest;
    const [detail, usage] = await Promise.all([
      port.get(storeId, workspaceId),
      port.usage(storeId, workspaceId),
    ]);
    if (mine !== latest) return;
    state.set({
      storeId,
      readOnly: catalog.store(storeId)?.readOnly ?? false,
      tab,
      detail,
      usage,
    });
  }

  // 名前・接頭辞・並びを変えたら、サイドバーも読み直す。
  function watching(onCatalogChange: () => void): WorkspacesPort {
    const touch =
      <A extends unknown[]>(fn: (...a: A) => Promise<void>) =>
      (...a: A) => {
        onCatalogChange();
        return fn(...a);
      };
    return {
      ...port,
      rename: touch(port.rename),
      changePrefix: touch(port.changePrefix),
      move: touch(port.move),
    };
  }

  return {
    state: readonly(state),
    async open(storeId, workspaceId) {
      if (current?.storeId !== storeId || current.workspaceId !== workspaceId) {
        tab = "general";
        // 古いワークスペースを、新しいアドレスの下に出さない。
        state.set(null);
      }
      current = { storeId, workspaceId };
      await reload();
    },
    selectTab(next) {
      tab = next;
      state.update((s) => (s ? { ...s, tab } : s));
    },
    reload,
    async run(op, ok, onRefused) {
      if (!current) return false;
      const { storeId, workspaceId } = current;
      let catalogChanged = false;
      let succeeded = false;
      try {
        await op(
          watching(() => {
            catalogChanged = true;
          }),
          storeId,
          workspaceId,
        );
        succeeded = true;
        if (ok) toasts.show(ok);
      } catch (e) {
        if (!(e instanceof Refused)) throw e;
        if (onRefused) onRefused(e.message);
        else toasts.show(e.message, { error: true });
      } finally {
        // 読み直しを先にする。サイドバーの読み直しが投げても、設定の読み直しを飛ばさない。
        await reload();
        if (catalogChanged) await catalog.load();
      }
      return succeeded;
    },
  };
}
