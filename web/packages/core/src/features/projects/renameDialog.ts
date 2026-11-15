import type { Project } from "../../model/project";
import { type Readable, readonly, writable } from "../../reactive/store";

// ツリーのプロジェクトの行の「名前を変更…」のダイアログ(E3a-14)。検査と断りの文は projectEditor の rename と同じで、
// 断りは入力欄の下(error)に出す。

export interface RenameProjectDialogState {
  project: Project;
  error: string | null;
}

export interface RenameProjectDialogDeps {
  // rename は、保存できたら null、断られたらその文言(projectEditor.renameOrMessage)。
  rename(project: Project, title: string): Promise<string | null>;
}

export interface RenameProjectDialog {
  state: Readable<RenameProjectDialogState | null>;
  open(project: Project): void;
  // save は、保存できたら閉じ、断られたら開いたまま error に文言を入れる。
  save(title: string): Promise<void>;
  close(): void;
}

export function createRenameProjectDialog(deps: RenameProjectDialogDeps): RenameProjectDialog {
  const state = writable<RenameProjectDialogState | null>(null);
  // 開く・閉じるの世代。保存の返事を待つあいだに閉じた・開き直したときは、その返事で状態を変えない。
  let seq = 0;
  let current: Project | null = null;
  // 保存の返事を待っている世代。そのあいだの save は何もしない(2 回目の Enter が、同じ値を「送り済み」と見て先に閉じ、
  // 1 回目が断られたときに閉じたダイアログが開き直るのを防ぐ)。
  let saving = -1;
  return {
    state: readonly(state),
    open(project) {
      seq += 1;
      current = project;
      state.set({ project, error: null });
    },
    async save(title) {
      const project = current;
      if (!project || saving === seq) return;
      const mine = seq;
      saving = mine;
      let message: string | null;
      try {
        message = await deps.rename(project, title);
      } finally {
        if (saving === mine) saving = -1;
      }
      if (mine !== seq) return;
      state.set(message === null ? null : { project, error: message });
    },
    close() {
      seq += 1;
      current = null;
      state.set(null);
    },
  };
}
