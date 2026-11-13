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
  // 保存の返事と、そのあとの読み直しが終わるまで、画面のプロジェクトは保存の前の値のままになる。そのあいだは、この
  // 画面で送った値(sent)と比べる(保存の前の値と比べると、送った値をすぐ元に戻す変更が「同じ」に見えて送られない)。
  // sent は、欄ごとに、送った順の値のうち断られていない・失敗していないもの。比べるのはその最後の値で、なければ画面の
  // プロジェクトの値。送る前に足し、断られた・失敗したらその 1 つを外す(同じ欄に送った値がいくつ断られても、
  // 断られた値を比べる先にしない)。busy は、終わっていない保存と読み直しの数。すべて終わったら sent を捨てる。
  let busy = 0;
  const sent = new Map<string, { value: string }[]>();
  const keyOf = (p: Project, field: Field) => `${p.storeId}/${p.id}/${field}`;
  const same = (p: Project, field: Field, value: string) => {
    const last = sent.get(keyOf(p, field))?.at(-1);
    return value === (last ? last.value : p[field]);
  };

  // attempt は、保存を 1 回行う。断られたらその文言を返し(読み直さない)、保存できたら読み直して null を返す。
  // 断り以外のエラーはそのまま投げる。保存が断られた・失敗したら、undo を呼ぶ。
  async function attempt(save: () => Promise<unknown>, undo = () => {}): Promise<string | null> {
    busy += 1;
    try {
      try {
        await save();
      } catch (e) {
        undo();
        if (e instanceof Refused) return e.message;
        throw e;
      }
      await deps.onChanged();
      return null;
    } finally {
      busy -= 1;
      if (busy === 0) sent.clear();
    }
  }

  // attemptWithToast は、断りをエラーのトーストで出す。保存できたら true。
  async function attemptWithToast(save: () => Promise<unknown>, undo?: () => void) {
    const message = await attempt(save, undo);
    if (message !== null) deps.toasts.show(message, { error: true });
    return message === null;
  }

  // setField は、値が前と同じなら送らず、違えば送る値を sent に足して保存する。断られた・失敗したら、その値を sent から
  // 外す。
  function setField(p: Project, field: Field, value: string, save: () => Promise<unknown>) {
    if (same(p, field, value)) return Promise.resolve(true);
    const key = keyOf(p, field);
    const entry = { value };
    const list = sent.get(key) ?? [];
    list.push(entry);
    sent.set(key, list);
    const undo = () => {
      const now = sent.get(key);
      if (!now) return;
      const rest = now.filter((e) => e !== entry);
      if (rest.length > 0) sent.set(key, rest);
      else sent.delete(key);
    };
    return attemptWithToast(save, undo);
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
