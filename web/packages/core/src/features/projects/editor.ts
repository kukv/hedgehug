import { Refused } from "../../model/errors";
import type { Project, ProjectStatus } from "../../model/project";
import type { ProjectsPort } from "../../port/projects";
import type { Toasts } from "../toasts/toasts";

// プロジェクトの概要の編集(02「概要の編集」「リポジトリの追加と削除」)。検査と断りの文言はサーバーが持つので、
// ここは、値が前と同じなら送らないことと、断りの出し方(トーストか、入力欄の下か)だけを決める。

export interface ProjectEditor {
  // 次の 4 つは、保存できたら true。false なら、画面は入力欄を保存済みの値に戻す(断りはトーストに出す)。
  rename(project: Project, title: string): Promise<boolean>;
  setStatus(project: Project, status: ProjectStatus): Promise<boolean>;
  setOwner(project: Project, owner: string): Promise<boolean>;
  setDate(project: Project, field: "start" | "end", value: string): Promise<boolean>;
  // addRepo は、追加できたら ""。断られたら、その文言(入力欄の下に出す。トーストは出さない)を返す。
  addRepo(project: Project, input: string): Promise<string>;
  // removeRepo は、確認なしで外し、トースト「{owner/name} を削除しました」を出す。
  removeRepo(project: Project, repo: string): Promise<void>;
}

export interface ProjectEditorDeps {
  projects: ProjectsPort;
  toasts: Toasts;
  // onChanged は、保存できたあと(一覧と、開いているプロジェクトの読み直し)。
  onChanged(): Promise<void>;
}

export function createProjectEditor(deps: ProjectEditorDeps): ProjectEditor {
  // attempt は、保存を 1 回行う。断られたらその文言を返し(読み直さない)、保存できたら読み直して null を返す。
  // 断り以外のエラーはそのまま投げる。
  async function attempt(save: () => Promise<unknown>): Promise<string | null> {
    try {
      await save();
    } catch (e) {
      if (e instanceof Refused) return e.message;
      throw e;
    }
    await deps.onChanged();
    return null;
  }

  // attemptWithToast は、断りをエラーのトーストで出す。保存できたら true。
  async function attemptWithToast(save: () => Promise<unknown>): Promise<boolean> {
    const message = await attempt(save);
    if (message !== null) deps.toasts.show(message, { error: true });
    return message === null;
  }

  return {
    async rename(p, title) {
      const next = title.trim();
      if (next === p.title) return true;
      return attemptWithToast(() => deps.projects.rename(p.storeId, p.id, next));
    },
    async setStatus(p, status) {
      if (status === p.status) return true;
      return attemptWithToast(() => deps.projects.setStatus(p.storeId, p.id, status));
    },
    async setOwner(p, owner) {
      if (owner === p.owner) return true;
      return attemptWithToast(() => deps.projects.setOwner(p.storeId, p.id, owner));
    },
    async setDate(p, field, value) {
      if (value === p[field]) return true;
      return attemptWithToast(() => deps.projects.setDate(p.storeId, p.id, field, value));
    },
    async addRepo(p, input) {
      return (await attempt(() => deps.projects.addRepo(p.storeId, p.id, input))) ?? "";
    },
    async removeRepo(p, repo) {
      if (await attemptWithToast(() => deps.projects.removeRepo(p.storeId, p.id, repo))) {
        deps.toasts.show(`${repo} を削除しました`);
      }
    },
  };
}
