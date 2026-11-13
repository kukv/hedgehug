import { NotFound, Refused } from "../../model/errors";
import type { Member } from "../../model/member";
import type {
  ValueList,
  WorkspaceAutomation,
  WorkspaceDetail,
  WorkspaceUsage,
} from "../../model/workspace";
import type { WorkspacesPort } from "../../port/workspaces";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Catalog } from "../catalog/catalog";
import type { Toasts } from "../toasts/toasts";
import { prefixRefusal } from "./general";
import { deleteRefusal } from "./values";

// workspaceSaveFailed は、別の画面でワークスペースが削除されたあとの保存(API の 404)のトースト。
export const workspaceSaveFailed = "保存できませんでした";

// ワークスペースの設定の画面の状態(03「範囲と画面の構成」)。
// 操作は run に渡す。断りはエラーのトーストにして、設定を読み直し、入力を元に戻す。

export type WorkspaceSettingsTab =
  | "general"
  | "properties"
  | "statuses"
  | "priorities"
  | "systems"
  | "workflow"
  | "templates"
  | "automation";

export const workspaceSettingsTabs: { id: WorkspaceSettingsTab; label: string }[] = [
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
  tab: WorkspaceSettingsTab;
  detail: WorkspaceDetail;
  usage: WorkspaceUsage;
  // Store のメンバー。テンプレートの担当者の選択肢(B3-19)。
  members: Member[];
  // 自動化のサブタブを開いているときだけ読む(B4-22)。
  automation: WorkspaceAutomation | null;
}

export interface WorkspaceSettingsScreen {
  state: Readable<SettingsState | null>;
  // 同じワークスペースなら、サブタブと state はそのままで読み直す(戻ったときに件数と ID の例を新しくする)。
  // 別のワークスペースなら、サブタブは一般にして、state を消してから読み込む。
  open(storeId: string, workspaceId: string): Promise<void>;
  // 履歴に積まない(01)。
  selectTab(tab: WorkspaceSettingsTab): void;
  reload(): Promise<void>;
  // 確認ダイアログを開く前の検査。断るときはエラーのトーストを出して false を返す(ダイアログは開かない)。
  askPrefix(prefix: string): boolean;
  askDelete(list: ValueList, valueId: string): boolean;
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
  listMembers(storeId: string): Promise<Member[]>;
}): WorkspaceSettingsScreen {
  const { port, catalog, toasts, listMembers } = deps;
  const state = writable<SettingsState | null>(null);
  let current: { storeId: string; workspaceId: string } | null = null;
  let tab: WorkspaceSettingsTab = "general";
  // 読み直しが重なったときは、最後に始めたものの結果だけを使う。
  let latest = 0;

  async function reload() {
    if (!current) return;
    const { storeId, workspaceId } = current;
    latest += 1;
    const mine = latest;
    const [detail, usage, members, automation] = await Promise.all([
      port.get(storeId, workspaceId),
      port.usage(storeId, workspaceId),
      listMembers(storeId),
      tab === "automation" ? port.automation(storeId, workspaceId) : null,
    ]);
    if (mine !== latest) return;
    state.set({
      storeId,
      readOnly: catalog.store(storeId)?.readOnly ?? false,
      tab,
      detail,
      usage,
      members,
      automation,
    });
  }

  function allowed(refusal: string | null): boolean {
    if (refusal !== null) toasts.show(refusal, { error: true });
    return refusal === null;
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
      if (next === "automation") void reload();
    },
    reload,
    askPrefix(prefix) {
      const others = (current && catalog.store(current.storeId)?.workspaces) || [];
      const refusal = prefixRefusal(
        prefix,
        others.filter((w) => w.id !== current?.workspaceId).map((w) => w.prefix),
      );
      return allowed(refusal);
    },
    askDelete(list, valueId) {
      const settings = get(state)?.detail.settings;
      return allowed(settings ? deleteRefusal(settings, list, valueId) : null);
    },
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
        if (e instanceof NotFound) toasts.show(workspaceSaveFailed, { error: true });
        else if (!(e instanceof Refused)) throw e;
        else if (onRefused) onRefused(e.message);
        else toasts.show(e.message, { error: true });
      } finally {
        // 読み直しを先にする。サイドバーの読み直しが投げても、設定の読み直しを飛ばさない。
        // 削除済みのワークスペースの読み直しは 404 になるが、トーストを出しているので、画面は前の表示のままにする。
        await reload().catch((e) => {
          if (!(e instanceof NotFound)) throw e;
        });
        if (catalogChanged) await catalog.load();
      }
      return succeeded;
    },
  };
}
