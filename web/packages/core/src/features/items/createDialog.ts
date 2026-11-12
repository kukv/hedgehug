import { Refused } from "../../model/errors";
import type { Item, Kind } from "../../model/item";
import type { WorkspaceSettings } from "../../model/workspace";
import type { CatalogPort } from "../../port/catalog";
import type { ItemsPort } from "../../port/items";
import type { WorkspacesPort } from "../../port/workspaces";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Catalog } from "../catalog/catalog";
import type { Option } from "../projects/createDialog";
import { itemDisplayId } from "../router/route";
import type { Toasts } from "../toasts/toasts";
import {
  type CreateContext,
  type Here,
  parentCandidates,
  parentMissing,
  pickParent,
  startValues,
} from "./createDefaults";
import { createContextLoader, rowsFor, type WorkspaceContext } from "./createDialogLoad";
import { initialValues, shownValues, switchWorkspace, type Values } from "./createValues";
import type { PropertyRow } from "./properties";

// アイテムの作成ダイアログ(02「アイテムの作成」のうち A2 の分)。検査はサーバーに任せ、断りを赤字で出す
// (A2 の計画の決定 14)。プロパティの値は A3 前半(決定 25)。テンプレート・スプリント・説明は A3 後半。

export interface ItemForm {
  workspaceId: string;
  projectId: string;
  kind: Kind;
  parentId: string | null;
  title: string;
  keepOpen: boolean;
  // values は、プロパティの内部 ID → 入力。隠した欄の値も覚えておく。
  values: Values;
}

export interface ItemDialogState {
  form: ItemForm;
  workspaces: Option[];
  projects: Option[];
  parents: Option[];
  // parentMissing は、親の候補がないときの赤字。Epic と、候補があるときは空。
  parentMissing: string;
  // preview は「ID は HH-13 になります。」。
  preview: string;
  // rows は、種類で使うプロパティの行。settings は、そのワークスペースの設定。
  rows: PropertyRow[];
  settings: WorkspaceSettings;
  error: string;
}

export interface ItemDialog {
  state: Readable<ItemDialogState | null>;
  open(ctx?: CreateContext): Promise<void>;
  update(patch: Partial<ItemForm>): Promise<void>;
  submit(): Promise<void>;
  close(): void;
}

export interface ItemDialogDeps {
  catalog: Catalog;
  items: ItemsPort;
  workspaces: WorkspacesPort;
  catalogPort: CatalogPort;
  toasts: Toasts;
  here(): Here;
  openProjectDialog(workspaceId: string): void;
  // onCreated は、作ったあと。keptOpen は「続けて作成」がオンだったか。
  onCreated(item: Item, keptOpen: boolean): void;
}

export const kinds: Kind[] = ["Epic", "Story", "Task", "Bug", "Step"];

export function createItemDialog(deps: ItemDialogDeps): ItemDialog {
  const state = writable<ItemDialogState | null>(null);
  // projectItems は、選んでいるプロジェクトのアイテム(親の候補を作る)。
  let projectItems: Item[] = [];
  // seq は、読み直しの世代。待っているあいだに開き直し・閉じ・別の読み直しがあったら、古い結果を捨てる。
  let seq = 0;
  // busy は、送信中。そのあいだは submit を受けない。
  let busy = false;
  // load は、ワークスペースの設定とメンバーを読む(ダイアログを開いているあいだだけ覚える)。current は、いま出している分。
  let load = createContextLoader(deps.workspaces, deps.catalogPort);
  let current: WorkspaceContext | null = null;

  const stores = () => get(deps.catalog.stores);
  const storeOf = (wsId: string) => stores().find((s) => s.workspaces.some((w) => w.id === wsId));
  const wsOf = (wsId: string) => storeOf(wsId)?.workspaces.find((w) => w.id === wsId);
  const wsOptions = () =>
    stores()
      .filter((s) => !s.readOnly)
      .flatMap((s) =>
        s.workspaces.map((w) => ({ value: w.id, label: `${s.name} / ${w.name} (${w.prefix})` })),
      );

  const parentOptions = (prefix: string, candidates: Item[]) =>
    candidates.map((it) => ({
      value: it.id,
      label: `${itemDisplayId(prefix, it.number)} ${it.title}`,
    }));
  const missing = (kind: Kind, candidates: Item[]) =>
    kind !== "Epic" && candidates.length === 0 ? parentMissing(kind) : "";

  // refresh は、ワークスペース・プロジェクト・種類から、選択肢・親・ID の予告を作り直す。
  async function refresh(
    form: ItemForm,
    error = "",
  ): Promise<{ next: ItemDialogState; ctx: WorkspaceContext } | null> {
    const store = storeOf(form.workspaceId);
    const ws = wsOf(form.workspaceId);
    if (!store || !ws) return null;
    const [list, nextNo, ctx] = await Promise.all([
      form.projectId ? deps.items.list(store.id, form.projectId) : Promise.resolve([]),
      deps.items.nextNumber(store.id, ws.id),
      load(store.id, ws.id),
    ]);
    projectItems = list;
    const candidates = parentCandidates(list, form.kind);
    const parentId = form.kind === "Epic" ? null : pickParent(candidates, form.parentId);
    const shown = { ...form, parentId };
    const next: ItemDialogState = {
      form: shown,
      workspaces: wsOptions(),
      projects: ws.projects.map((p) => ({ value: p.id, label: p.title })),
      parents: parentOptions(ws.prefix, candidates),
      parentMissing: missing(form.kind, candidates),
      preview: `ID は ${itemDisplayId(ws.prefix, nextNo)} になります。`,
      rows: rowsFor(ctx, shown),
      settings: ctx.settings,
      error,
    };
    return { next, ctx };
  }

  // memberCheck は、ワークスペースを変えたあとも担当者を残せるか。同じ Store なら残し、別の Store ならメンバーにいるときだけ。
  const memberCheck = (from: string, to: string, ctx: WorkspaceContext) => (login: string) =>
    storeOf(from)?.id === storeOf(to)?.id ||
    ctx.people.members.some((m) => m.login.toLowerCase() === login.toLowerCase());

  // reloaded は、読み直した結果を出す。待っているあいだに入力されたタイトル・「続けて作成」・値は残す
  // (決定 27)。ワークスペースが変わったときは、そのいまの値に新しいワークスペースへの切り替えを当てる。
  async function reloaded(form: ItemForm, error = "") {
    const gen = ++seq;
    const loaded = await refresh(form, error);
    const now = get(state);
    if (gen !== seq || !now) return;
    if (!loaded) return void state.set(null);
    const { title, keepOpen } = now.form;
    const values =
      now.form.workspaceId === form.workspaceId
        ? now.form.values
        : switchWorkspace(
            now.form.values,
            loaded.ctx.settings,
            memberCheck(now.form.workspaceId, form.workspaceId, loaded.ctx),
          );
    const kept = { ...loaded.next.form, title, keepOpen, values };
    current = loaded.ctx;
    state.set({ ...loaded.next, form: kept, rows: rowsFor(loaded.ctx, kept) });
  }

  // send は、アイテムを作る。作ったあと、「続けて作成」ならタイトルを空にして読み直し、でなければ閉じる。
  async function send(s: ItemDialogState) {
    const store = storeOf(s.form.workspaceId);
    const ws = wsOf(s.form.workspaceId);
    if (!store || !ws) return;
    const { projectId, kind, title } = s.form;
    const parentId = kind === "Epic" ? null : s.form.parentId;
    let created: Item;
    try {
      created = await deps.items.create(store.id, {
        projectId,
        parentId,
        kind,
        title,
        values: shownValues(s.form.values, s.settings, kind),
      });
    } catch (e) {
      if (!(e instanceof Refused)) throw e;
      // 送っているあいだに入力された分を残す(決定 27)。
      const now = get(state);
      if (now) state.set({ ...now, error: e.message });
      return;
    }
    deps.toasts.show(`${itemDisplayId(ws.prefix, created.number)} を作成しました`);
    if (s.form.keepOpen) {
      // 続けて作成: タイトルとエラーだけを空にし、ほかは残す(02「作成したあと」)。
      const now = get(state);
      if (now) {
        const form = { ...now.form, title: "" };
        state.set({ ...now, form, error: "" });
        await reloaded(form);
      }
    } else {
      seq += 1;
      state.set(null);
    }
    deps.onCreated(created, s.form.keepOpen);
  }

  return {
    state: readonly(state),
    async open(ctx = {}) {
      const start = startValues(stores(), ctx, deps.here());
      if (!start.ok) {
        deps.toasts.show(start.message, { error: true });
        if (start.workspaceWithoutProject) deps.openProjectDialog(start.workspaceWithoutProject.id);
        return;
      }
      const form: ItemForm = {
        workspaceId: start.workspace.id,
        projectId: start.projectId,
        kind: start.kind,
        parentId: start.parentId,
        title: "",
        keepOpen: false,
        values: {},
      };
      load = createContextLoader(deps.workspaces, deps.catalogPort);
      const gen = ++seq;
      const store = storeOf(form.workspaceId);
      if (!store) return;
      const loadedCtx = await load(store.id, form.workspaceId);
      form.values = initialValues(loadedCtx.settings);
      const loaded = await refresh(form);
      if (gen !== seq) return;
      current = loaded?.ctx ?? null;
      state.set(loaded?.next ?? null);
    },
    async update(patch) {
      const s = get(state);
      if (!s) return;
      const form = { ...s.form, ...patch };
      if (patch.workspaceId !== undefined && patch.workspaceId !== s.form.workspaceId) {
        // ワークスペースを変えたら、プロジェクトを先頭にし、親を選び直す(02「ダイアログの中での連動」)。
        form.projectId = wsOf(patch.workspaceId)?.projects[0]?.id ?? "";
        await reloaded(form, s.error);
        return;
      }
      if (patch.projectId !== undefined && patch.projectId !== s.form.projectId) {
        await reloaded(form, s.error);
        return;
      }
      if (patch.kind !== undefined && patch.kind !== s.form.kind) {
        // 種類を変えたら、親を新しい種類の候補から選び直す。タイトルは残す。
        const candidates = parentCandidates(projectItems, form.kind);
        const ws = wsOf(form.workspaceId);
        form.parentId = form.kind === "Epic" ? null : pickParent(candidates, form.parentId);
        state.set({
          ...s,
          form,
          parents: parentOptions(ws?.prefix ?? "", candidates),
          parentMissing: missing(form.kind, candidates),
          rows: current ? rowsFor(current, form) : s.rows,
        });
        return;
      }
      const rows = patch.values && current ? rowsFor(current, form) : s.rows;
      state.set({ ...s, form, rows });
    },
    async submit() {
      const s = get(state);
      if (!s || busy) return;
      busy = true;
      try {
        await send(s);
      } finally {
        busy = false;
      }
    },
    close() {
      seq += 1;
      state.set(null);
    },
  };
}
