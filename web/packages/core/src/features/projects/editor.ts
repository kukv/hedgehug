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

// Field は、概要で 1 つずつ保存する項目。
type Field = "title" | "status" | "owner" | "start" | "end";

export interface ProjectEditorDeps {
  projects: ProjectsPort;
  toasts: Toasts;
  // onChanged は、保存できたあと(一覧と、開いているプロジェクトの読み直し)。
  onChanged(): Promise<void>;
}

export function createProjectEditor(deps: ProjectEditorDeps): ProjectEditor {
  // 保存のあとの読み直しが終わるまで、画面のプロジェクトは保存の前の値のままになる。そのあいだは、この画面で保存した
  // 値(saved)と比べる(保存の前の値と比べると、保存した値をすぐ元に戻す変更が「同じ」に見えて送られない)。
  // reloading は、終わっていない読み直しの数。すべて終わったら saved を捨てる。
  let reloading = 0;
  const saved = new Map<string, string>();
  const keyOf = (p: Project, field: Field) => `${p.storeId}/${p.id}/${field}`;
  const same = (p: Project, field: Field, value: string) => {
    const key = keyOf(p, field);
    return value === (reloading > 0 && saved.has(key) ? saved.get(key) : p[field]);
  };

  // attempt は、保存を 1 回行う。断られたらその文言を返し(読み直さない)、保存できたら done を呼び、読み直して null を
  // 返す。断り以外のエラーはそのまま投げる。
  async function attempt(save: () => Promise<unknown>, done = () => {}): Promise<string | null> {
    try {
      await save();
    } catch (e) {
      if (e instanceof Refused) return e.message;
      throw e;
    }
    done();
    reloading += 1;
    try {
      await deps.onChanged();
    } finally {
      reloading -= 1;
      if (reloading === 0) saved.clear();
    }
    return null;
  }

  // attemptWithToast は、断りをエラーのトーストで出す。保存できたら true。
  async function attemptWithToast(save: () => Promise<unknown>, done?: () => void) {
    const message = await attempt(save, done);
    if (message !== null) deps.toasts.show(message, { error: true });
    return message === null;
  }

  // setField は、値が前と同じなら送らず、違えば保存する。
  function setField(p: Project, field: Field, value: string, save: () => Promise<unknown>) {
    if (same(p, field, value)) return Promise.resolve(true);
    return attemptWithToast(save, () => saved.set(keyOf(p, field), value));
  }

  return {
    rename(p, title) {
      const next = title.trim();
      return setField(p, "title", next, () => deps.projects.rename(p.storeId, p.id, next));
    },
    setStatus(p, status) {
      return setField(p, "status", status, () => deps.projects.setStatus(p.storeId, p.id, status));
    },
    setOwner(p, owner) {
      return setField(p, "owner", owner, () => deps.projects.setOwner(p.storeId, p.id, owner));
    },
    setDate(p, field, value) {
      return setField(p, field, value, () => deps.projects.setDate(p.storeId, p.id, field, value));
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
