import { NotFound, Refused } from "../../model/errors";
import type { ProjectDeletePreview } from "../../model/itemAction";
import type { Project } from "../../model/project";
import type { ItemActionsPort } from "../../port/itemActions";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Catalog } from "../catalog/catalog";
import { itemDeleteFailedMessage } from "../items/delete";
import type { Navigation } from "../router/navigation";
import { routeOfWorkspace } from "../router/resolve";
import type { Toasts } from "../toasts/toasts";

// プロジェクトの削除(02「名前の変更・移動・削除」の削除)。設定タブの「削除」の区画の説明と、確認ダイアログ。
// アイテムの削除(items/delete.ts)と同じ形で、文言が違う。

export const projectDeletedMessage = "プロジェクトを削除しました";

// DeleteTarget は、削除するプロジェクト(ProjectNode も渡せる)。
export type DeleteTarget = Pick<Project, "id" | "storeId" | "workspaceId" | "title">;

const restore = (p: ProjectDeletePreview) =>
  `設定の「削除済み」から戻せます。${p.retentionDays} 日を過ぎると完全に削除されます。`;

// projectDeleteNote は、設定の「削除」の区画の説明(02「画面の見出しと構成」)。
export function projectDeleteNote(p: ProjectDeletePreview): string {
  return `プロジェクトと、その下の ${p.items} 件のアイテムを削除します。${restore(p)}`;
}

export interface ProjectDeleteDialogState {
  project: DeleteTarget;
  title: string;
  lines: string[];
  ok: string;
}

// projectDeleteDialog は、確認ダイアログの中身(02の表)。
export function projectDeleteDialog(
  project: DeleteTarget,
  p: ProjectDeletePreview,
): ProjectDeleteDialogState {
  return {
    project,
    title: "プロジェクトを削除",
    lines: [`「${project.title}」と、その下の ${p.items} 件のアイテムを削除します。`, restore(p)],
    ok: "削除",
  };
}

export interface LeaveDeletedDeps {
  navigation: Pick<Navigation, "replace">;
  catalog: Pick<Catalog, "stores" | "load">;
}

// leaveDeletedProject は、削除できたあとの移り先。ワークスペースの概要へ移り(履歴には積まない。戻るで削除済みの
// アドレスに戻らない)、サイドバーの一覧を読み直す。先に移るのは、一覧を先に読み直すと、消えたプロジェクトの
// アドレスがホームに解決されるため。読み直しの失敗は、削除はできているので投げない(次の同期の読み直しで直る)。
export async function leaveDeletedProject(
  deps: LeaveDeletedDeps,
  project: DeleteTarget,
): Promise<void> {
  deps.navigation.replace(
    routeOfWorkspace(get(deps.catalog.stores), project.storeId, project.workspaceId, "overview"),
  );
  await deps.catalog.load().catch(() => {});
}

export interface ProjectDeleteDeps {
  actions: Pick<ItemActionsPort, "projectDeletePreview" | "deleteProject">;
  catalog: Catalog;
  toasts: Toasts;
  // afterDelete は、削除できたあと。app が、ワークスペースの概要へ移し、サイドバーの一覧を読み直す。
  afterDelete(project: DeleteTarget): Promise<void>;
}

export interface ProjectDelete {
  // note は、設定の「削除」の区画の説明。load で読んだプロジェクトのもの(読めていなければ null)。
  note: Readable<{ projectId: string; text: string } | null>;
  load(project: DeleteTarget): Promise<void>;
  state: Readable<ProjectDeleteDialogState | null>;
  // open は、予告を読んでから確認ダイアログを出す。閲覧のみの Store では何もしない。
  open(project: DeleteTarget): Promise<void>;
  confirm(): Promise<void>;
  cancel(): void;
}

export function createProjectDelete(deps: ProjectDeleteDeps): ProjectDelete {
  const note = writable<{ projectId: string; text: string } | null>(null);
  const state = writable<ProjectDeleteDialogState | null>(null);
  // seq は、開く・閉じるの世代。noteSeq は、説明を読む世代。古い結果は捨てる。
  let seq = 0;
  let noteSeq = 0;
  // busy は、削除の実行中。そのあいだの confirm は受けない。
  let busy = false;

  const failed = (e: unknown) =>
    deps.toasts.show(e instanceof Refused ? e.message : itemDeleteFailedMessage, { error: true });

  return {
    note: readonly(note),
    state: readonly(state),
    async load(project) {
      const gen = ++noteSeq;
      try {
        const p = await deps.actions.projectDeletePreview(project.storeId, project.id);
        if (gen === noteSeq) note.set({ projectId: project.id, text: projectDeleteNote(p) });
      } catch {
        // 読めなかったときは、説明を出さない(ボタンを押すと、確認ダイアログの予告をもう一度読む)。
        if (gen === noteSeq) note.set(null);
      }
    },
    async open(project) {
      if (deps.catalog.store(project.storeId)?.readOnly) return;
      const gen = ++seq;
      try {
        const p = await deps.actions.projectDeletePreview(project.storeId, project.id);
        if (gen === seq) state.set(projectDeleteDialog(project, p));
      } catch (e) {
        if (gen !== seq) return;
        // すでに消えていたら、確認なしで移る・読み直す。
        if (e instanceof NotFound) await deps.afterDelete(project);
        else failed(e);
      }
    },
    async confirm() {
      const s = get(state);
      if (!s || busy) return;
      busy = true;
      try {
        await deps.actions.deleteProject(s.project.storeId, s.project.id);
        state.set(null);
        deps.toasts.show(projectDeletedMessage);
      } catch (e) {
        state.set(null);
        // すでに消えているのは、消した結果と同じなので、トーストなしで移る。
        if (!(e instanceof NotFound)) return failed(e);
      } finally {
        busy = false;
      }
      await deps.afterDelete(s.project);
    },
    cancel() {
      seq += 1;
      state.set(null);
    },
  };
}
