import { Refused } from "../../model/errors";
import type { Project, ProjectStatus } from "../../model/project";
import type { CatalogPort } from "../../port/catalog";
import type { ProjectsPort } from "../../port/projects";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import { type Catalog, writableWorkspaces } from "../catalog/catalog";

// プロジェクトの作成ダイアログ(02「プロジェクト」の「作成」)。検査はサーバーに任せ、断りの文言を赤字で出す
// (A2 の計画の決定 14)。作ったら一覧を読み直し、onCreated でそのプロジェクトの概要を開く。

export interface Option {
  value: string;
  label: string;
}

export interface ProjectForm {
  workspaceId: string;
  title: string;
  owner: string;
  status: ProjectStatus;
  start: string;
  end: string;
}

export interface ProjectDialogState {
  form: ProjectForm;
  // workspaces は「Store名 / ワークスペース名」の選択肢(閲覧のみの Store を除く)。
  workspaces: Option[];
  // owners は「なし」とメンバー。
  owners: Option[];
  error: string;
}

export interface ProjectDialog {
  state: Readable<ProjectDialogState | null>;
  // open は、workspaceId のワークスペースで開く。書けないか省いたときは、書ける先頭のワークスペース。
  open(workspaceId?: string): Promise<void>;
  // update は、入力を変える。ワークスペースを変えたら、オーナーの選択肢とあなたを読み直す。
  update(patch: Partial<ProjectForm>): Promise<void>;
  submit(): Promise<void>;
  close(): void;
}

export interface ProjectDialogDeps {
  catalog: Catalog;
  catalogPort: CatalogPort;
  projects: ProjectsPort;
  onCreated(storeId: string, project: Project): void;
}

export function createProjectDialog(deps: ProjectDialogDeps): ProjectDialog {
  const state = writable<ProjectDialogState | null>(null);

  const storeOf = (workspaceId: string) =>
    get(deps.catalog.stores).find((s) => s.workspaces.some((w) => w.id === workspaceId));

  // owners は、ワークスペースの Store のメンバーと、あなた(決まらなければ空 = なし。A2 の計画の決定 3)。
  async function owners(workspaceId: string): Promise<{ options: Option[]; me: string }> {
    const store = storeOf(workspaceId);
    if (!store) return { options: [{ value: "", label: "なし" }], me: "" };
    const [members, me] = await Promise.all([
      deps.catalogPort.listMembers(store.id),
      deps.catalogPort.me(store.id).catch(() => ""),
    ]);
    const options = [
      { value: "", label: "なし" },
      ...members.map((m) => ({ value: m.login, label: m.name })),
    ];
    return { options, me };
  }

  function set(patch: Partial<ProjectDialogState>) {
    state.update((s) => (s ? { ...s, ...patch } : s));
  }

  return {
    state: readonly(state),
    async open(workspaceId) {
      const choices = writableWorkspaces(get(deps.catalog.stores));
      const ws = choices.find((w) => w.id === workspaceId) ?? choices[0];
      if (!ws) return;
      const labels = get(deps.catalog.stores)
        .filter((s) => !s.readOnly)
        .flatMap((s) => s.workspaces.map((w) => ({ value: w.id, label: `${s.name} / ${w.name}` })));
      const o = await owners(ws.id);
      state.set({
        form: { workspaceId: ws.id, title: "", owner: o.me, status: "Todo", start: "", end: "" },
        workspaces: labels,
        owners: o.options,
        error: "",
      });
    },
    async update(patch) {
      const s = get(state);
      if (!s) return;
      const form = { ...s.form, ...patch };
      if (patch.workspaceId === undefined || patch.workspaceId === s.form.workspaceId) {
        set({ form });
        return;
      }
      set({ form });
      // 別の Store のワークスペースに替えたら、その Store のメンバーと、その Store でのあなたにする。
      if (storeOf(patch.workspaceId)?.id === storeOf(s.form.workspaceId)?.id) return;
      const o = await owners(patch.workspaceId);
      state.update((cur) =>
        cur ? { ...cur, owners: o.options, form: { ...cur.form, owner: o.me } } : cur,
      );
    },
    async submit() {
      const s = get(state);
      const store = s && storeOf(s.form.workspaceId);
      if (!s || !store) return;
      const { workspaceId, ...input } = s.form;
      try {
        const created = await deps.projects.create(store.id, workspaceId, input);
        state.set(null);
        await deps.catalog.load();
        deps.onCreated(store.id, created);
      } catch (e) {
        if (!(e instanceof Refused)) throw e;
        set({ error: e.message });
      }
    },
    close() {
      state.set(null);
    },
  };
}
